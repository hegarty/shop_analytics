// Package scheduler emits work onto analytics.jobs on a cron schedule. It
// never computes analytics itself — see ADR-0008.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/hegarty/shop_platform/period"

	"github.com/hegarty/shop_analytics/internal/job"
)

// AnalyticsJobsTopic is where job.Request messages are published.
const AnalyticsJobsTopic = "analytics.jobs"

// Schedule is one tenant's recurring analytics job configuration.
// PeriodKind is decoded from job_schedules.configuration (JSON:
// {"period_kind": "daily"}) by the Store implementation, not by this
// package — see store.ScheduleStore.
type Schedule struct {
	ID         int64
	TenantID   string
	JobName    string
	Frequency  string // standard 5-field cron expression
	Timezone   string // IANA name
	PeriodKind period.Kind
	NextRunAt  *time.Time
}

// Store persists and queries schedules.
type Store interface {
	DueSchedules(ctx context.Context, now time.Time) ([]Schedule, error)
	UpdateNextRun(ctx context.Context, id int64, next time.Time) error
}

// Publisher publishes a value to a topic, keyed by tenant.
type Publisher interface {
	Publish(ctx context.Context, topic, tenantID, eventType string, value any) error
}

type Scheduler struct {
	Store     Store
	Publisher Publisher
}

// Tick publishes a job.Request for every schedule due at or before now, and
// advances each one's next_run_at. Intended to be called periodically
// (e.g. every minute) by cmd/scheduler.
func (s *Scheduler) Tick(ctx context.Context, now time.Time) error {
	due, err := s.Store.DueSchedules(ctx, now)
	if err != nil {
		return fmt.Errorf("scheduler: fetch due schedules: %w", err)
	}

	for _, sched := range due {
		if err := s.runOne(ctx, sched, now); err != nil {
			// One bad schedule (e.g. an invalid cron expression) shouldn't
			// block the rest from running.
			continue
		}
	}
	return nil
}

func (s *Scheduler) runOne(ctx context.Context, sched Schedule, now time.Time) error {
	loc, err := time.LoadLocation(sched.Timezone)
	if err != nil {
		return fmt.Errorf("scheduler: schedule %d: invalid timezone %q: %w", sched.ID, sched.Timezone, err)
	}

	p, err := period.Resolve(sched.PeriodKind, now, loc)
	if err != nil {
		return fmt.Errorf("scheduler: schedule %d: resolve period: %w", sched.ID, err)
	}

	req := job.Request{
		TenantID:    sched.TenantID,
		Job:         sched.JobName,
		PeriodStart: p.Start.Format(time.RFC3339),
		PeriodEnd:   p.End.Format(time.RFC3339),
		PeriodLabel: p.Label,
	}
	if err := s.Publisher.Publish(ctx, AnalyticsJobsTopic, sched.TenantID, sched.JobName, req); err != nil {
		return fmt.Errorf("scheduler: schedule %d: publish: %w", sched.ID, err)
	}

	schedule, err := cron.ParseStandard(sched.Frequency)
	if err != nil {
		return fmt.Errorf("scheduler: schedule %d: invalid cron expression %q: %w", sched.ID, sched.Frequency, err)
	}
	next := schedule.Next(now.In(loc))
	if err := s.Store.UpdateNextRun(ctx, sched.ID, next.UTC()); err != nil {
		return fmt.Errorf("scheduler: schedule %d: update next_run_at: %w", sched.ID, err)
	}
	return nil
}
