-- +migrate Up
-- Migration: 037_widen_api_key_prefix
-- Description: key_prefix 改为展示脱敏串（llmproxy_xxxxxxxx…xxxxxx，约 45 字符），原 VARCHAR(16) 不够
-- Date: 2026-08-06

ALTER TABLE api_keys ALTER COLUMN key_prefix TYPE VARCHAR(64);

-- +migrate Down
ALTER TABLE api_keys ALTER COLUMN key_prefix TYPE VARCHAR(16);
