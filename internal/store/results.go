package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hegarty/shop_analytics/internal/job"
)

// ResultStore persists analytics_jobs lifecycle rows and their
// analytics_results.
type ResultStore struct {
	pool *pgxpool.Pool
}

func NewResultStore(pool *pgxpool.Pool) *ResultStore {
	return &ResultStore{pool: pool}
}

// StartJob records a new analytics_jobs row in "running" state and returns
// its ID.
func (s *ResultStore) StartJob(ctx context.Context, tenantID, jobName string, periodStart, periodEnd time.Time) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO analytics_jobs (tenant_id, job_name, period_start, period_end, status, started_at)
		VALUES ($1, $2, $3, $4, 'running', now())
		RETURNING id
	`, tenantID, jobName, periodStart, periodEnd).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: start job: %w", err)
	}
	return id, nil
}

// CompleteJob marks a job completed and stores its result.
func (s *ResultStore) CompleteJob(ctx context.Context, jobID int64, tenantID, jobName string, periodStart, periodEnd time.Time, result *job.Result) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("store: marshal result: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE analytics_jobs SET status = 'completed', completed_at = now() WHERE id = $1
	`, jobID); err != nil {
		return fmt.Errorf("store: mark job completed: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO analytics_results (job_id, tenant_id, job_name, period_start, period_end, result)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, jobID, tenantID, jobName, periodStart, periodEnd, resultJSON); err != nil {
		return fmt.Errorf("store: insert result: %w", err)
	}

	return tx.Commit(ctx)
}

// FailJob marks a job failed with the given error message.
func (s *ResultStore) FailJob(ctx context.Context, jobID int64, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE analytics_jobs SET status = 'failed', completed_at = now(), error = $2 WHERE id = $1
	`, jobID, errMsg)
	if err != nil {
		return fmt.Errorf("store: mark job failed: %w", err)
	}
	return nil
}
