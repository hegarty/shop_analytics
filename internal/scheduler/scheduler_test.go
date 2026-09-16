package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/hegarty/shop_platform/period"

	"github.com/hegarty/shop_analytics/internal/job"
)

type fakeStore struct {
	due        []Schedule
	nextRunSet map[int64]time.Time
}

func (f *fakeStore) DueSchedules(_ context.Context, _ time.Time) ([]Schedule, error) {
	return f.due, nil
}

func (f *fakeStore) UpdateNextRun(_ context.Context, id int64, next time.Time) error {
	if f.nextRunSet == nil {
		f.nextRunSet = map[int64]time.Time{}
	}
	f.nextRunSet[id] = next
	return nil
}

type fakePublisher struct {
	published []job.Request
}

func (f *fakePublisher) Publish(_ context.Context, _, _, _ string, value any) error {
	f.published = append(f.published, value.(job.Request))
	return nil
}

func TestTick_PublishesAndAdvancesNextRun(t *testing.T) {
	store := &fakeStore{due: []Schedule{
		{ID: 1, TenantID: "devmoto", JobName: "sales.channel.breakdown", Frequency: "0 8 * * *", Timezone: "America/New_York", PeriodKind: period.Yesterday},
	}}
	pub := &fakePublisher{}
	s := &Scheduler{Store: store, Publisher: pub}

	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	if err := s.Tick(t.Context(), now); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(pub.published) != 1 {
		t.Fatalf("expected 1 published request, got %d", len(pub.published))
	}
	req := pub.published[0]
	if req.TenantID != "devmoto" || req.Job != "sales.channel.breakdown" {
		t.Errorf("unexpected request: %+v", req)
	}

	if _, ok := store.nextRunSet[1]; !ok {
		t.Error("expected next_run_at to be updated for schedule 1")
	}
}

func TestTick_SkipsInvalidScheduleWithoutBlockingOthers(t *testing.T) {
	store := &fakeStore{due: []Schedule{
		{ID: 1, TenantID: "a", JobName: "job-a", Frequency: "not a cron expr", Timezone: "America/New_York", PeriodKind: period.Daily},
		{ID: 2, TenantID: "b", JobName: "job-b", Frequency: "0 8 * * *", Timezone: "America/New_York", PeriodKind: period.Daily},
	}}
	pub := &fakePublisher{}
	s := &Scheduler{Store: store, Publisher: pub}

	if err := s.Tick(t.Context(), time.Now()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	// Schedule 1 still publishes (the publish happens before the cron parse
	// that fails), but only schedule 2 successfully advances next_run_at.
	if _, ok := store.nextRunSet[2]; !ok {
		t.Error("expected schedule 2 to advance despite schedule 1's bad cron expression")
	}
}
