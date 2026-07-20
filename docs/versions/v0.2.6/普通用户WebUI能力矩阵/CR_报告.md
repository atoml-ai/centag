# CR 报告 — 普通用户 WebUI 能力矩阵

> 日期：2026-07-20 | 审查人：AI（待人工复核）  
> 审查范围：`feature/v0.2.6` 工作区（capabilities / nav / edition 路由 / Memory / storage `teamAdminWriteOnly` / UI 浏览器验收）  
> 关联：`技术方案.md` / `任务计划.md` / `开发风险评估.md` / `自测记录.md` / `UI测试流程.md`

## 门禁检查报告 — Gate 3

| 检查项 | 状态 | 证据 |
|--------|------|------|
| G3.1 全量单元测试通过 | ✅ | `npm run test:ui-caps` ✅；`bash scripts/ci-go-packages.sh \| xargs go test -count=1` ✅（0 FAIL）；`apps/proxyctl` ✅。Step 5 已修 scheduler 价格期望与 setup-status 环境耦合 |
| G3.2 覆盖率达标 | ✅ | 前端表驱动 selftest；Go `teamAdminWriteOnly` + useraccess；关键路径有边界用例 |
| G3.3 lint 无新增 | ✅ | `web` `npm run lint:ci` ✅；`make lint` **0 issues**；`make harness-check` ✅ |
| G3.4 人工检查点 | ⚠️ | 自测记录 Gate 4 抽查项待勾选 |

**结论**：Gate 3 **通过**。合并前请用户完成 Gate 4 人工抽查。

---

## 审查维度

| 维度 | 结果 | 说明 |
|------|------|------|
| 架构合规 | ✅ | UI 差异收敛到 `getCapabilities`；personal/team_user 同源 `buildWorkerNav`；backends/pipelines API 未分叉；无平行页面 |
| 反模式检测 | ✅ | 未新建 `*Personal.vue` / ChatAdmin；`chatNav` 不进入普通用户菜单；路由仅列表精确 redirect，深链放行 |
| 命名规范 | ✅ | capability / nav id 与现有风格一致 |
| 错误处理 | ✅ | Go：`teamAdminWriteOnly` 对 team normal 返回 403；前端 redirect 无静默吞错 |
| 测试覆盖 | ✅ | `test:ui-caps` + edition_middleware / useraccess；T8 Playwright 三发行版 |
| 风险闭环 | ✅ | Critical=0；High R01–R05 均有实现 + 浏览器验证证据 |

## Critical 问题

| # | 问题描述 | 位置 | 状态 |
|---|---------|------|------|
| — | 无 | — | — |

## High / 应关注（非阻断）

| # | 问题 | 位置 | 状态 / 建议 |
|---|------|------|-------------|
| H1 | `TestModelPriceTable` 价格期望过期 | `scorer_test.go` | **已修复**（对齐 `price_table`） |
| H2 | `TestGetSetupStatus_LocalMode` 读环境 | `proxy_setup_status_test.go` | **已修复**（`t.Setenv` + 正向用例） |
| H3 | `ci-go-packages` 未覆盖 go.work | `scripts/ci-go-packages.sh` | **已修复** |
| H4 | U13 测试对话发送未强断言 | T8 skip | 可接受残余；需稳定 mock 后再强断言 |

## 与方案一致性（抽样）

| 方案要点 | 实现 |
|----------|------|
| backends/pipelines API 全版统一 | 未删 handler；首页仍 Provider/流水线面板 |
| 对话仅流水线「测试」；含 team_admin | `pipelineTestChat` / `liteChatDrawer` 全角色 true；U12 绿 |
| personal ≈ team_user 产品面 | `buildWorkerNav` 同源；top-level nav id 一致 |
| 存储配置 personal+admin；记忆查询 personal+user | capability + Memory mode；U06–U08 |
| `/chat` 不停留；列表 redirect；深链放行 | `resolveCapabilityRouteRedirect` + U14/U15 |
| 最大复用、无平行页 | CR `rg` 未见平行聊天/角色专用页 |

## 产物完整性

- [x] 技术方案已落盘
- [x] 开发风险评估已落盘（Critical=0；High 已关闭）
- [x] 任务计划已落盘
- [x] 自测记录已落盘
- [x] CR 报告已落盘（本文件）
- [x] 代码 + 测试已就绪（含 `test:ui-caps`）
- [x] UI 测试流程 / `centag-ui-browser-test` 正本
- [x] `docs/guide/multi-tenant.md` 已同步

## 结论

- [x] **批准（待人工 Gate 4 抽查）** — 自动化 / lint / 浏览器 P0 已绿；无 Critical。  
- [ ] **需修改**  
- [ ] **拒绝**

## 人工复核清单（请回复确认）

1. [ ] 抽查 personal / team_user / team_admin UI 符合能力矩阵  
2. [ ] 知悉 U13 skip（发送响应未强断言）  
3. [ ] Gate 4 准出确认  
