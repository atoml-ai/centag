-- ============================================
-- Proxy Claw SQLite 数据库迁移脚本
-- Phase 6: 用户配置表
-- ============================================

-- 用户配置表（存储用户级配置，如后端、代理、缓存等设置）
CREATE TABLE IF NOT EXISTS user_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    backends TEXT DEFAULT '[]',
    proxy_settings TEXT DEFAULT '{}',
    cache_settings TEXT DEFAULT '{}',
    embedding TEXT DEFAULT '{}',
    qa_split TEXT DEFAULT '{}',
    preset_modes TEXT DEFAULT '[]',
    scheduling TEXT DEFAULT '{}',
    cache_control TEXT DEFAULT '{}',
    auth_settings TEXT DEFAULT '{"require_api_key":false,"allow_no_auth":true}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_user_config_user_id ON user_config(user_id);
