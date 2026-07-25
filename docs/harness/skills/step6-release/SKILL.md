---
name: step6-release
description: "工作流 Step 6：全流程发版 — GitHub Release（curl|install.sh）、npm、CI；共用构建后按渠道顺序执行。触发：step6-release、发版、publish release"
---

# Step 6: 全流程发版

> 操作正本：`procedure.md`（渠道注册表、共用构建、分渠道命令、验收）  
> 工作流指南：`docs/harness/workflow/phase-5-release.md`  
> Cursor 收参：`.cursor/rules/step6-release.mdc`

## 触发词

`step6-release` / `step6` / 发版 / release / 打 release / 发布 github release / publish release / 一键发版 / `centag release`

## 设计原则

1. **一个 Skill 跑完**：Gate 4 → 选渠道 → 共用准备/构建 → **按依赖顺序**逐渠道发版 → 分渠道验收 → Gate 5。
2. **先问渠道**：用户一开始就明确发哪些（GitHub / npm / 全部 / 仅构建不上传）；未来渠道在 `procedure.md` 注册表扩展。
3. **共用步骤只跑一次**：版本分支门禁、鉴权探测、交叉编译与 tarball（见 procedure §共用构建）。
4. **缺信息才问**：版本号、token、是否草稿、是否冒烟等；能静默探测的不重复问。
5. **鉴权失败只提示、不编造**：给出具体登录命令，等用户完成后再继续。

## 前置门禁（Gate 4）

**未通过则拒绝发版**，退回 `step5-review`：

| 检查项 | 要求 |
|--------|------|
| G4.1 | `CR_报告.md` 存在，Critical = 0 |
| G4.5 | 标准产物齐全 |
| G4.6 | 开发风险评估无开放 Critical |
| **G4.7** | `workflow_state.md` Gate 4 = ✅，且 CR 勾选「**批准 — 可发版**」 |

> 仅 `build-github-artifacts.sh` / `build-artifacts.sh`（不上传）不受 Gate 4 约束，但仍建议在版本分支操作。

## 流水线总览

```
┌─────────────┐   ┌──────────────┐   ┌─────────────┐   ┌──────────────┐   ┌─────────┐
│ 0 静默探测  │ → │ 1 收参(渠道) │ → │ 2 共用准备  │ → │ 3 共用构建  │ → │ 4 按序  │
│ Gate4/分支  │   │ AskQuestion  │   │ 鉴权/分支   │   │ (可选/一次) │   │ 发渠道  │
└─────────────┘   └──────────────┘   └─────────────┘   └─────────────┘   └────┬────┘
                                                                               │
                    ┌──────────────┐   ┌─────────────┐                         │
                    │ 6 Gate5落盘  │ ← │ 5 分渠道验收│ ←───────────────────────┘
                    └──────────────┘   └─────────────┘
```

## 发布渠道（当前）

| 渠道 ID | 用户可见名 | 产物 / 消费方式 | 前置依赖 | 鉴权 |
|---------|-----------|----------------|----------|------|
| `github` | GitHub Release + `install.sh` | **cli** 全平台（默认安装）+ **desktop**(Win/mac，`--desktop`) + checksums | desktop 需原生 OS | `gh auth` |
| `npm` | npm 包 | 全平台 **cli**；`centag`（在线 lazy-download）+ `centag-offline` | **在线版依赖** 对应版本 GitHub Release 已 publish | `npm login` 或 `CENTAG_NPM_TOKEN` |
| `ci` | GitHub Actions | 推 `v*` tag 或 workflow_dispatch（desktop 分 runner + linux cli） | 无（CI 内构建） | `git push` + Actions 权限 |

**推荐组合与顺序**（写死依赖，Agent 自动排序）：

