-- +migrate Up
-- Migration: 037_widen_api_key_prefix
-- Description: SQLite api_keys.key_prefix 已是 TEXT，无需扩容；占位保持版本对齐
-- Date: 2026-08-06

SELECT 1;

-- +migrate Down
SELECT 1;
