# 工作流状态 — 普通用户 WebUI 能力矩阵

> 版本：v0.2.6 | 需求：普通用户 WebUI 能力矩阵 | 分支：`feature/v0.2.6`  
> 创建日期：2026-07-20 | 最后更新：2026-07-20（既有单测债已修；Gate 4 待人工）

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 产物路径 |
|------|------|:----:|:--------:|---------|
| **Phase 1** 方案设计 | Step 1: 方案设计与确认 | ✅ | 2026-07-20 | `技术方案.md` + `开发风险评估.md` |
| **Phase 2** 任务规划 | Step 2: 任务规划 | ✅ | 2026-07-20 | `任务计划.md`（T1–T8） |
| **Phase 3** 编码实现 | Step 3: SDD 编码实现 | ✅ | 2026-07-20 | T1–T7 + **T8 Playwright UI 验收通过**（P0 绿） |
| | Step 4: 单元测试补全 | ✅ | 2026-07-20 | `npm run test:ui-caps` + Go `teamAdminWriteOnly` / useraccess |
| **Phase 4** 质量交付 | Step 5: CR 审查 | ✅ | 2026-07-20 | `自测记录.md` + `CR_报告.md`（有条件批准） |

> 状态标记：⬜ = 未开始 | 🔄 = 进行中 | ✅ = 已完成

## 门禁状态

| 门禁 | 位置 | 状态 | 通过日期 | 备注 |
|------|------|:----:|:--------:|------|
| Gate 1 | Phase 1 → 2 | ✅ | 2026-07-20 | 技术方案 + 开发风险评估落盘；Critical=0；含 UI 浏览器验收约定；人工确认通过 |
| Gate 2 | Phase 2 → 3 | ✅ | 2026-07-20 | 人工确认任务计划与风险映射 |
| Gate 3 | Phase 3 → 4 | ✅ | 2026-07-20 | 工作区 `ci-go-packages` 全量 0 FAIL；lint/harness/UI 绿；scheduler/handler/ci 脚本已修 |
| Gate 4 | 交付准出 | ⬜ | | 待人工抽查导航/测试抽屉并确认准出 |

## 产物清单

- [x] **技术方案** → `docs/versions/v0.2.6/普通用户WebUI能力矩阵/技术方案.md`
- [x] **开发风险评估** → `docs/versions/v0.2.6/普通用户WebUI能力矩阵/开发风险评估.md`
- [x] **UI 测试流程（Browser MCP）** → `UI测试流程.md` + 正本 `docs/harness/skills/centag-ui-browser-test.md`
- [x] **任务计划** → `任务计划.md`
- [x] **自测记录** → `自测记录.md`（含 `/tmp/centag_ui_browser_results.json` 摘要）
- [x] **CR 报告** → `CR_报告.md`
- [x] **代码 + 测试** → `web/` capabilities/nav/router + `test:ui-caps`；storage `teamAdminWriteOnly`；T8 浏览器验收绿

## 决策日志

| 日期 | 决策 | 原因 | 提出人 |
|------|------|------|--------|
| 2026-07-20 | Personal 与 Team User 产品面对齐，差异在权限/租户 | 多租户产品原则 | 用户 |
| 2026-07-20 | 普通用户后端/流水线主入口仅首页；Admin 保留独立页 | lite 首页已覆盖多数操作 | 用户 |
| 2026-07-20 | backends/pipelines API 全版统一；非 edition 分叉接口 | 最大复用 | 用户 |
| 2026-07-20 | 对话仅流水线测试入口；含 Team Admin | 取消独立对话导航 | 用户 |
| 2026-07-20 | 存储配置：personal+admin；记忆查询：personal+team_user | 使用场景拆分 | 用户 |
| 2026-07-20 | 分支自 `feature/v0.2.5` 拉出 `feature/v0.2.6` | 承接已有导航/useraccess 基础 | 用户/AI |
| 2026-07-20 | 准出强制 Browser MCP UI 验收；HTTP e2e 不可替代 | UI 操作方式改动大 | 用户 |
| 2026-07-20 | **Gate 1 通过** | 人工确认技术方案与风险评估 | 用户 |
| 2026-07-20 | Step 2 产出 T1–T8（含强制 T8 浏览器验收） | High 风险全部挂任务 | AI |
| 2026-07-20 | **Gate 2 通过**；Step 3 编码 T1–T7 | 人工确认后开工 | 用户/AI |
| 2026-07-20 | T8 UI浏览器测试 **BLOCKED** | 本会话无 Browser MCP（`browser_mcp_unavailable`） | AI |
| 2026-07-20 | T8 改用 Playwright MCP 重跑 | team_admin+team_user+personal+minimal P0 全 pass | AI |
| 2026-07-20 | Step 4 单元测试补全 | 增 nav/dashboard-sections selftest；强化 capabilities/routes | AI |
| 2026-07-20 | Step 5 CR **有条件批准**；Gate 3 附条件通过；Gate 4 待人工 | 既有单测债 + 人工抽查 | AI |
| 2026-07-20 | 修复 scheduler/handler 单测 + `ci-go-packages` 覆盖 go.work | Gate 3 升为无条件通过 | AI |

---

## 使用说明

### 何时更新

- 每个 Step 完成时：更新对应步骤状态为 ✅，记录完成日期
- 进入新 Step 时：更新当前步骤状态为 🔄
- 通过门禁时：更新门禁状态为 ✅，记录通过日期
- 有重要决策时：追加到决策日志
