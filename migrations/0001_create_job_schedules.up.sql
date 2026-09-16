CREATE TABLE job_schedules (
    id             BIGSERIAL PRIMARY KEY,
    tenant_id      TEXT NOT NULL,
    job_name       TEXT NOT NULL,
    frequency      TEXT NOT NULL,       -- cron expression
    timezone       TEXT NOT NULL,       -- IANA name
    enabled        BOOLEAN NOT NULL DEFAULT true,
    configuration  JSONB NOT NULL DEFAULT '{}',
    next_run_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_job_schedules_enabled_next_run ON job_schedules (enabled, next_run_at);
