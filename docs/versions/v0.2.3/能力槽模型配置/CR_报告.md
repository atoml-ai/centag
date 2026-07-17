# CR 报告 — 能力槽模型配置

> 日期：2026-07-17（**二次审查**） | 审查人：AI（待人工复核）  
> 审查范围：`feature/v0.2.3`（能力槽主线已提交；含复跑补测与同分支后续改动）  
> 关联：`技术方案.md` / `任务计划.md` / `开发风险评估.md` / `自测记录.md`

## 变更摘要

| 区域 | 内容 |
|------|------|
| `web/src/utils/capabilitySlots.ts` | Slot 契约、发现、绑定、推荐、新增分类；复跑加固关键词冲突校验 |
| `web/.../CapabilitySlotsDialog.vue` | 通用「配置模型」面板 |
| `web/.../AddCategoryDialog.vue` | 画布「新增分类」向导 |
| 入口 | HomePipelineCard / PipelineModes / PipelineEditorDialog |
| 兼容层 | `routeModelAssign.ts`、`RouterModelAssignDialog.vue` 薄包装 |
| 模板 | router-mode / education / coding 能力槽样板 |
| minimal 落盘 | `FilePipelineStore` → `data/pipeline-templates`（`0b473ab`） |
| 测试 | selftest 增强；`TestFilePipelineStore_ReloadFromDisk`；team 写权限中间件测 |
| 文档 | versions/v0.2.3/*、proxy-modes |

### 同分支后续（纳入回归，非本需求核心）

| 区域 | 内容 |
|------|------|
| Provider UI | `ProviderManagerPanel` 统一列表；full `POST /backends/import`；minimal `probe-all` |
| team 写权限 | `teamAdminWriteOnly`（工作区待提交）+ 前端只读 |

## 审查维度

| 维度 | 结果 | 说明 |
|------|------|------|
| 架构合规 | ✅ | 前端纯函数 + 现有 PUT 流水线；无跨层；Provider 写权限仅 team 收紧 |
| 反模式检测 | ✅ | 配置模型不改 routes；推荐不静默 PUT；关键词不静默覆盖 |
| 命名规范 | ✅ | 与方案一致 |
| 错误处理 | ✅ | 绑定/加分类抛可读中文错误 |
| 测试覆盖 | ✅ | selftest + Go 模板契约 + FileStore 重启 + 全量 `go test ./...` |
| 风险闭环 | ✅ | High R01/R02/R03/R11 均已关闭且有复跑证据 |

## 对照非目标

| 非目标 | 结论 |
|--------|------|
| 配置模型弹窗内改拓扑 | ✅ 未实现 routes CRUD |
| 运行时 LLM 选型 | ✅ 仅静态 tags 打分 |
| team 配额改造 | ✅ 未触及（仅 backends 写权限） |
| 自动改写用户库旧 coding | ✅ 仅模板 + 文档说明 |

## Critical 问题

| # | 问题描述 | 位置 | 状态 |
|---|---------|------|------|
| — | 无 | — | Critical = 0 |

## 非阻断观察（建议，不挡合并）

| # | 说明 | 建议 |
|---|------|------|
| 1 | 前端测试依赖 `npx tsx` selftest | CI 固定调用 `npm run test:capability-slots` |
| 2 | G5 端到端「发请求命中新分类」需联调 | 合并前/后手工补一轮 |
| 3 | 已落库旧单节点 `coding-agent` 不静默升级 | 发布说明强调从模板重建 |
| 4 | team 后端写权限改动尚在工作区 | 建议单独 commit 后再合并 |

## 产物完整性

- [x] 技术方案已落盘
- [x] 开发风险评估已落盘且无开放 Critical（High 均已关闭）
- [x] 任务计划已落盘
- [x] 自测记录已落盘（二次复跑）
- [x] CR 报告已落盘（本文件，二次）
- [x] 代码 + 测试已就绪（能力槽主线已 commit；team 写权限待提交）

## Gate 3 证据（复跑 2026-07-17）

| 检查 | 结果 |
|------|------|
| `make lint` | ✅ 0 issues |
| `make harness-check` | ✅ |
| `cd web && npm run lint` | ✅ |
| `npm run test:capability-slots` | ✅ |
| `go test ./pkg/pipeline/ -run 'TestCapabilitySlots\|TestFilePipelineStore'` | ✅ |
| `go test ./pkg/server/ -run TestTeamAdminWriteOnly` | ✅ |
| `cd core && go test ./...` | ✅ |

## 结论

- [x] **批准（AI）** — 二次审查通过，Gate 3 证据齐全
- [ ] **人工复核** — 待用户确认后准出 Gate 4
- [ ] 需修改
- [ ] 拒绝
