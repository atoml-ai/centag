-- v2.1: Create team quota table for team-level token limits

CREATE TABLE IF NOT EXISTS team_quota (
    id              SERIAL PRIMARY KEY,
    tenant_id       VARCHAR(255) DEFAULT '',
    daily_token_limit   BIGINT DEFAULT 0,
    monthly_token_limit BIGINT DEFAULT 0,
    daily_token_used    BIGINT DEFAULT 0,
    monthly_token_used  BIGINT DEFAULT 0,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_team_quota_tenant ON team_quota(tenant_id);
