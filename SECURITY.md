# Security Policy

This service reads a real business's sales data and computes analytics from it. Please
report suspected vulnerabilities privately: GitHub's private vulnerability reporting for
this repository (Security tab -> "Report a vulnerability"), or me@terencehegarty.com.

## Scope

Particularly interested in: anything that could let one tenant's analytics job read or
influence another tenant's data (see `internal/jobs`'s tenant-scoped queries), and any
injection via job configuration (`job_schedules.configuration` is JSON parsed with a fixed
schema — `period_kind` — not evaluated or interpolated into SQL).

## Secrets handling

No database credential should ever be committed here — read from AWS Secrets Manager at
runtime. Report any committed secret via the private channel above immediately.
