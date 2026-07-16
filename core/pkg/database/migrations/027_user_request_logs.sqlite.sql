-- v2.1: Create user request logs table for request auditing

CREATE TABLE IF NOT EXISTS user_request_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL,
    tenant_id   TEXT DEFAULT '',               -- 租户 ID，单用户模式为空
    request_id  TEXT NOT NULL,
    model       TEXT NOT NULL,
    backend     TEXT NOT NULL,
    pipeline    TEXT DEFAULT '',
    input_tokens  INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    latency_ms    INTEGER DEFAULT 0,
    status_code   INTEGER DEFAULT 200,
    request_body  TEXT DEFAULT '',             -- 请求摘要（截断存储）
    response_body TEXT DEFAULT '',             -- 响应摘要（截断存储）
    created_at  TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_url_user_time ON user_request_logs(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_url_tenant_time ON user_request_logs(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_url_model ON user_request_logs(model, created_at);
