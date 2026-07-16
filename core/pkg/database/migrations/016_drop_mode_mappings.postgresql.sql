-- +migrate Up
-- 废弃 mode_mappings 表，快捷码已迁移到 pipelines 表
DROP TABLE IF EXISTS mode_mappings;

-- +migrate Down
-- 如需回退，需手动重建（此处不自动重建，因为数据结构已变更）
