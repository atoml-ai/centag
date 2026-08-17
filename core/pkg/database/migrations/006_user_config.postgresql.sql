-- ============================================
-- Centag PostgreSQL 数据库迁移脚本
-- Phase 6: 用户配置表（PostgreSQL 版）
-- 006 迁移此前仅有 sqlite 版本，导致 PostgreSQL 部署缺失 user_config 表，
-- 后续迁移（如 041 ALTER TABLE user_config）在 PostgreSQL 上失败。
-- ============================================

-- 用户配置表（存储用户级配置，如后端、代理、缓存等设置）
CREATE TABLE IF NOT EXISTS user_config (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    backends TEXT DEFAULT '[]',
    proxy_settings TEXT DEFAULT '{}',
    cache_settings TEXT DEFAULT '{}',
    embedding TEXT DEFAULT '{}',
    qa_split TEXT DEFAULT '{}',
    preset_modes TEXT DEFAULT '[]',
    scheduling TEXT DEFAULT '{}',
    cache_control TEXT DEFAULT '{}',
    auth_settings TEXT DEFAULT '{"require_api_key":false,"allow_no_auth":true}',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_user_config_user_id ON user_config(user_id);
