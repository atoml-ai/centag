-- +migrate Up
-- Migration: 020_usage_dept_tag
-- Description: Department / cost-center tag on token usage rows
-- Date: 2026-06-12

ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS dept_tag VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_token_usage_dept_tag ON token_usage(dept_tag) WHERE dept_tag IS NOT NULL;

-- +migrate Down
DROP INDEX IF EXISTS idx_token_usage_dept_tag;
ALTER TABLE token_usage DROP COLUMN IF EXISTS dept_tag;