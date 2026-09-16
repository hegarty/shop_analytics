CREATE TABLE analytics_results (
    id            BIGSERIAL PRIMARY KEY,
    job_id        BIGINT NOT NULL REFERENCES analytics_jobs(id),
    tenant_id     TEXT NOT NULL,
    job_name      TEXT NOT NULL,
    period_start  TIMESTAMPTZ NOT NULL,
    period_end    TIMESTAMPTZ NOT NULL,
    result        JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_analytics_results_tenant_job ON analytics_results (tenant_id, job_name, period_start);
