-- v2.1: Create team quota table for team-level token limits

CREATE TABLE IF NOT EXISTS team_quota (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id       TEXT DEFAULT '',           -- 租户 ID，单用户模式为空
    daily_token_limit   INTEGER DEFAULT 0,    -- 团队每日 Token 总限额，0=不限
    monthly_token_limit INTEGER DEFAULT 0,    -- 团队每月 Token 总限额，0=不限
    daily_token_used    INTEGER DEFAULT 0,    -- 团队当日已用
    monthly_token_used  INTEGER DEFAULT 0,    -- 团队当月已用
    created_at      TEXT DEFAULT (datetime('now')),
    updated_at      TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_team_quota_tenant ON team_quota(tenant_id);