| 用户选择 | 实际执行顺序 |
|----------|-------------|
| 仅 `github` | `github` |
| 仅 `npm` | 若 Release 不存在 → 先提示补 `github` 或改选「全部」 |
| `github` + `npm` 或 `all` | `github` → `npm` |
| `ci` | `ci`（本地可 skip 构建）；若同时选 `npm` 且 CI 含 npm job → 等 CI 完成再验收 npm |
| `build-only` | 仅 §共用构建，不上传 |

未来渠道（fnOS 包、Docker Hub、短链 CDN 等）在 `procedure.md` §渠道注册表 增加一行即可，**不改** SKILL 主流程。

## 环境变量契约（入口层导出）

| 变量 | 必需 | 说明 |
|------|:----:|------|
| `CENTAG_RELEASE_VERSION` | ✅ | 无 `v` 前缀，如 `0.2.8` |
| `CENTAG_RELEASE_CHANNELS` | ✅ | 逗号分隔：`github` / `npm` / `ci` / `all` / `build-only` |
| `CENTAG_RELEASE_BUILD` | ✅ | `reuse`（已有产物）/ `rebuild` / `skip`（CI 或仅上传） |
| `CENTAG_RELEASE_GITHUB_MODE` | 含 github 时 | `upload`（Path A/B）/ `ci`（Path C，与 `ci` 渠道合并） |
| `CENTAG_RELEASE_DRAFT` | 可选 | `true`（新建 Release 默认草稿）/ `false` |
| `CENTAG_RELEASE_VERIFY` | 可选 | `assets`（默认）/ `smoke`（用户点名 install 冒烟） |
| `CENTAG_RELEASE_REPO` | 可选 | 默认 `atoml-ai/centag` |
| `CENTAG_INSTALL_REF` | 可选 | install.sh raw ref，默认 `v<version>` |

兼容旧变量：`CENTAG_RELEASE_PATH=A|B|C` 映射为 `BUILD` + `GITHUB_MODE`（见 procedure §兼容映射）。

## Agent 执行步骤

### 0. 静默探测（必须）

```bash
git rev-parse --abbrev-ref HEAD
VER="$(node -p "require('./apps/wrap-npm/package.json').version" 2>/dev/null || true)"
bash scripts/release/require-release-branch.sh --version "${VER:-0.0.0}" 2>&1 || true
gh auth status 2>&1 | head -5 || true
npm whoami 2>&1 || true
RELEASE_OUT="${CENTAG_INSTALL_ROOT:-$HOME/.centag}/var/release/${VER}"
ls "${RELEASE_OUT}"/centag-cli-*.tar.gz "${RELEASE_OUT}"/centag-desktop-* 2>/dev/null | wc -l
gh release view "v${VER}" --repo atoml-ai/centag 2>&1 | head -5 || true
# 读 docs/versions/<ver>/<需求>/workflow_state.md + CR_报告.md
```

探测结果**摘要给用户**（分支、版本、本地产物数、远端 Release 是否存在、gh/npm 登录态），再进入 AskQuestion。

**渠道产物**：GitHub/`install.sh` = **全平台 cli**（默认）+ Win/mac **desktop**（`--desktop`）；npm = **全平台 cli**。本地打包：`./start.sh package <cli|desktop> <os> [arch]`（见 procedure §1.1）。

### 1. Gate 4 拦截

不满足 → 列出缺失项，**中止**，引导 `step5-review`。

### 2. 收参（入口 AskQuestion）

见 `.cursor/rules/step6-release.mdc`。核心：**先选渠道（可多选）**，再选构建策略与 GitHub 投递方式。

根据探测**条件追问**（第二轮 AskQuestion 或短文字，仅当需要）：

