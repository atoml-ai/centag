-- +migrate Up
-- Migration: 039_token_usage_session_id
-- Description: token_usage 添加 session_id 字段，支持按会话查询计量计价明细
-- Date: 2026-08-13

ALTER TABLE token_usage ADD COLUMN session_id TEXT;

-- +migrate Down
ALTER TABLE token_usage DROP COLUMN session_id;
