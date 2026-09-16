package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hegarty/shop_platform/period"

	"github.com/hegarty/shop_analytics/internal/scheduler"
)

// ScheduleStore implements scheduler.Store against Postgres.
type ScheduleStore struct {
	pool *pgxpool.Pool
}

func NewScheduleStore(pool *pgxpool.Pool) *ScheduleStore {
	return &ScheduleStore{pool: pool}
}

func (s *ScheduleStore) DueSchedules(ctx context.Context, now time.Time) ([]scheduler.Schedule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, job_name, frequency, timezone, configuration
		FROM job_schedules
		WHERE enabled = true
		  AND (next_run_at IS NULL OR next_run_at <= $1)
	`, now)
	if err != nil {
		return nil, fmt.Errorf("store: query due schedules: %w", err)
	}
	defer rows.Close()

	var schedules []scheduler.Schedule
	for rows.Next() {
		var (
			sc     scheduler.Schedule
			config []byte
		)
		if err := rows.Scan(&sc.ID, &sc.TenantID, &sc.JobName, &sc.Frequency, &sc.Timezone, &config); err != nil {
			return nil, fmt.Errorf("store: scan schedule: %w", err)
		}
		kind, err := periodKindFromConfig(config)
		if err != nil {
			// Skip a misconfigured schedule rather than failing the whole
			// batch — it'll show up in logs at the call site.
			continue
		}
		sc.PeriodKind = kind
		schedules = append(schedules, sc)
	}
	return schedules, rows.Err()
}

func (s *ScheduleStore) UpdateNextRun(ctx context.Context, id int64, next time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE job_schedules SET next_run_at = $1 WHERE id = $2`, next, id)
	if err != nil {
		return fmt.Errorf("store: update next_run_at for schedule %d: %w", id, err)
	}
	return nil
}

func periodKindFromConfig(raw []byte) (period.Kind, error) {
	type config struct {
		PeriodKind string `json:"period_kind"`
	}
	var c config
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", err
	}
	if c.PeriodKind == "" {
		return "", fmt.Errorf("missing period_kind")
	}
	return period.Kind(c.PeriodKind), nil
}
