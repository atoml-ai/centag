-- +migrate Up
-- Migration: 017_add_tenant_id
-- Description: 为现有表添加 tenant_id 列，建立"一用户一租户"模型
-- Date: 2026-06-02

-- 1. users 表添加 tenant_id 列（唯一，一用户一租户）
ALTER TABLE users ADD COLUMN tenant_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);

-- 2. api_keys 表添加 tenant_id 列
ALTER TABLE api_keys ADD COLUMN tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys(tenant_id);

-- 3. backends 表添加 tenant_id 列（NULL = 系统共享）
ALTER TABLE backends ADD COLUMN tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_backends_tenant_id ON backends(tenant_id);

-- 4. preset_modes 表添加 tenant_id 列（NULL = 系统共享）
ALTER TABLE preset_modes ADD COLUMN tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_preset_modes_tenant_id ON preset_modes(tenant_id);

-- 5. token_usage 表添加 tenant_id 列
ALTER TABLE token_usage ADD COLUMN tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_token_usage_tenant_id ON token_usage(tenant_id);

-- 6. token_usage_daily 表添加 tenant_id 列
ALTER TABLE token_usage_daily ADD COLUMN tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_token_usage_daily_tenant_id ON token_usage_daily(tenant_id);

-- 7. token_quotas 表添加 tenant_id 列（替代 user_id 成为租户级配额）
-- 保留 user_id 用于关联，新增 tenant_id 用于配额隔离
ALTER TABLE token_quotas ADD COLUMN tenant_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_token_quotas_tenant_id ON token_quotas(tenant_id);

-- 8. clash_rules 表添加 tenant_id 列
ALTER TABLE clash_rules ADD COLUMN tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_clash_rules_tenant_id ON clash_rules(tenant_id);

-- 9. tenant_quotas 表（010 已建 tenants，此处建配额表）
CREATE TABLE IF NOT EXISTS tenant_quotas (
    tenant_id TEXT PRIMARY KEY,
    daily_limit INTEGER DEFAULT 0,
    monthly_limit INTEGER DEFAULT 0,
    used_today INTEGER DEFAULT 0,
    used_this_month INTEGER DEFAULT 0,
    daily_request_limit INTEGER DEFAULT 0,
    monthly_request_limit INTEGER DEFAULT 0,
    used_today_requests INTEGER DEFAULT 0,
    used_this_month_requests INTEGER DEFAULT 0,
    max_backends INTEGER DEFAULT 0,
    max_api_keys INTEGER DEFAULT 0,
    reset_date TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 10. 系统配置：标记多租户模式状态
INSERT OR IGNORE INTO system_config (config_key, config_value, description)
VALUES ('multi_tenant_enabled', 'false', '是否启用多租户模式');

-- +migrate Down
-- 回滚：SQLite 不支持 DROP COLUMN，需重建表（此处简化，生产环境慎用）
-- 如需回滚，建议手动导出数据后重建表结构
