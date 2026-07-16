-- ============================================
-- Migration: 002_add_token_usage_indexes
-- Description: 添加 Token 计量相关索引
-- ============================================

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_token_usage_user_backend 
ON token_usage(user_id, backend_id, created_at);

CREATE INDEX IF NOT EXISTS idx_token_usage_model_date
ON token_usage(model, date(created_at));

CREATE INDEX IF NOT EXISTS idx_token_quotas_reset_date 
ON token_quotas(reset_date);

-- +migrate Down
DROP INDEX IF EXISTS idx_token_usage_user_backend;
DROP INDEX IF EXISTS idx_token_usage_model_date;
DROP INDEX IF EXISTS idx_token_quotas_reset_date;
