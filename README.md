# shop_analytics

Scheduler + generic analytics worker framework for the commerce-intel platform. See
[shop_docs](https://github.com/hegarty/shop_docs) for the architecture and
[ADR-0008](https://github.com/hegarty/shop_docs/blob/main/docs/adr/0008-generic-analytics-worker-framework.md)
for why this is one service with a plugin-style job registry, not one deployable service
per analytics capability.

## What's here

- **`cmd/scheduler`** — polls `job_schedules` for due work (cron expressions, tenant
  timezone-aware) and publishes `analytics.jobs` messages. Never computes analytics itself.
- **`cmd/worker`** — consumes `analytics.jobs`, executes the matching `job.Job`
  implementation, persists the result, publishes `analytics.results`.
- **`cmd/migrate`** — applies/rolls back this repo's schema migrations
  (`job_schedules`, `analytics_jobs`, `analytics_results`).
- **`internal/job`** — the `Job` interface, `Registry`, and result types.
- **`internal/jobs`** — concrete implementations. Currently just
  `sales.channel.breakdown` — the platform's first, launch-blocking analytics job. Its
  aggregation logic is tested directly against the worked example from the original
  product brief (see `internal/jobs/sales_channel_breakdown_test.go`).
- **`internal/scheduler`**, **`internal/worker`**, **`internal/store`** — the plumbing
  connecting the two `cmd/` binaries to Postgres and Redpanda.

## Sales metric definition

`sales.channel.breakdown` reports **net_sales** (gross - discounts - returns, excluding
tax and shipping) as "sales" throughout — not total_sales. See the doc comment on
`SalesChannelBreakdown` and [shop_docs/docs/database-model.md](https://github.com/hegarty/shop_docs/blob/main/docs/database-model.md)
for why.

## Adding a schedule

`job_schedules` rows aren't managed by this codebase yet (no admin API/UI) — insert
directly for now:

```sql
INSERT INTO job_schedules (tenant_id, job_name, frequency, timezone, configuration)
VALUES ('devmoto', 'sales.channel.breakdown', '0 8 * * *', 'America/New_York', '{"period_kind": "yesterday"}');
```

`configuration.period_kind` must be one of `shop_platform/period`'s `Kind` values (`today`,
`yesterday`, `week_to_date`, `previous_week`, `month_to_date`, `previous_month`,
`quarter_to_date`, `previous_quarter`, `year_to_date`, `daily`, `weekly`, `monthly`,
`quarterly`).

## Development

```bash
make test
make lint
make migrate-up   # against a local/test database only
```
