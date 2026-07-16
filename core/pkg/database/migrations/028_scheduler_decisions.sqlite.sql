-- v2.1: Create scheduler decisions table for decision logging

CREATE TABLE IF NOT EXISTS scheduler_decisions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id  TEXT NOT NULL,
    user_id     INTEGER DEFAULT 0,
    tenant_id   TEXT DEFAULT '',               -- 租户 ID，单用户模式为空
    model       TEXT NOT NULL,
    backend     TEXT NOT NULL,
    strategy    TEXT NOT NULL,
    score       REAL DEFAULT 0,
    reason      TEXT DEFAULT '',
    created_at  TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_sd_time ON scheduler_decisions(created_at);
CREATE INDEX IF NOT EXISTS idx_sd_tenant ON scheduler_decisions(tenant_id, created_at);
