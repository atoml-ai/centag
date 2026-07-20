-- Team: per-user shared resource whitelist + self-service flags
ALTER TABLE users ADD COLUMN allowed_backend_ids TEXT DEFAULT '[]';
ALTER TABLE users ADD COLUMN allowed_model_ids TEXT DEFAULT '[]';
ALTER TABLE users ADD COLUMN allowed_pipeline_ids TEXT DEFAULT '[]';
ALTER TABLE users ADD COLUMN can_add_own_backends INTEGER DEFAULT 1;
ALTER TABLE users ADD COLUMN can_add_own_pipelines INTEGER DEFAULT 1;
ALTER TABLE users ADD COLUMN can_change_default_pipeline INTEGER DEFAULT 1;
