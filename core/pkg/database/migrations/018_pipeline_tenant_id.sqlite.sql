-- +migrate Up
-- 流水线表增加 tenant_id（NULL = 系统共享预设）
ALTER TABLE pipelines ADD COLUMN tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_pipelines_tenant_id ON pipelines(tenant_id);

-- +migrate Down
-- SQLite 不支持 DROP COLUMN，生产回滚需手动处理