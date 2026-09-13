-- 047: token_usage 增加 account_id（上游后端账户池 Key 维度）。
-- 记录请求实际使用的 AccountPool 账户 ID（backend_id 内唯一），用于按 Key 分开计量与筛选。
-- 历史行保持 NULL（= 未指定/单 Key 后端），token_usage_daily 聚合表保持不变。

ALTER TABLE token_usage ADD COLUMN account_id VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_token_usage_account_id ON token_usage(account_id);
