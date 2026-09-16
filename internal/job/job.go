// Package job defines the generic analytics worker framework — see
// shop_docs/docs/adr/0008-generic-analytics-worker-framework.md. Adding a
// new analytics capability means implementing Job and registering it, not
// building a new deployable service.
package job

import (
	"context"

	"github.com/hegarty/shop_platform/money"
	"github.com/hegarty/shop_platform/period"
)

// Job is one analytics capability. Implementations must be safe to run
// concurrently for different (tenantID, period) pairs, and idempotent —
// running the same job for the same tenant and period twice must produce
// the same result and must be safe to store twice.
type Job interface {
	// Name is the job's identifier, e.g. "sales.channel.breakdown". Used as
	// the analytics_jobs.job_name / analytics_results.job_name value and to
	// route an incoming analytics.jobs message to the right implementation.
	Name() string

	// Execute runs the job for one tenant and period, returning a
	// structured Result to be stored and published.
	Execute(ctx context.Context, tenantID string, p period.Range) (*Result, error)
}

// Result is a job's structured output. Sales is specific to
// sales.channel.breakdown today; future jobs will need their own typed
// payload shape here (or a job-specific field alongside Sales, once a
// second job exists — not speculatively added before it does).
type Result struct {
	TenantID string       `json:"tenant_id"`
	Job      string       `json:"job"`
	Period   ResultPeriod `json:"period"`
	Sales    *SalesResult `json:"sales,omitempty"`
}

type ResultPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Label string `json:"label"`
}

// SalesResult is sales.channel.breakdown's output shape.
type SalesResult struct {
	Total      money.Amount        `json:"total"`
	Shop       money.Amount        `json:"shop"`
	Collective CollectiveBreakdown `json:"collective"`
	OrderCount int                 `json:"order_count"`
	AOV        money.Amount        `json:"aov"`
}

type CollectiveBreakdown struct {
	Total    money.Amount       `json:"total"`
	Partners []PartnerBreakdown `json:"partners"`
}

type PartnerBreakdown struct {
	Name  string       `json:"name"`
	Sales money.Amount `json:"sales"`
}
