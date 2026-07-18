# 计费功能指南（v0.2.4）

## 能力概览

- 按 `backend_id + model` 配置输入/输出单价（**美元 / 1M tokens**）
- YAML 预置与导入导出（默认币种 **USD**，含 `usd_to_cny` 汇率）
- Token 计量写入成本（列名 `cost_usd` 存 USD）
- 管理端规则 CRUD；成本查询复用 `/api/v1/admin/cost/summary`
- 前端可切换 **美元 / 人民币** 显示；选人民币时用配置汇率做一次换算（不改正本）

## 发行版差异

| 模式 | 规则存储 | 用量存储 |
|------|----------|----------|
| minimal | 内存（启动读 YAML） | 进程内 ephemeral SQLite |
| personal | SQLite `pricing_rules` | SQLite `token_usage` |
| team | PostgreSQL `pricing_rules` | PostgreSQL `token_usage` |

## 价格解析顺序

1. 启用的 DB/内存规则（priority + 特异性）
2. 硬编码 `ModelPriceTable`（deprecated fallback）
3. 默认 0.7 / 0.7 USD

## 汇率

写在 `config/pricing/default.yaml`：

```yaml
currency: "USD"
usd_to_cny: 7.2
```

导入 YAML 若标记 `currency: CNY`，会按 `usd_to_cny` 换算为 USD 后再入库。导出始终为 USD。

## 非本版本范围

钱包充值、支付、发票、用量预警、租户级折扣、`auth` 预算改造。

详见 `docs/versions/v0.2.4/billing/技术方案.md`。
