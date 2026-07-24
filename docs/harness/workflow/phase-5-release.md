# Phase 5: 发版（Step 6）

> 快捷别名：`step6-release` / 发版 / release / step6

## 目标

在 **Gate 4（CR 发版许可）** 通过后，于版本分支完成用户所选渠道的发布与验收：

- **GitHub Release** — 全平台 cli（install.sh 默认）+ desktop(Win/mac，`--desktop`)
- **npm** — 全平台 cli：`centag` + `centag-offline`
- **CI**（可选）— 推 tag / Actions（desktop 原生 runner）
- 本地打包：`./start.sh package <cli|desktop> <os> [arch]`；未来渠道见 `step6-release/procedure.md` §渠道注册表

安装/部署冒烟默认由用户手动完成。

## 前置门禁

**Gate 4**（见 `docs/harness/workflow/gate-checklist.md`）

> ⚠️ 未通过 Gate 4（含人工未确认批准）→ **拒绝发版**，退回 `step5-review`。

## 执行清单

### 1. 门禁复核

- [ ] `workflow_state.md`：Gate 4 = ✅，Step 5 = ✅
- [ ] `CR_报告.md`：结论为「批准 — 可发版」
- [ ] 当前分支为版本分支：`bash scripts/release/require-release-branch.sh --version <ver>`

### 2. 收参 → 全流程发版

按 `docs/harness/skills/step6-release/SKILL.md` + `.cursor/rules/step6-release.mdc`：

- [ ] **先选渠道**（GitHub / npm / 全部 / build-only / CI）
- [ ] 共用构建（reuse / rebuild / skip，只跑一次）
- [ ] **按序**发版：github → npm（依赖顺序）
- [ ] 鉴权失败时提示 `gh auth` / `npm login`，等用户就绪再继续
- [ ] 不擅自扩大组件（无 minimal/launcher）

### 3. 验收 → Gate 5

- [ ] 各**已选渠道**验收通过（Release 资产 / npm version / CI 绿）
- [ ] **默认不做**本机 install 冒烟（用户手动）；仅用户点名时才跑
- [ ] 更新 `workflow_state`：Step 6 → ✅，Gate 5 → ✅

## 产出

| 产物 | 条件 |
|------|------|
| GitHub Release | 渠道含 github 或 ci |
| npm 包 | 渠道含 npm |
| workflow_state | 始终 |

## 完成后

- 确认 Gate 5；提示合入 `main`（若需要）与版本索引更新
