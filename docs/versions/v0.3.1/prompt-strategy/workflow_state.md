# 工作流状态 — prompt-strategy

> 版本：v0.3.1 | 需求：prompt-strategy | 分支：`feature/v0.3.1`
> 创建日期：2026-07-25 | 最后更新：2026-07-25

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 产物路径 |
|------|------|:----:|:--------:|---------|
| **Phase 1** 方案设计 | Step 1: 方案设计与确认 | ✅ | 2026-07-25 | `docs/versions/v0.3.1/prompt-strategy/技术方案.md` |
| **Phase 2** 任务规划 | Step 2: 任务规划 | 🔄 | | `docs/versions/v0.3.1/prompt-strategy/任务计划.md`（已落盘，待人工确认） |
| **Phase 3** 编码实现 | Step 3: SDD 编码实现 | ⬜ | | `core/pkg/pipeline/` 等 |
| | Step 4: 单元测试补全 | ⬜ | | 对应 `*_test.go` |
| **Phase 4** 质量交付 | Step 5: CR 审查 | ⬜ | | `docs/versions/v0.3.1/prompt-strategy/` |
| **Phase 5** 发版 | Step 6: 发版 | ⬜ | | GitHub Release `v0.3.1` |

> 状态标记：⬜ = 未开始 | 🔄 = 进行中 | ✅ = 已完成

## 门禁状态

| 门禁 | 位置 | 状态 | 通过日期 | 备注 |
|------|------|:----:|:--------:|------|
| Gate 1 | Phase 1 → 2（技术方案落盘 + 内审通过） | ✅ | 2026-07-25 | 用户要求建分支并进入 step2-plan，视为方案与风险确认 |
| Gate 2 | Phase 2 → 3（任务计划落盘 + 可执行验收标准） | ⬜ | | |
| Gate 3 | Phase 3 → 4（测试通过 + 覆盖率达标） | ⬜ | | |
| Gate 4 | Phase 4 → 5（CR 准出 + **人工批准可发版**） | ⬜ | | 未人工确认不得 ✅ |
| Gate 5 | 发版准出（Release 资产 + 冒烟） | ⬜ | | |

## 产物清单

- [x] **技术方案** → `docs/versions/v0.3.1/prompt-strategy/技术方案.md`
- [x] **开发风险评估** → `docs/versions/v0.3.1/prompt-strategy/开发风险评估.md`（Critical=0）
- [x] **前端配置操作说明** → `docs/versions/v0.3.1/prompt-strategy/前端配置操作说明.md`
- [x] **任务计划** → `docs/versions/v0.3.1/prompt-strategy/任务计划.md`（待人工确认后 Gate 2）
- [ ] **自测记录** → `docs/versions/v0.3.1/prompt-strategy/自测记录.md`
- [ ] **CR 报告** → `docs/versions/v0.3.1/prompt-strategy/CR_报告.md`
- [ ] **代码 + 测试** → 对应 `core/` / `plugins/` / `web/` / `config/`
- [ ] **GitHub Release**（Step 6）→ `v0.3.1`

## 决策日志

| 日期 | 决策 | 原因 | 提出人 |
|------|------|------|--------|
| 2026-07-25 | 版本 v0.3.1 / 需求名 prompt-strategy | 用户确认 | 用户 |
| 2026-07-25 | 覆盖 system / user / 输出后处理；分阶段（框架+基础能力优先，预留专用 LLM） | 用户确认 | 用户 |
| 2026-07-25 | 补前端配置操作说明（画布 + 抽屉示意） | 方案前端缺口 | 用户/AI |
| 2026-07-25 | Web 配置定为 Phase A **必做**（验收入口）；路线增 A4 | 用户确认：无 Web 无法测试 | 用户 |
| 2026-07-25 | 不区分流水线；凡涉及 system/user 设置与替换均可配置策略 | 需求核心 | 用户 |
