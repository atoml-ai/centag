# CR 报告 — 能力槽模型配置

> 日期：2026-07-17 | 审查人：AI（待人工复核）  
> 审查范围：`feature/v0.2.3` 工作区相对 `7101b1d`（v0.2.2 tip）未提交变更  
> 关联：`技术方案.md` / `任务计划.md` / `开发风险评估.md` / `自测记录.md`

## 变更摘要

| 区域 | 内容 |
|------|------|
| `web/src/utils/capabilitySlots.ts` | Slot 契约、发现、绑定、推荐、新增分类纯函数 |
| `web/.../CapabilitySlotsDialog.vue` | 通用「配置模型」面板 |
| `web/.../AddCategoryDialog.vue` | 画布「新增分类」向导 |
| 入口 | HomePipelineCard / PipelineModes / PipelineEditorDialog |
| 兼容层 | `routeModelAssign.ts`、`RouterModelAssignDialog.vue` 薄包装 |
| 模板 | router-mode slots 双写；education 跟随默认 + slots；coding 四阶段重构 |
| 测试 | `capabilitySlots.selftest.ts`、`capability_slots_templates_test.go` |
| 文档 | `proxy-modes.md`、versions/v0.2.3/* |

## 审查维度

| 维度 | 结果 | 说明 |
|------|------|------|
| 架构合规 | ✅ | 前端纯函数 + 现有 PUT 流水线；无新增后端协议；无跨层 |
| 反模式检测 | ✅ | 配置模型不改 routes；推荐不静默 PUT；无业务 panic |
| 命名规范 | ✅ | capabilitySlots / CapabilitySlotsDialog 与方案一致 |
| 错误处理 | ✅ | 绑定/加分类抛可读中文错误；Dialog 捕获并 ElMessage |
| 测试覆盖 | ✅ | 前端表驱动 selftest + Go 三模板契约；`go test ./...` 通过 |
| 风险闭环 | ✅ | High R01/R02/R03/R11 均已关闭且有验证证据 |

## 对照非目标

| 非目标 | 结论 |
|--------|------|
| 配置模型弹窗内改拓扑 | ✅ 未实现 routes CRUD |
| 运行时 LLM 选型 | ✅ 仅静态 tags 打分 |
| team 配额改造 | ✅ 未触及 |
| 自动改写用户库旧 coding | ✅ 仅模板 + 文档说明 |

## Critical 问题

| # | 问题描述 | 位置 | 状态 |
|---|---------|------|------|
| — | 无 | — | Critical = 0 |

## 非阻断观察（建议，不挡合并）

| # | 说明 | 建议 |
|---|------|------|
| 1 | 前端测试依赖 `npx tsx` selftest，未进 CI 固定依赖 | 后续可加 vitest 或 CI 调用 `npm run test:capability-slots` |
| 2 | G5 端到端「发请求命中新分类」需联调环境 | 合并前或合并后手工补一轮 |
| 3 | 已落库旧单节点 `coding-agent` 不会静默升级 | 发布说明强调从模板重建 |

## 产物完整性

- [x] 技术方案已落盘
- [x] 开发风险评估已落盘且无开放 Critical（High 均已关闭）
- [x] 任务计划已落盘
- [x] 自测记录已落盘
- [x] CR 报告已落盘（本文件）
- [x] 代码 + 测试已就绪（尚未 git commit）

## Gate 3 证据

| 检查 | 结果 |
|------|------|
| `make lint` | ✅ 0 issues |
| `make harness-check` | ✅ |
| `cd web && npm run lint` | ✅ |
| `npm run test:capability-slots` | ✅ |
| `go test ./pkg/pipeline/` | ✅ |
| `cd core && go test ./...` | ✅ |

## 结论

- [x] **批准（AI）** — 技术与风险门禁满足，可合并（建议先提交本分支变更）
- [ ] **人工复核** — 待用户确认产物与联调观感后准出 Gate 4
- [ ] 需修改
- [ ] 拒绝
