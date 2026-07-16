-- +migrate Up
-- 给流水线表添加快捷码字段，并迁移现有 mode_mappings 数据

-- 1. 添加快捷码列（唯一，允许 NULL 表示未设置）
ALTER TABLE pipelines ADD COLUMN shortcut_code TEXT UNIQUE;

-- 2. 根据已有 mode_mappings 初始化内置流水线的快捷码
UPDATE pipelines SET shortcut_code = '#d' WHERE id = 'direct-backend';
UPDATE pipelines SET shortcut_code = '#s' WHERE id = 'smart-scheduling';
UPDATE pipelines SET shortcut_code = '#f' WHERE id = 'fallback-mode';
UPDATE pipelines SET shortcut_code = '#o' WHERE id = 'optimize-mode';
UPDATE pipelines SET shortcut_code = '#a' WHERE id = 'audit-mode';
UPDATE pipelines SET shortcut_code = '#m' WHERE id = 'model-matching';
UPDATE pipelines SET shortcut_code = '#r' WHERE id = 'router-mode';
UPDATE pipelines SET shortcut_code = '#ag' WHERE id = 'aggregator-mode';
UPDATE pipelines SET shortcut_code = '#l' WHERE id = 'translate-mode';
UPDATE pipelines SET shortcut_code = '#p' WHERE id = 'pipeline-mode';
UPDATE pipelines SET shortcut_code = '#t' WHERE id = 'transparent-proxy';
-- #c 意图分类已合并到 router-mode，不再单独映射
-- UPDATE pipelines SET shortcut_code = '#c' WHERE id = 'router-mode' AND shortcut_code IS NULL;
UPDATE pipelines SET shortcut_code = '#mem0' WHERE id = 'mem0-memory';

-- +migrate Down
ALTER TABLE pipelines DROP COLUMN IF EXISTS shortcut_code;
