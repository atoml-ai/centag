-- Conversation sessions and messages (team)
CREATE TABLE IF NOT EXISTS conversation_sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL DEFAULT 0,
    tenant_id VARCHAR(255) DEFAULT '',
    title TEXT DEFAULT '',
    category VARCHAR(128) DEFAULT 'general',
    pipeline_id VARCHAR(255) DEFAULT '',
    proxy_mode VARCHAR(128) DEFAULT '',
    message_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_conv_sess_user_time ON conversation_sessions(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_conv_sess_category ON conversation_sessions(category, updated_at);
CREATE INDEX IF NOT EXISTS idx_conv_sess_tenant_time ON conversation_sessions(tenant_id, updated_at);

CREATE TABLE IF NOT EXISTS conversation_messages (
    id VARCHAR(64) PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL REFERENCES conversation_sessions(id),
    role VARCHAR(32) NOT NULL,
    content TEXT DEFAULT '',
    request_id VARCHAR(255) DEFAULT '',
    model VARCHAR(255) DEFAULT '',
    backend VARCHAR(255) DEFAULT '',
    pipeline_id VARCHAR(255) DEFAULT '',
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    latency_ms BIGINT DEFAULT 0,
    status_code INTEGER DEFAULT 200,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_conv_msg_session ON conversation_messages(session_id, created_at);
