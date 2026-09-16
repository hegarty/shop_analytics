package worker

import (
	"context"
	"testing"
	"time"

	"github.com/hegarty/shop_platform/period"

	"github.com/hegarty/shop_analytics/internal/job"
)

type fakeJob struct {
	name   string
	result *job.Result
	err    error
}

func (f *fakeJob) Name() string { return f.name }
func (f *fakeJob) Execute(_ context.Context, _ string, _ period.Range) (*job.Result, error) {
	return f.result, f.err
}

type fakeStore struct {
	started   bool
	completed bool
	failed    bool
	failMsg   string
}

func (f *fakeStore) StartJob(_ context.Context, _, _ string, _, _ time.Time) (int64, error) {
	f.started = true
	return 1, nil
}
func (f *fakeStore) CompleteJob(_ context.Context, _ int64, _, _ string, _, _ time.Time, _ *job.Result) error {
	f.completed = true
	return nil
}
func (f *fakeStore) FailJob(_ context.Context, _ int64, errMsg string) error {
	f.failed = true
	f.failMsg = errMsg
	return nil
}

type fakePublisher struct{ published int }

func (f *fakePublisher) Publish(_ context.Context, _, _, _ string, _ any) error {
	f.published++
	return nil
}

func req() job.Request {
	return job.Request{
		TenantID:    "devmoto",
		Job:         "sales.channel.breakdown",
		PeriodStart: "2026-09-11T00:00:00Z",
		PeriodEnd:   "2026-09-12T00:00:00Z",
		PeriodLabel: "Sep 11",
	}
}

func TestHandleRequest_Success(t *testing.T) {
	registry := job.NewRegistry(&fakeJob{name: "sales.channel.breakdown", result: &job.Result{}})
	store := &fakeStore{}
	pub := &fakePublisher{}
	w := &Worker{Registry: registry, Store: store, Notifier: pub}

	if err := w.HandleRequest(t.Context(), req()); err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if !store.started || !store.completed {
		t.Errorf("expected StartJob and CompleteJob to be called, got started=%v completed=%v", store.started, store.completed)
	}
	if pub.published != 1 {
		t.Errorf("expected 1 publish, got %d", pub.published)
	}
}

func TestHandleRequest_JobExecutionFails_RecordsFailure(t *testing.T) {
	registry := job.NewRegistry(&fakeJob{name: "sales.channel.breakdown", err: context.DeadlineExceeded})
	store := &fakeStore{}
	pub := &fakePublisher{}
	w := &Worker{Registry: registry, Store: store, Notifier: pub}

	if err := w.HandleRequest(t.Context(), req()); err == nil {
		t.Fatal("expected error to propagate")
	}
	if !store.failed {
		t.Error("expected FailJob to be called")
	}
	if pub.published != 0 {
		t.Error("expected no publish on job failure")
	}
}

func TestHandleRequest_UnknownJob(t *testing.T) {
	w := &Worker{Registry: job.NewRegistry(), Store: &fakeStore{}, Notifier: &fakePublisher{}}
	if err := w.HandleRequest(t.Context(), req()); err == nil {
		t.Fatal("expected error for unregistered job")
	}
}
