-- +migrate Up
-- Migration: 020_usage_dept_tag
-- Description: Department / cost-center tag on token usage rows
-- Date: 2026-06-12

ALTER TABLE token_usage ADD COLUMN dept_tag TEXT;
CREATE INDEX IF NOT EXISTS idx_token_usage_dept_tag ON token_usage(dept_tag);

-- +migrate Down
DROP INDEX IF EXISTS idx_token_usage_dept_tag;
ALTER TABLE token_usage DROP COLUMN dept_tag;