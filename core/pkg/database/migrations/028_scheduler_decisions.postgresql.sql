-- v2.1: Create scheduler decisions table for decision logging

CREATE TABLE IF NOT EXISTS scheduler_decisions (
    id          SERIAL PRIMARY KEY,
    request_id  VARCHAR(255) NOT NULL,
    user_id     INTEGER DEFAULT 0,
    tenant_id   VARCHAR(255) DEFAULT '',
    model       VARCHAR(255) NOT NULL,
    backend     VARCHAR(255) NOT NULL,
    strategy    VARCHAR(255) NOT NULL,
    score       REAL DEFAULT 0,
    reason      TEXT DEFAULT '',
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sd_time ON scheduler_decisions(created_at);
CREATE INDEX IF NOT EXISTS idx_sd_tenant ON scheduler_decisions(tenant_id, created_at);
