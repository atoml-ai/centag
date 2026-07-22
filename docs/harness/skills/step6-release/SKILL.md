---
name: step6-release
description: "工作流 Step 6：发版 — GitHub Release 与资产校验（安装由用户手动）。触发场景：step6-release、发版、publish release"
---

# Step 6: 发版（GitHub Release）

> 执行指南：`docs/harness/workflow/phase-5-release.md`  
> 发版操作正本：`docs/harness/skills/step6-release/procedure.md`

## 触发词

`step6-release` / `step6` / 发版 / release / 打 release / 发布 github release / publish release / 一键发版

## 前置门禁

**Gate 4（CR 准出 / 发版许可）**：必须全部满足，否则**拒绝发版**并退回 `step5-review`：

| 检查项 | 要求 |
|--------|------|
| G4.1 | `CR_报告.md` 存在，Critical = 0 |
| G4.5 | 标准产物齐全（技术方案 + 开发风险评估 + 任务计划 + 自测记录 + CR + 代码/测试） |
| G4.6 | 开发风险评估无开放 Critical |
| **G4.7** | **人工已确认批准发版**：`workflow_state.md` 中 Gate 4 = ✅，且 CR 结论勾选「批准 — 可发版」 |

> ⚠️ Gate 4 **不会**在 Step 5 写完 CR 后自动通过；必须经人工确认（AskQuestion）。  
> 仅构建不上传（`build-artifacts.sh` / 不加 `--release`）不受本门禁约束，但仍建议在版本分支操作。

## 相关上下文

- 版本分支门禁：`scripts/release/require-release-branch.sh`
- 安装脚本：`scripts/install.sh`
- npm 发布脚本：`scripts/publish-centag-npm.sh`
- npm 包定义：`apps/centag-npm/`
- 交互入口（Cursor）：`.cursor/rules/step6-release.mdc`

## 执行流程

### 第一步：检查 Gate 4

1. 定位当前需求目录：`docs/versions/<版本>/<需求>/`
2. 读取 `workflow_state.md`：Gate 4 必须为 ✅；Step 5 必须为 ✅
3. 读取 `CR_报告.md`：结论必须为**批准 — 可发版**（或等价「批准」且备注可发版）
4. 任一项不满足 → 输出缺失项，**中止**，提示先完成 `step5-review` 并确认 Gate 4

### 第二步：收参（入口层）

由 Cursor / 其它 Agent 入口用 AskQuestion 收集（见 `step6-release.mdc`）：

| 参数 | 环境变量 | 说明 |
|------|----------|------|
| 版本号 | `CENTAG_RELEASE_VERSION` | 无 `v` 前缀 |
| 发版路径 | `CENTAG_RELEASE_PATH` | A / B / C |
| 发布渠道 | `CENTAG_RELEASE_CHANNEL` | `github` / `npm` / `all`（默认） |
| 是否草稿 | `CENTAG_RELEASE_DRAFT` | 默认建议 `true` |
| 仓库 | `CENTAG_RELEASE_REPO` | 可选 |
| 安装脚本 ref | `CENTAG_INSTALL_REF` | 可选 |

### 第三步：执行发版正本

打开并严格执行 `docs/harness/skills/step6-release/procedure.md`（含版本分支校验、Path A/B/C、验收）。

### 第四步：发版验收与 Gate 5

按 procedure「验收」完成 **Release 资产齐全**检查后（**默认不跑**安装冒烟）：

- 更新 `workflow_state.md`：Step 6 → ✅，Gate 5 → ✅（资产齐全即可）
- 汇报 Release URL、资产摘要；安装/部署验收写「用户手动」

## 产出

| 产物 | 说明 |
|------|------|
| GitHub Release `v<version>` | personal + 资产 + checksums |
| npm `centag` | 在线版（postinstall lazy-download） |
| npm `centag-offline` | 离线版（打包 6 平台二进制） |
| `workflow_state.md` | Step 6 / Gate 5 状态 |
| 可选：发版记录备注 | 可写在需求目录决策日志 |

## 完成后

- 确认 Gate 5 准出（Release 公开或按约定保留草稿 + 资产齐全）
- 提示用户：自行安装/部署验收；合入 `main`（若尚未）、更新版本索引；**禁止**在非版本分支补传正式 Release
