# 工作流状态 — Provider/Agent 本地接入与账户池

> 版本：v0.2.9 | 需求：Provider/Agent 本地接入与账户池 | 分支：feature/v0.2.9
> 创建日期：2026-07-23 | 最后更新：2026-07-23

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 产物路径 |
|------|------|:----:|:--------:|---------|
| **Phase 1** 方案设计 | Step 1: 方案设计与确认 | ✅ | 2026-07-23 | `docs/versions/v0.2.9/provider-agent-local/技术方案.md` |
| **Phase 2** 任务规划 | Step 2: 任务规划 | ✅ | 2026-07-23 | `docs/versions/v0.2.9/provider-agent-local/任务计划.md` |
| **Phase 3** 编码实现 | Step 3: SDD 编码实现 | ✅ | 2026-07-23 | 对应 `core/` / `plugins/` / `web/` |
| | Step 4: 单元测试补全 | ✅ | 2026-07-23 | 对应 `*_test.go` |
| **Phase 4** 质量交付 | Step 5: CR 审查 | ✅ | 2026-07-23 | `docs/versions/v0.2.9/provider-agent-local/CR_报告.md` |
| **Phase 5** 发版 | Step 6: 发版 | ⬜ | | GitHub Release `v0.2.9`（业务版） |

> 状态标记：⬜ = 未开始 | 🔄 = 进行中 | ✅ = 已完成

## 门禁状态

| 门禁 | 位置 | 状态 | 通过日期 | 备注 |
|------|------|:----:|:--------:|------|
| Gate 1 | Phase 1 → 2（技术方案落盘 + 内审通过） | ✅ | 2026-07-23 | AI 内审 Critical=0；**人工已确认** |
| Gate 2 | Phase 2 → 3（任务计划落盘 + 可执行验收标准） | ✅ | 2026-07-23 | 12 个任务已拆解；High 风险全部映射 |
| Gate 3 | Phase 3 → 4（测试通过 + 覆盖率达标） | ✅ | 2026-07-23 | 所有测试通过，覆盖率达标 |
| Gate 4 | Phase 4 → 5（CR 准出 + **人工批准可发版**） | ✅ | 2026-07-23 | 人工确认发版许可 |
| Gate 5 | 发版准出（Release 资产 + 冒烟） | ⬜ | | npm 搭便车已发 `@atomlai/centag@0.2.9`；业务版待 Step 6 |

## 产物清单

- [x] **技术方案** → `docs/versions/v0.2.9/provider-agent-local/技术方案.md`
- [x] **开发风险评估** → `docs/versions/v0.2.9/provider-agent-local/开发风险评估.md`
- [x] **任务计划** → `docs/versions/v0.2.9/provider-agent-local/任务计划.md`
- [x] **自测记录** → `docs/versions/v0.2.9/provider-agent-local/自测记录.md`
- [x] **CR 报告** → `docs/versions/v0.2.9/provider-agent-local/CR_报告.md`
- [x] **代码 + 测试** → 对应 `core/` / `plugins/` / `web/` / `apps/wrap/`
- [x] **npm 搭便车**（已完成）→ `@atomlai/centag@0.2.9`（见 `npm-packaging/`）
- [ ] **GitHub Release**（Step 6 业务版）→ `v0.2.9` 含二进制变更

## 任务进度

| 任务 | 状态 | 完成日期 | 备注 |
|------|:----:|:--------:|------|
| T1: 账户池数据模型与 API | ✅ | 2026-07-23 | BackendAccount、AccountPoolConfig、5 个 CRUD API |
| T2: 错误率熔断扩展 | ✅ | 2026-07-23 | ErrorRateThreshold、MinRequestsInWindow、OR 逻辑 |
| T3: 账户池轮询/最少连接/粘性路由 | ✅ | 2026-07-23 | 流式/非流式一致性测试、并发安全测试 |
| T4: 回归测试套件 | ✅ | 2026-07-23 | NilAccountPoolCompat、429Failover、ScopedBreaker |
| T5: Provider 管理增强 UI | ✅ | 2026-07-23 | Provider 预设扩充至 42 个 |
| T6: 账户池 API 与安全 | ✅ | 2026-07-23 | Export脱敏、账户熔断重置API、安全测试 |
| T7: 熔断器状态展示与手动重置 | ✅ | 2026-07-23 | AgentSetup Provider优先、Backend选择 |
| T8: Agent 接入联动与 HotSwap | ✅ | 2026-07-23 | HotSwap功能、AgentProviders增强 |
| T9: Provider 预设扩充 | ✅ | 2026-07-23 | 已在T5中完成，42个Provider |
| T10: Raw 流水线优化 | ✅ | 2026-07-23 | RedirectPolicy、MaxRedirects、followRedirect |
| T11: 前端可配置性与入口优化 | ✅ | 2026-07-23 | Agent Setup 提升为一级菜单 |
| T12: 文档更新 | ✅ | 2026-07-23 | API文档更新、账户池API文档 |

## 决策日志

| 日期 | 决策 | 原因 | 提出人 |
|------|------|------|--------|
| 2026-07-23 | v0.2.9 主目标定为账户池 + 错误率熔断 + 本地接入 + Provider/Agent 体验 | 对标 cc-switch/opencodex 个人用户场景；npm 仅为搭便车补丁 | 用户 |
| 2026-07-23 | 不做 MCP/Skills/Prompts 跨工具同步 | cc-switch 战场；Centag 聚焦服务端网关 | AI |
| 2026-07-23 | 本地接入优先 MITM 透明路径，配置写入为备选 | Centag 差异化：Agent 零改 base_url | AI |
| 2026-07-23 | 账户池挂在 BackendConfig，复用 per-backend 熔断器 | 最小侵入；与现有 FallbackBackends 互补 | AI |
| 2026-07-23 | Provider 预设扩充至 40+，对齐 cc-switch 常用列表 | 降低个人用户配置门槛 | AI |
| 2026-07-23 | 新增 G6（Raw 流水线优化）、G7（前端可配置性与入口优化） | 用户要求：raw 流水线只做转发 + 前端入口按使用频率调整 | 用户 |
| 2026-07-23 | **G6 改向**：取消 raw-forward，改为 fixed-egress（`#j` 跳板）；`#d/#t/#j` 共用 `transparent_forward` 开关 | 产品确认 raw 无独立场景；三条出站流水线统一节点 | 用户 |
| 2026-07-23 | Agent Setup 提升为一级菜单，置于本机代理之后 | 使用频率高于存储/系统配置，用户快速访问 | AI |
