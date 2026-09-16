// Package worker executes analytics jobs consumed from analytics.jobs and
// publishes their results — see ADR-0008.
package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/hegarty/shop_platform/period"

	"github.com/hegarty/shop_analytics/internal/job"
)

// AnalyticsResultsTopic is where completed job.Result values are
// published.
const AnalyticsResultsTopic = "analytics.results"

// ResultStore records job lifecycle + persists results.
type ResultStore interface {
	StartJob(ctx context.Context, tenantID, jobName string, periodStart, periodEnd time.Time) (int64, error)
	CompleteJob(ctx context.Context, jobID int64, tenantID, jobName string, periodStart, periodEnd time.Time, result *job.Result) error
	FailJob(ctx context.Context, jobID int64, errMsg string) error
}

// Publisher publishes a value to a topic, keyed by tenant.
type Publisher interface {
	Publish(ctx context.Context, topic, tenantID, eventType string, value any) error
}

type Worker struct {
	Registry *job.Registry
	Store    ResultStore
	Notifier Publisher
}

// HandleRequest executes one job.Request: looks up the job by name,
// records start/completion in Postgres, and publishes the result.
func (w *Worker) HandleRequest(ctx context.Context, req job.Request) error {
	impl, ok := w.Registry.Lookup(req.Job)
	if !ok {
		return fmt.Errorf("worker: no job registered for %q", req.Job)
	}

	periodStart, err := time.Parse(time.RFC3339, req.PeriodStart)
	if err != nil {
		return fmt.Errorf("worker: parse period_start: %w", err)
	}
	periodEnd, err := time.Parse(time.RFC3339, req.PeriodEnd)
	if err != nil {
		return fmt.Errorf("worker: parse period_end: %w", err)
	}

	jobID, err := w.Store.StartJob(ctx, req.TenantID, req.Job, periodStart, periodEnd)
	if err != nil {
		return fmt.Errorf("worker: start job: %w", err)
	}

	p := period.Range{Start: periodStart, End: periodEnd, Label: req.PeriodLabel}
	result, err := impl.Execute(ctx, req.TenantID, p)
	if err != nil {
		if failErr := w.Store.FailJob(ctx, jobID, err.Error()); failErr != nil {
			return fmt.Errorf("worker: execute failed (%v) and failed to record failure: %w", err, failErr)
		}
		return fmt.Errorf("worker: execute %q: %w", req.Job, err)
	}

	if err := w.Store.CompleteJob(ctx, jobID, req.TenantID, req.Job, periodStart, periodEnd, result); err != nil {
		return fmt.Errorf("worker: complete job: %w", err)
	}

	if err := w.Notifier.Publish(ctx, AnalyticsResultsTopic, req.TenantID, req.Job, result); err != nil {
		return fmt.Errorf("worker: publish result: %w", err)
	}
	return nil
}
