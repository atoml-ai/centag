-- ============================================
-- Migration: 002_add_token_usage_indexes
-- Description: 添加 Token 计量相关索引
-- Created: 2026-03-22
-- Author: centag team
-- ============================================

-- +migrate Up
-- 添加 Token 使用记录的复合索引
CREATE INDEX IF NOT EXISTS idx_token_usage_user_backend 
ON token_usage(user_id, backend_id, created_at);

-- 勿用 DATE(timestamptz)：依赖会话时区，非 IMMUTABLE，无法建表达式索引。
-- 固定按 UTC 取日历日，表达式对索引合法。
CREATE INDEX IF NOT EXISTS idx_token_usage_model_date
ON token_usage (model, ((created_at AT TIME ZONE 'UTC')::date));

-- 添加配额表的日期索引
CREATE INDEX IF NOT EXISTS idx_token_quotas_reset_date 
ON token_quotas(reset_date);

-- 添加 Clash 规则的用户索引
CREATE INDEX IF NOT EXISTS idx_clash_rules_user_id 
ON clash_rules(user_id);

-- +migrate Down
-- 回滚：删除索引
DROP INDEX IF EXISTS idx_token_usage_user_backend;
DROP INDEX IF EXISTS idx_token_usage_model_date;
DROP INDEX IF EXISTS idx_token_quotas_reset_date;
DROP INDEX IF EXISTS idx_clash_rules_user_id;
