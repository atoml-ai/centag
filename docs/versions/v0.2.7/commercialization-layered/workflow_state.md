# 工作流状态 — 商业化分层（Open Core）

> 版本：v0.2.7 | 需求：commercialization-layered | 分支：v0.2.7
> 创建日期：2026-07-20 | 最后更新：2026-07-20

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 产物路径 |
|------|------|:----:|:--------:|---------|
| **Phase 1** 方案设计 | Step 1: 方案设计与确认 | ✅ | 2026-07-20 | `docs/versions/v0.2.7/commercialization-layered/技术方案.md` |
| **Phase 2** 任务规划 | Step 2: 任务规划 | ⬜ | | `docs/versions/v0.2.7/commercialization-layered/任务计划.md` |
| **Phase 3** 编码实现 | Step 3: SDD 编码实现 | ⬜ | | 对应 `internal/` / `plugins/` / `web/` |
| | Step 4: 单元测试补全 | ⬜ | | 对应 `*_test.go` |
| **Phase 4** 质量交付 | Step 5: CR 审查 | ⬜ | | `docs/versions/v0.2.7/commercialization-layered/` |

> 状态标记：⬜ = 未开始 | 🔄 = 进行中 | ✅ = 已完成

## 门禁状态

| 门禁 | 位置 | 状态 | 通过日期 | 备注 |
|------|------|:----:|:--------:|------|
| Gate 1 | Phase 1 → 2（技术方案落盘 + 内审通过） | ⬜ | | 待 step2-plan 触发 |
| Gate 2 | Phase 2 → 3（任务计划落盘 + 可执行验收标准） | ⬜ | | |
| Gate 3 | Phase 3 → 4（测试通过 + 覆盖率达标） | ⬜ | | |
| Gate 4 | 交付准出（CR Critical = 0 + 产物齐全） | ⬜ | | |

## 产物清单

- [x] **技术方案** → `docs/versions/v0.2.7/commercialization-layered/技术方案.md`
- [x] **开发风险评估** → `docs/versions/v0.2.7/commercialization-layered/开发风险评估.md`
- [ ] **任务计划** → `docs/versions/v0.2.7/commercialization-layered/任务计划.md`
- [ ] **自测记录** → `docs/versions/v0.2.7/commercialization-layered/自测记录.md`
- [ ] **CR 报告** → `docs/versions/v0.2.7/commercialization-layered/CR_报告.md`
- [ ] **代码 + 测试** → 对应 `core/` / `plugins/` / `web/`

## 决策日志

| 日期 | 决策 | 原因 | 提出人 |
|------|------|------|--------|
| 2026-07-20 | 采用 `centag-business` 独立闭源仓库承载 team 增值 | 单代码库维护核心、免费版真正开源、team 闭源三约束同时满足 | 用户 |
| 2026-07-20 | 核心仓完全开源（minimal/personal/team 壳开源），增值闭源 | 开源引流 + 商业化，避免双份代码维护 | 用户 |

---

## 使用说明

### 何时更新
- 每个 Step 完成时：更新对应步骤状态为 ✅，记录完成日期
- 进入新 Step 时：更新当前步骤状态为 🔄
- 通过门禁时：更新门禁状态为 ✅，记录通过日期
- 有重要决策时：追加到决策日志

### 文件位置

```
docs/versions/v0.2.7/commercialization-layered/
├── workflow_state.md        ← 本文件
├── 技术方案.md               ← Step 1 产出
├── 开发风险评估.md           ← Step 1 强制产出
├── 任务计划.md               ← Step 2 产出
├── 自测记录.md               ← Step 5 产出
└── CR_报告.md               ← Step 5 产出
```
