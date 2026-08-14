-- Agent sessions and messages
CREATE TABLE IF NOT EXISTS agent_sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL DEFAULT 0,
    tenant_id VARCHAR(255) DEFAULT '',
    title TEXT DEFAULT '',
    skill VARCHAR(128) DEFAULT '',
    backend_id VARCHAR(255) DEFAULT '',
    model VARCHAR(255) DEFAULT '',
    status VARCHAR(32) DEFAULT 'active',
    message_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_agent_sess_user_time ON agent_sessions(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_agent_sess_tenant_time ON agent_sessions(tenant_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_agent_sess_status ON agent_sessions(status, updated_at);

CREATE TABLE IF NOT EXISTS agent_messages (
    id VARCHAR(64) PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL REFERENCES agent_sessions(id),
    role VARCHAR(32) NOT NULL,
    content TEXT DEFAULT '',
    skill VARCHAR(128) DEFAULT '',
    tool_name VARCHAR(255) DEFAULT '',
    tool_params TEXT,
    tool_result TEXT,
    is_error BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_agent_msg_session ON agent_messages(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_msg_role ON agent_messages(role, created_at);