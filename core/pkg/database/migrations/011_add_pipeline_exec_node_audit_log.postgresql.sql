-- +migrate Up
-- 为 pipeline_executions 表补充 node_audit_log 字段
ALTER TABLE pipeline_executions ADD COLUMN node_audit_log TEXT;

-- +migrate Down
ALTER TABLE pipeline_executions DROP COLUMN node_audit_log;
