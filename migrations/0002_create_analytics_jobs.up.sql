CREATE TABLE analytics_jobs (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     TEXT NOT NULL,
    job_name      TEXT NOT NULL,
    period_start  TIMESTAMPTZ NOT NULL,
    period_end    TIMESTAMPTZ NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','completed','failed')),
    requested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    error         TEXT
);

CREATE INDEX idx_analytics_jobs_status ON analytics_jobs (status, requested_at);
