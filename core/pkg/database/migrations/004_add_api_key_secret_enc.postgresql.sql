-- ============================================
-- Migration: 004_add_api_key_secret_enc
-- Description: 可选加密存储完整 API Key，供登录后反复查看
-- ============================================

-- +migrate Up
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS key_secret_enc TEXT;
