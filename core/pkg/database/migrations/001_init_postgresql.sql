-- ============================================
-- Centag PostgreSQL 数据库迁移脚本
-- Phase 0: 基础表结构
-- ============================================

-- ============================================
-- 1. 用户与认证相关
-- ============================================

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'normal' CHECK (role IN ('admin', 'normal')),
    display_name VARCHAR(100),
    email VARCHAR(255),
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- API Key 表
CREATE TABLE IF NOT EXISTS api_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100),
    key_hash VARCHAR(64) UNIQUE NOT NULL,
    key_prefix VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 刷新令牌表
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    revoked BOOLEAN DEFAULT false
);

-- ============================================
-- 2. 配置相关
-- ============================================

-- 系统配置表（键值对）
CREATE TABLE IF NOT EXISTS system_config (
    id BIGSERIAL PRIMARY KEY,
    config_key VARCHAR(100) UNIQUE NOT NULL,
    config_value JSONB NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_by BIGINT REFERENCES users(id)
);

-- 后端配置表
CREATE TABLE IF NOT EXISTS backends (
    id BIGSERIAL PRIMARY KEY,
    backend_id VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    backend_type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN DEFAULT true,
    priority INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 预设模式表
CREATE TABLE IF NOT EXISTS preset_modes (
    id BIGSERIAL PRIMARY KEY,
    mode_id VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- 3. Token 计量相关（新增）
-- ============================================

-- Token 使用记录表（明细）
CREATE TABLE IF NOT EXISTS token_usage (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    api_key_id BIGINT REFERENCES api_keys(id),
    backend_id VARCHAR(100) NOT NULL,
    model VARCHAR(100) NOT NULL,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    request_id VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    source VARCHAR(20) NOT NULL DEFAULT 'real'  -- 038: 'real' 或 'estimated'
);

-- Token 使用统计表（按天聚合，加速查询）
CREATE TABLE IF NOT EXISTS token_usage_daily (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    backend_id VARCHAR(100) NOT NULL,
    model VARCHAR(100) NOT NULL,
    date DATE NOT NULL,
    total_prompt_tokens INTEGER DEFAULT 0,
    total_completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, backend_id, model, date)
);

-- Token 配额表
CREATE TABLE IF NOT EXISTS token_quotas (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id),
    daily_limit INTEGER DEFAULT 0,  -- 0 = 无限制
    monthly_limit INTEGER DEFAULT 0, -- 0 = 无限制
    used_today INTEGER DEFAULT 0,
    used_this_month INTEGER DEFAULT 0,
    reset_date DATE DEFAULT CURRENT_DATE,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- 4. Clash 订阅相关
-- ============================================

CREATE TABLE IF NOT EXISTS clash_rules (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    rule_content TEXT,
    subscribe_token VARCHAR(64) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- 5. 索引优化
-- ============================================

-- 用户相关索引
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON api_keys(key_prefix);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- Token 计量索引
CREATE INDEX IF NOT EXISTS idx_token_usage_user_id ON token_usage(user_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_created_at ON token_usage(created_at);
CREATE INDEX IF NOT EXISTS idx_token_usage_user_date ON token_usage(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_token_usage_daily_user_date ON token_usage_daily(user_id, date);

-- 后端配置索引
CREATE INDEX IF NOT EXISTS idx_backends_enabled ON backends(enabled);
CREATE INDEX IF NOT EXISTS idx_backends_type ON backends(backend_type);

-- ============================================
-- 6. 触发器：自动更新 updated_at
-- ============================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_backends_updated_at BEFORE UPDATE ON backends
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_clash_rules_updated_at BEFORE UPDATE ON clash_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- 7. 迁移记录表（用于跟踪迁移历史）
-- ============================================

CREATE TABLE IF NOT EXISTS schema_migrations (
    id SERIAL PRIMARY KEY,
    version VARCHAR(100) UNIQUE NOT NULL,
    applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- 8. 初始化默认数据
-- ============================================
-- 管理员由应用首次启动时 bootstrap.Seed 根据 LLM_PROXY_ADMIN_* 创建（勿在此插入占位密码）

-- 插入默认后端配置（Ollama 本地）
INSERT INTO backends (backend_id, name, backend_type, config, enabled, priority)
VALUES ('ollama-local', 'Ollama 本地', 'ollama', '{"base_url": "http://localhost:21434"}'::jsonb, true, 100)
ON CONFLICT (backend_id) DO NOTHING;
