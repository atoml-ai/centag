-- +migrate Up
-- Migration: 038_token_usage_source
-- Description: token_usage 添加 source 字段区分真实/估算 token 用量
-- Date: 2026-08-13

ALTER TABLE token_usage ADD COLUMN source TEXT NOT NULL DEFAULT 'real';

-- +migrate Down
ALTER TABLE token_usage DROP COLUMN source;
