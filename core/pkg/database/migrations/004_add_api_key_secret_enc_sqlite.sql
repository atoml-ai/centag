-- ============================================
-- Migration: 004_add_api_key_secret_enc
-- Description: 添加 API Key 加密字段
-- ============================================

-- +migrate Up
ALTER TABLE api_keys ADD COLUMN key_secret_enc TEXT;

-- +migrate Down
-- SQLite 不支持 DROP COLUMN
