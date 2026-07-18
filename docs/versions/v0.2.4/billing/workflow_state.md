# 工作流状态 — 计费功能

> 版本：v0.2.4 | 需求：计费功能（规则引擎、YAML、计量成本归因、管理 API） | 分支：`feature/v0.2.4`  
> 创建日期：2026-07-17 | 最后更新：2026-07-18

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 产物路径 |
|------|------|:----:|:--------:|---------|
| **Phase 1** 方案设计 | Step 1: 方案设计与确认 | ✅ | 2026-07-18 | `docs/versions/v0.2.4/billing/技术方案.md` + `开发风险评估.md` |
| **Phase 2** 任务规划 | Step 2: 任务规划 | ✅ | 2026-07-18 | `docs/versions/v0.2.4/billing/任务计划.md` |
| **Phase 3** 编码实现 | Step 3: SDD 编码实现 | ✅ | 2026-07-18 | `core/internal/billing/`、`tokenusage/`、`pkg/server/`、migrations、`config/pricing/` |
| | Step 4: 单元测试补全 | ⬜ | | 对应 `*_test.go`（核心单测已随 T1–T5 落地，Step 4 可补覆盖率） |
| **Phase 4** 质量交付 | Step 5: CR 审查 | ⬜ | | `docs/versions/v0.2.4/billing/` |

## 门禁状态

| 门禁 | 位置 | 状态 | 通过日期 | 备注 |
|------|------|:----:|:--------:|------|
| Gate 1 | Phase 1 → 2 | ✅ | 2026-07-18 | 复审修订后：Critical=0；High 均有应对 |
| Gate 2 | Phase 2 → 3 | ✅ | 2026-07-18 | 任务可执行验收 + High 全映射 |
| Gate 3 | Phase 3 → 4 | ⬜ | | Step 4 补测 + 全量通过后置位 |
| Gate 4 | 交付准出 | ⬜ | | |

## 产物清单

- [x] **技术方案** → `docs/versions/v0.2.4/billing/技术方案.md`
- [x] **开发风险评估** → `docs/versions/v0.2.4/billing/开发风险评估.md`
- [x] **任务计划** → `docs/versions/v0.2.4/billing/任务计划.md`
- [ ] **自测记录** → `docs/versions/v0.2.4/billing/自测记录.md`
- [ ] **CR 报告** → `docs/versions/v0.2.4/billing/CR_报告.md`
- [x] **代码 + 测试** → `core/internal/billing/`、`tokenusage/`、`pkg/server/`、`config/pricing/`、migrations 031/032

## Step 3 任务完成情况

| 任务 | 状态 | 验证 |
|------|:----:|------|
| T1 迁移 + YAML + ephemeral | ✅ | `go test ./pkg/database/ -run Migration031` |
| T2 RuleStore | ✅ | `go test ./internal/billing/... -run TestRuleStore` |
| T3 PricingService | ✅ | `go test ./internal/billing/... -run TestPricing` |
| T4 tokenusage 接入 | ✅ | `go test ./internal/tokenusage/...` |
| T5 管理 API | ✅ | `go test ./pkg/server/ -run Billing` |
| T6 文档与非目标验收 | ✅ | `docs/api/billing.md`、`docs/guide/billing.md`；无 `/billing/costs*` |

## 决策日志

| 日期 | 决策 | 原因 | 提出人 |
|------|------|------|--------|
| 2026-07-17 | v0.2.4 范围：规则 + 计量归因 + 统计/管理，不做钱包/支付/Web | 版本边界 | 用户 |
| 2026-07-17 | 定价键：`backend_id + model` | 与业界及现网 price_table 一致 | 用户 |
| 2026-07-17 | 规则用 YAML，支持导入导出 | 版本管理与预置发布 | 用户 |
| 2026-07-18 | 代码路径以 `core/` 为准；管理 API 落在 `core/pkg/server/` | 对齐现网真源 | 复审 |
| 2026-07-18 | 新增 `PricingService`/`RuleStore`，不复用 Event 型 `billing.Service` | 避免与异步钩子冲突 | 复审 |
| 2026-07-18 | 保留列名 `cost_usd`，金额为规则币种（默认 CNY），API 增 `currency` | 兼容既有契约，避免大迁移 | 复审 |
| 2026-07-18 | 正本改为 USD；YAML/`cost_usd`/估算均为美元；`usd_to_cny` 仅前端显示换算 | 用户要求汇率与美元本位 | 用户 |
| 2026-07-18 | 成本查询增强既有 `/api/v1/admin/cost/summary`，不新建 `/billing/costs*` | 避免双入口 | 复审 |
| 2026-07-18 | 不改 `auth` 预算与 `proxy` 主路径 | 非目标 / 降回归风险 | 复审 |
| 2026-07-18 | minimal 用量继续 ephemeral SQLite；仅规则用 MemoryRuleStore | 对齐 `NewEphemeralService` | 复审 |
| 2026-07-18 | Step 3 落地 T1–T6 | 按任务计划编码 | AI |
| 2026-07-18 | minimal 补 Web：计费规则页 + 成本看板 + 概览成本 | 用户反馈看不到计费入口 | 用户/AI |
