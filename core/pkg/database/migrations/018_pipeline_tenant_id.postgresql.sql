-- +migrate Up
-- 流水线表增加 tenant_id（NULL = 系统共享预设）
ALTER TABLE pipelines ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_pipelines_tenant_id ON pipelines(tenant_id) WHERE tenant_id IS NOT NULL;

-- +migrate Down
-- ALTER TABLE pipelines DROP COLUMN IF EXISTS tenant_id;