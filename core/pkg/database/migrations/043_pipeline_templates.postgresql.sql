-- Pipeline templates from remote public configsync snapshot
CREATE TABLE IF NOT EXISTS pipeline_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    shortcut_code TEXT,
    schema_version TEXT,
    version TEXT,
    edition TEXT DEFAULT 'all',
    nodes JSONB NOT NULL DEFAULT '[]',
    global_config JSONB,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pipeline_templates_shortcut ON pipeline_templates(shortcut_code);
CREATE INDEX IF NOT EXISTS idx_pipeline_templates_edition ON pipeline_templates(edition);
