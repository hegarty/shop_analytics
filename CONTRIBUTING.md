# Contributing

See [shop_platform/CONTRIBUTING.md](https://github.com/hegarty/shop_platform/blob/main/CONTRIBUTING.md)
for the general workflow (PRs required, signed commits, required checks).

## Adding a new analytics job

Implement `job.Job` (see `internal/job/job.go` and `internal/jobs/sales_channel_breakdown.go`
for the pattern) and register it in `cmd/worker/main.go`'s `job.NewRegistry(...)` call. New
analytics capability is a new `Job` implementation, not a new deployable service — see
[ADR-0008](https://github.com/hegarty/shop_docs/blob/main/docs/adr/0008-generic-analytics-worker-framework.md).

## Local development

```bash
make test
make lint
make migrate-up   # against a local/test database only
```
