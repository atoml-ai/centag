-- v2.1: Create user request logs table for request auditing

CREATE TABLE IF NOT EXISTS user_request_logs (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL,
    tenant_id   VARCHAR(255) DEFAULT '',
    request_id  VARCHAR(255) NOT NULL,
    model       VARCHAR(255) NOT NULL,
    backend     VARCHAR(255) NOT NULL,
    pipeline    VARCHAR(255) DEFAULT '',
    input_tokens  BIGINT DEFAULT 0,
    output_tokens BIGINT DEFAULT 0,
    latency_ms    BIGINT DEFAULT 0,
    status_code   INTEGER DEFAULT 200,
    request_body  TEXT DEFAULT '',
    response_body TEXT DEFAULT '',
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_url_user_time ON user_request_logs(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_url_tenant_time ON user_request_logs(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_url_model ON user_request_logs(model, created_at);
