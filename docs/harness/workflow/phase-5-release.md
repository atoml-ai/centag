# Phase 5: 发版（Step 6）

> 快捷别名：`step6-release` / 发版 / release / step6

## 目标

在 **Gate 4（CR 发版许可）** 通过后，于版本分支发布 GitHub Release（personal + proxyctl），并完成安装验收。

## 前置门禁

**Gate 4**（见 `docs/harness/workflow/gate-checklist.md`）

> ⚠️ 未通过 Gate 4（含人工未确认批准）→ **拒绝发版**，退回 `step5-review`。

## 执行清单

### 1. 门禁复核

- [ ] `workflow_state.md`：Gate 4 = ✅，Step 5 = ✅
- [ ] `CR_报告.md`：结论为「批准 — 可发版」
- [ ] 当前分支为版本分支：`bash scripts/release/require-release-branch.sh --version <ver>`

### 2. 发版

按 `docs/harness/skills/step6-release/SKILL.md` 收参，执行 `step6-release/procedure.md`：

- [ ] Path A / B / C 之一
- [ ] 不擅自扩大组件（无 minimal/launcher）
- [ ] 默认建议先草稿，确认后再 Publish

### 3. 验收 → Gate 5

- [ ] Release 资产齐全（personal + proxyctl × 平台 + checksums）
- [ ] 本机 `install.sh` 冒烟通过（或用户明确跳过并记录）
- [ ] 更新 `workflow_state`：Step 6 → ✅，Gate 5 → ✅

## 产出

| 产物 | 说明 |
|------|------|
| GitHub Release | `v<version>` 资产 |
| workflow_state | Step 6 / Gate 5 |

## 完成后

- 确认 Gate 5；提示合入 `main`（若需要）与版本索引更新
