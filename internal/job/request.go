package job

// Request is what the scheduler publishes to analytics.jobs and the worker
// consumes. The scheduler only ever emits these — it never executes
// analytics itself. See ADR-0008.
type Request struct {
	TenantID    string `json:"tenant_id"`
	Job         string `json:"job"`
	PeriodStart string `json:"period_start"` // RFC3339, UTC
	PeriodEnd   string `json:"period_end"`   // RFC3339, UTC
	PeriodLabel string `json:"period_label"`
}

// Registry maps job names to implementations, so the worker can route an
// incoming Request to the right Job without a switch statement that grows
// with every new job.
type Registry struct {
	jobs map[string]Job
}

func NewRegistry(jobs ...Job) *Registry {
	r := &Registry{jobs: make(map[string]Job, len(jobs))}
	for _, j := range jobs {
		r.jobs[j.Name()] = j
	}
	return r
}

// Lookup returns the Job registered under name, or (nil, false).
func (r *Registry) Lookup(name string) (Job, bool) {
	j, ok := r.jobs[name]
	return j, ok
}
