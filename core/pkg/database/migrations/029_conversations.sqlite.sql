-- Conversation sessions and messages (personal)
CREATE TABLE IF NOT EXISTS conversation_sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL DEFAULT 0,
    tenant_id TEXT DEFAULT '',
    title TEXT DEFAULT '',
    category TEXT DEFAULT 'general',
    pipeline_id TEXT DEFAULT '',
    proxy_mode TEXT DEFAULT '',
    message_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_conv_sess_user_time ON conversation_sessions(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_conv_sess_category ON conversation_sessions(category, updated_at);

CREATE TABLE IF NOT EXISTS conversation_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT DEFAULT '',
    request_id TEXT DEFAULT '',
    model TEXT DEFAULT '',
    backend TEXT DEFAULT '',
    pipeline_id TEXT DEFAULT '',
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    status_code INTEGER DEFAULT 200,
    created_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY(session_id) REFERENCES conversation_sessions(id)
);
CREATE INDEX IF NOT EXISTS idx_conv_msg_session ON conversation_messages(session_id, created_at);
