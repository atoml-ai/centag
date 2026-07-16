-- +migrate Up
-- 流水线配置表
CREATE TABLE IF NOT EXISTS pipelines (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    version TEXT NOT NULL DEFAULT '1.0',
    nodes_json JSONB NOT NULL,
    global_config_json JSONB NOT NULL,
    metadata_json JSONB,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pipelines_enabled ON pipelines(enabled);
CREATE INDEX IF NOT EXISTS idx_pipelines_created ON pipelines(created_at);

-- 流水线执行历史表
CREATE TABLE IF NOT EXISTS pipeline_executions (
    id SERIAL PRIMARY KEY,
    pipeline_id TEXT NOT NULL,
    input_content TEXT,
    output_content TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    duration_ms INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_pipeline_executions_pipeline ON pipeline_executions(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_executions_created ON pipeline_executions(created_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_pipeline_executions_created;
DROP INDEX IF EXISTS idx_pipeline_executions_pipeline;
DROP TABLE IF EXISTS pipeline_executions;

DROP INDEX IF EXISTS idx_pipelines_created;
DROP INDEX IF EXISTS idx_pipelines_enabled;
DROP TABLE IF EXISTS pipelines;
