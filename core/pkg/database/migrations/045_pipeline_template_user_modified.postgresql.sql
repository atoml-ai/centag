-- Add is_user_modified field to track Admin customizations
-- is_user_modified = FALSE: synced from remote, can be auto-updated
-- is_user_modified = TRUE: Admin has customized, should not be auto-updated
ALTER TABLE pipeline_templates ADD COLUMN is_user_modified BOOLEAN DEFAULT FALSE;

-- Add index for efficient filtering
CREATE INDEX IF NOT EXISTS idx_pipeline_templates_user_modified ON pipeline_templates(is_user_modified);