| 条件 | 追问 |
|------|------|
| 选了 `npm` 且 gh/npm 未登录 | 提示 `npm login` 或导出 `CENTAG_NPM_TOKEN`，确认后继续 |
| 选了 `github` 且 `gh auth` 失败 | 提示 `gh auth login` / `gh auth refresh`，确认后继续 |
| 选了 `npm` 且 GitHub Release 不存在 | 建议改为 `all` 或先完成 `github` |
| 选了 `ci` 且 tag 已存在 | 确认覆盖策略或改版本 |
| 版本与 package.json 不一致 | **以 `CENTAG_RELEASE_VERSION` 为准**，执行 `sync-npm-version.sh` 并提交（见 procedure「版本号同步」） |

### 3. 共用准备

1. `require-release-branch.sh --version <ver>`
2. **`sync-npm-version.sh --version <ver>`**（github / npm / ci 均需；有 diff 则提交后再继续）
3. 按渠道检查鉴权（procedure §鉴权矩阵）
4. 确定构建策略：reuse / rebuild / skip

### 4. 共用构建（一次，GitHub 形态）

仅当 `CENTAG_RELEASE_BUILD` ≠ `skip` 且渠道含 `github`（或需本地验 GitHub 包）时执行：

```bash
./scripts/release/build-github-artifacts.sh --version "${CENTAG_RELEASE_VERSION}"
```

`reuse`：已有 cli ×6 + checksums → 跳过。  
`rebuild`：始终跑 `build-github-artifacts.sh`（全平台 CLI + 本机 desktop；完整 Win/mac desktop 靠 CI）。  
`skip`：Path C / 仅 npm（npm 自带全平台 CLI 编译）时跳过。

### 5. 按序发渠道

严格按 procedure §执行顺序 与依赖表执行；**每完成一个渠道再进入下一个**；失败则停，汇报已完成部分。

### 6. 分渠道验收

- `github` → Release 资产齐全；草稿则提醒 Publish 后 curl 才可用
- `npm` → `npm view @atomlai/centag version` 与 `@atomlai/centag-offline version`（须等于 `CENTAG_RELEASE_VERSION`）
- `ci` → Actions run 成功 + Release job 绿
- `verify=smoke` → 仅用户点名时跑 `install.sh` 冒烟

### 7. Gate 5 与落盘

- 更新 `docs/versions/<ver>/<需求>/workflow_state.md`：Step 6 ✅、Gate 5 ✅（已选渠道均验收通过）
- 决策日志追加：渠道列表、构建策略、Release URL、npm 版本
- 汇报：各渠道结果摘要；安装/部署 = 用户手动（除非 smoke）

## 产出

| 产物 | 条件 |
|------|------|
| GitHub Release `v<version>` | 渠道含 `github` 或 `ci` |
| npm `centag` + `centag-offline` | 渠道含 `npm` |
| `workflow_state.md` | 始终 |
| 本地 `~/.centag/var/release/<version>/` | BUILD ≠ skip |

## 禁止

- 非版本分支上传正式 Release（除非用户明确要求 `CENTAG_RELEASE_ALLOW_ANY_BRANCH=1`）
- 擅自扩大组件（minimal / 独立 wrap tarball；形态仅 cli|desktop）
- 发版过程中改业务代码
- 编造 token 或跳过 Gate 4

## 相关脚本

| 脚本 | 用途 |
|------|------|
| `./start.sh package <form> <os> [arch]` | 本地统一打包（`cli`/`desktop` × os × arch） |
| `scripts/release/build-github-artifacts.sh` | GitHub 渠道：全平台 cli + 本机 desktop |
| `scripts/release/package-desktop.sh` | 本机 desktop（dmg/zip） |
| `scripts/release/build-artifacts.sh` | cli 交叉编译（可与 desktop 共存同一 OUT_DIR） |
| `scripts/release/publish-binaries.sh` | GitHub 上传（`--release`） |
| `scripts/publish-centag-npm.sh` | npm 打包发布（全平台 cli） |
| `scripts/install.sh` | 用户 curl 安装（按 OS 选 desktop/cli） |
| `scripts/release/require-release-branch.sh` | 版本分支门禁 |

细节命令见 **`procedure.md`**。
