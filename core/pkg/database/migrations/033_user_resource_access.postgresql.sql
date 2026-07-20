-- Team: per-user shared resource whitelist + self-service flags
ALTER TABLE users ADD COLUMN IF NOT EXISTS allowed_backend_ids TEXT DEFAULT '[]';
ALTER TABLE users ADD COLUMN IF NOT EXISTS allowed_model_ids TEXT DEFAULT '[]';
ALTER TABLE users ADD COLUMN IF NOT EXISTS allowed_pipeline_ids TEXT DEFAULT '[]';
ALTER TABLE users ADD COLUMN IF NOT EXISTS can_add_own_backends BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS can_add_own_pipelines BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS can_change_default_pipeline BOOLEAN DEFAULT TRUE;
