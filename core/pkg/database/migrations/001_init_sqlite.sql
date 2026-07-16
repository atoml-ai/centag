-- ============================================
-- Centag SQLite 数据库迁移脚本
-- Phase 0: 基础表结构
-- ============================================

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT DEFAULT 'normal' CHECK (role IN ('admin', 'normal')),
    display_name TEXT,
    email TEXT,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- API Key 表
CREATE TABLE IF NOT EXISTS api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT,
    key_hash TEXT UNIQUE NOT NULL,
    key_prefix TEXT NOT NULL,
    expires_at DATETIME,
    last_used_at DATETIME,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 刷新令牌表
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    revoked INTEGER DEFAULT 0
);

-- 系统配置表
CREATE TABLE IF NOT EXISTS system_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_key TEXT UNIQUE NOT NULL,
    config_value TEXT NOT NULL,
    description TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by INTEGER REFERENCES users(id)
);

-- 后端配置表
CREATE TABLE IF NOT EXISTS backends (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    backend_id TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    backend_type TEXT NOT NULL,
    config TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    priority INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 预设模式表
CREATE TABLE IF NOT EXISTS preset_modes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mode_id TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    config TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Token 使用记录表
CREATE TABLE IF NOT EXISTS token_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    api_key_id INTEGER REFERENCES api_keys(id),
    backend_id TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    request_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Token 使用统计表
CREATE TABLE IF NOT EXISTS token_usage_daily (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    backend_id TEXT NOT NULL,
    model TEXT NOT NULL,
    date DATE NOT NULL,
    total_prompt_tokens INTEGER DEFAULT 0,
    total_completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, backend_id, model, date)
);

-- Token 配额表
CREATE TABLE IF NOT EXISTS token_quotas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER UNIQUE NOT NULL REFERENCES users(id),
    daily_limit INTEGER DEFAULT 0,
    monthly_limit INTEGER DEFAULT 0,
    used_today INTEGER DEFAULT 0,
    used_this_month INTEGER DEFAULT 0,
    reset_date DATE DEFAULT CURRENT_DATE,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Clash 规则表
CREATE TABLE IF NOT EXISTS clash_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    rule_content TEXT,
    subscribe_token TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON api_keys(key_prefix);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_user_id ON token_usage(user_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_created_at ON token_usage(created_at);
CREATE INDEX IF NOT EXISTS idx_token_usage_user_date ON token_usage(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_token_usage_daily_user_date ON token_usage_daily(user_id, date);
CREATE INDEX IF NOT EXISTS idx_backends_enabled ON backends(enabled);
CREATE INDEX IF NOT EXISTS idx_backends_type ON backends(backend_type);
CREATE INDEX IF NOT EXISTS idx_clash_rules_user_id ON clash_rules(user_id);

-- 默认数据
INSERT OR IGNORE INTO backends (backend_id, name, backend_type, config, enabled, priority)
VALUES ('ollama-local', 'Ollama 本地', 'ollama', '{"base_url": "http://localhost:21434"}', 1, 100);
