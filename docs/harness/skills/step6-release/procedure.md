# Step 6 发版 — 操作正本

> **入口**：`SKILL.md`（编排） + 本文件（命令与渠道细节）  
> **交互**：`.cursor/rules/step6-release.mdc`（AskQuestion 收参）  
> **分层**：根目录 `AGENT.md`

---

## 1. 渠道注册表（可扩展）

新增发版方式时：**只在本表加一行**，并在 §5 执行顺序 与 §6 验收 各加对应小节。SKILL 主流程不变。

| ID | 状态 | 用户说明 | 命令 / 入口 | 依赖 | 鉴权 | 验收 |
|----|:----:|---------|-------------|------|------|------|
| `github` | ✅ | GitHub Release；用户 `curl …/install.sh \| bash` | `publish-binaries.sh --release` 或 `gh release upload` | — | `gh` | `gh release view` |
| `npm` | ✅ | `npm i -g centag` / `centag-offline` | `publish-centag-npm.sh` | 在线版需 **已 publish** 的 GitHub Release | `npm` / `CENTAG_NPM_TOKEN` | `npm view` |
| `ci` | ✅ | 推 tag，Actions 构建上传 | `git tag` + `git push` / workflow_dispatch | — | git remote | Actions 绿 + Release 资产 |
| `build-only` | ✅ | 只构建不上传（验证包） | `build-artifacts.sh` | — | — | 本地目录 + checksums |
| `fnos` | 🔜 | NAS fnOS 包 | `scripts/packaging/` | github 或 build | TBD | TBD |
| `docker` | 🔜 | 容器镜像 | TBD | build | registry login | TBD |

**暂不发布**：独立 `centag-wrap` tarball、`minimal`、`launcher`（进程代理用 `centag wrap`）。

---

## 2. 范围与真源

| 项目 | 真源 |
|------|------|
| 默认组件 | `personal`（含 `centag wrap`） |
| 默认平台 | darwin/linux/windows × amd64/arm64（6 包） |
| 版本号（未指定时） | `apps/wrap-npm/package.json` → `version` |
| npm 包版本 | `apps/centag-npm/package.json`（须与 Release 对齐） |
| 本地产物目录 | `${CENTAG_INSTALL_ROOT:-~/.centag}/var/release/<version>/` |
| 默认仓库 | `atoml-ai/centag`（`CENTAG_RELEASE_REPO` 覆盖） |
| 一键安装 | `scripts/install.sh` |

### 版本分支门禁

对版本 `X.Y.Z`，允许：`vX.Y.Z` / `feature/vX.Y.Z` / `release/vX.Y.Z`。

```bash
bash scripts/release/require-release-branch.sh --version "${CENTAG_RELEASE_VERSION}"
```

| 场景 | 规则 |
|------|------|
| 本地上传 / `--release` | 必须在版本分支 |
| CI `workflow_dispatch` | 在对应版本分支触发 |
| CI 推 `v*` tag | tag 落在版本分支历史上 |
| 仅 `build-only` | 任意分支可构建 |

紧急绕过（默认禁止建议）：`CENTAG_RELEASE_ALLOW_ANY_BRANCH=1`。

---

## 3. 环境变量

| 变量 | 说明 |
|------|------|
| `CENTAG_RELEASE_VERSION` | 无 `v` |
| `CENTAG_RELEASE_CHANNELS` | `github,npm,ci,all,build-only`（逗号分隔；`all` = `github,npm`） |
| `CENTAG_RELEASE_BUILD` | `reuse` / `rebuild` / `skip` |
| `CENTAG_RELEASE_GITHUB_MODE` | `upload`（本地 A/B）/ `ci`（Path C） |
| `CENTAG_RELEASE_DRAFT` | `true` / `false` |
| `CENTAG_RELEASE_VERIFY` | `assets` / `smoke` |
| `CENTAG_RELEASE_REPO` | 默认 `atoml-ai/centag` |
| `CENTAG_INSTALL_REF` | install.sh 的 raw ref，默认 tag `v<version>` |
| `CENTAG_NPM_TOKEN` | npm 发布 token |
| `GH_TOKEN` | 可选，与 `gh auth` 二选一 |

### 兼容旧参数（Cursor 历史会话）

| 旧 | 新 |
|----|-----|
| `CENTAG_RELEASE_PATH=A` | `BUILD=reuse`, `GITHUB_MODE=upload`, channels 含 `github` |
| `CENTAG_RELEASE_PATH=B` | `BUILD=rebuild`, `GITHUB_MODE=upload`, channels 含 `github` |
| `CENTAG_RELEASE_PATH=C` | `BUILD=skip`, `GITHUB_MODE=ci`, channels 含 `ci`（或 `github`+`ci`） |
| `CENTAG_RELEASE_CHANNEL=all` | `CENTAG_RELEASE_CHANNELS=github,npm` |

---

## 4. 共用流程

### 4.1 静默探测脚本

```bash
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
BR="$(git rev-parse --abbrev-ref HEAD)"
VER="$(node -p "require('./apps/wrap-npm/package.json').version" 2>/dev/null || echo '')"
OUT="${CENTAG_INSTALL_ROOT:-$HOME/.centag}/var/release/${VER}"
echo "branch=${BR} version=${VER}"
bash scripts/release/require-release-branch.sh --version "${VER:-0.0.0}" 2>&1 || true
echo "--- gh ---"; gh auth status 2>&1 | head -3 || true
echo "--- npm ---"; npm whoami 2>&1 || true
echo "--- local artifacts ---"; ls "${OUT}"/centag-personal-*.tar.gz 2>/dev/null | wc -l | xargs echo "tarball_count="
echo "--- remote release ---"; gh release view "v${VER}" --repo atoml-ai/centag --json isDraft,assets 2>&1 | head -3 || true
```

### 4.2 鉴权矩阵

| 渠道 | 检查 | 失败时提示用户 |
|------|------|----------------|
| `github` | `gh auth status` | `gh auth login` 或 `gh auth refresh -h github.com` |
| `npm` | `npm whoami` 或 env `CENTAG_NPM_TOKEN` | `npm login` 或 `export CENTAG_NPM_TOKEN=…`（勿在聊天粘贴 token） |
| `ci` | `git remote -v` + 推送权限 | 确认 SSH/HTTPS 凭据 |

**Agent 行为**：鉴权失败 → **停止该渠道**，给出命令；用户确认已登录后 **从该渠道重试**，不必重跑已完成渠道。

### 4.3 共用构建（只跑一次）

产物：`centag-personal-<goos>-<goarch>.tar.gz` × 6 + `checksums.txt`。

```bash
RELEASE_OUT="${CENTAG_INSTALL_ROOT:-$HOME/.centag}/var/release/${CENTAG_RELEASE_VERSION}"

# reuse：已有完整产物则跳过
if [[ "${CENTAG_RELEASE_BUILD}" == "reuse" ]]; then
  count="$(find "${RELEASE_OUT}" -maxdepth 1 -name 'centag-personal-*.tar.gz' 2>/dev/null | wc -l | tr -d ' ')"
  [[ "$count" -ge 6 ]] && [[ -f "${RELEASE_OUT}/checksums.txt" ]] && echo "reuse OK" && exit 0
  echo "reuse 不可用：产物不足，改为 rebuild"
fi

# rebuild（默认）
./scripts/release/build-artifacts.sh \
  --version "${CENTAG_RELEASE_VERSION}" \
  --components personal
```

| `CENTAG_RELEASE_BUILD` | 行为 |
|------------------------|------|
| `reuse` | 有 6 tarball + checksums → 跳过；否则 fallback rebuild |
| `rebuild` | 始终 `build-artifacts.sh` |
| `skip` | 不跑共用构建（CI 发版，或仅 npm 且接受脚本内二次编译） |

> **说明**：`publish-centag-npm.sh` 会为 npm vendor 目录**再次交叉编译**。选 `github`+`npm` 时仍先共用构建供 GitHub；npm 步骤暂无法消除二次编译（脚本层待优化）。Agent **不得**为省事先 npm 后 github（在线包依赖 Release）。

---

## 5. 按序执行发版

解析 `CENTAG_RELEASE_CHANNELS` 后按下列顺序执行（跳过未选渠道）：

```
build-only → (github | ci) → npm
```

### 5.1 `build-only`

```bash
# 已在 §4.3 完成；无上传
ls -lh "${RELEASE_OUT}/"
```

不受 Gate 4 约束时可用于本地验包；工作流内仍建议过 Gate 4。

### 5.2 `github` — GitHub Release + install.sh

**前置**：Gate 4 ✅、版本分支、`gh auth`、§4.3 产物就绪（`BUILD=skip` 时必须有 reuse 产物）。

#### 模式 A — 仅上传（`BUILD=reuse` + `GITHUB_MODE=upload`）

```bash
bash scripts/release/require-release-branch.sh --version "${CENTAG_RELEASE_VERSION}"
RELEASE_OUT="${CENTAG_INSTALL_ROOT:-$HOME/.centag}/var/release/${CENTAG_RELEASE_VERSION}"
TAG="v${CENTAG_RELEASE_VERSION}"
REPO="${CENTAG_RELEASE_REPO:-atoml-ai/centag}"

if gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  gh release upload "$TAG" --repo "$REPO" --clobber \
    "${RELEASE_OUT}"/centag-personal-*.tar.gz \
    "${RELEASE_OUT}/checksums.txt"
else
  # 新建：脚本写 notes；默认 --draft（见下「草稿」）
  ./scripts/release/publish-binaries.sh --version "${CENTAG_RELEASE_VERSION}" --release
fi
```

#### 模式 B — 构建并上传（`BUILD=rebuild` + `GITHUB_MODE=upload`）

```bash
./scripts/release/publish-binaries.sh --version "${CENTAG_RELEASE_VERSION}" --release
```

#### 草稿 / 公开

| 情况 | 行为 |
|------|------|
| Release **不存在**，`DRAFT=true` | `publish-binaries.sh` 创建 **draft** Release |
| Release **不存在**，`DRAFT=false` | 创建后 `gh release edit … --draft=false` |
| Release **已存在** | 上传 **不会**改 draft 状态；用户选草稿但已公开 → **告知**并继续上传资产 |

Release notes 真源：`scripts/release/publish-binaries.sh` 内 `NOTES`（英文）。已存在 Release 且 notes 过期 → `gh release edit … --notes-file …` 单独处理。

#### 用户安装命令（汇报用）

```bash
curl -fsSL "https://raw.githubusercontent.com/${CENTAG_RELEASE_REPO:-atoml-ai/centag}/v${CENTAG_RELEASE_VERSION}/scripts/install.sh" \
  | bash -s "${CENTAG_RELEASE_VERSION}"
```

`CENTAG_INSTALL_REF` 非 tag 时替换 URL 中的 ref。

### 5.3 `npm`

**前置**：

- 在线版 `centag`：`download.js` 从 GitHub Release 拉二进制 → **必须先有已 publish 的 Release**（或用户仅发 offline 且接受限制）。
- 鉴权：`CENTAG_NPM_TOKEN` 或 `npm whoami`。

```bash
# 仅打包（不 publish）— 调试
DRY_RUN=1 ./scripts/publish-centag-npm.sh

# 发布（不重复建 GitHub Release）
CENTAG_NPM_TOKEN="${CENTAG_NPM_TOKEN:-}" ./scripts/publish-centag-npm.sh

# 若尚未发 GitHub 且用户选 all — 应先完成 §5.2，再跑本命令
# 禁止：CENTAG_NPM_TOKEN=… ./scripts/publish-centag-npm.sh --release
#       （--release 会再次调 publish-binaries，与共用构建重复；仅在「仅 npm 且要顺带建 Release」时用）
```

版本须与 `apps/centag-npm/package.json` 一致；发版前 Agent 应核对与 `CENTAG_RELEASE_VERSION` 相同，不一致则 **AskQuestion** 确认以哪边为准或先改 package.json。

### 5.4 `ci` — GitHub Actions

**前置**：版本分支；`BUILD=skip` 本地构建。

```bash
git checkout "feature/v${CENTAG_RELEASE_VERSION}"   # 或 v / release 前缀
git pull
bash scripts/release/require-release-branch.sh --version "${CENTAG_RELEASE_VERSION}"

# 新 tag
git tag "v${CENTAG_RELEASE_VERSION}"
git push origin "v${CENTAG_RELEASE_VERSION}"
```

或：GitHub → Actions → **release** → 选版本分支 → Run workflow。

CI 含 `npm-publish` job 时，选 `ci`+`npm` 可只在 CI 发 npm；本地跳过 §5.3。验收见 §6.3。

**注意**：workflow `run: |` 内禁止顶格 heredoc。

---

## 6. 分渠道验收

### 6.1 GitHub（`CENTAG_RELEASE_VERIFY` ≥ assets）

```bash
gh release view "v${CENTAG_RELEASE_VERSION}" \
  --repo "${CENTAG_RELEASE_REPO:-atoml-ai/centag}" \
  --json name,isDraft,assets \
  --jq '{name,isDraft,assets:[.assets[].name]}'
```

期望：`centag-personal-*` × 6 + `checksums.txt`。  
**Draft** → 提醒：匿名 `curl` 下载不可用，需 Publish。

### 6.2 npm

```bash
npm view centag version
npm view centag-offline version
```

应与 `CENTAG_RELEASE_VERSION` 一致。

### 6.3 CI

```bash
gh run list --workflow=release.yml --limit 3
gh run view <run-id> --log-failed
```

### 6.4 可选冒烟（`CENTAG_RELEASE_VERIFY=smoke`）

```bash
PREFIX="${TMPDIR:-/tmp}/centag-install-smoke"
rm -rf "$PREFIX"
bash scripts/install.sh \
  --version "${CENTAG_RELEASE_VERSION}" \
  --prefix "$PREFIX" \
  --bin-dir "$PREFIX/bin" \
  --no-modify-path
test -x "$PREFIX/bin/centag" || test -x "$PREFIX/bin/centag-personal"
"$PREFIX/bin/centag" wrap help >/dev/null
```

---

## 7. Agent 执行清单（逐步打勾）

1. [ ] 复核 Gate 4；失败 → 中止
2. [ ] 静默探测 + **向用户摘要**环境
3. [ ] AskQuestion：**渠道** → 构建策略 → GitHub 投递 → 草稿 → 验收
4. [ ] 条件追问：鉴权 / npm 依赖 Release / 版本不一致
5. [ ] `require-release-branch.sh`
6. [ ] 鉴权矩阵；缺则停、提示登录
7. [ ] §4.3 共用构建（按 `BUILD`）
8. [ ] §5.2 github（若选）
9. [ ] §5.3 npm（若选）
10. [ ] §5.4 ci（若选）
11. [ ] §6 分渠道验收
12. [ ] 更新 `workflow_state.md` Step 6 / Gate 5 + 决策日志
13. [ ] 汇报各渠道 URL/版本；安装 = 用户手动（除非 smoke）

**禁止**：发版中改业务代码；非版本分支正式 Release；擅自加 minimal/launcher。

---

## 8. 常见失败

| 现象 | 处理 |
|------|------|
| `only from version branch` | 切换 `feature/vX` |
| `gh` not logged in | `gh auth login` / `refresh` |
| npm publish 403 | `npm login` 或检查 `CENTAG_NPM_TOKEN` 权限 |
| 在线 centag 安装失败 | GitHub Release 仍为 draft 或版本未 publish |
| Release 已存在但用户要草稿 | 说明脚本无法回退 draft；仅覆盖资产 |
| `publish-centag-npm` 版本不符 | 对齐 `apps/centag-npm/package.json` |
| CI tag not on version branch | 在版本分支重打 tag |
| install.sh 404 on main | 用 tag raw URL 或先合 main |

---

## 9. 相关文件

| 文件 | 职责 |
|------|------|
| `scripts/release/build-artifacts.sh` | 共用构建 |
| `scripts/release/publish-binaries.sh` | GitHub Release |
| `scripts/publish-centag-npm.sh` | npm |
| `scripts/install.sh` | curl 安装 |
| `scripts/release/require-release-branch.sh` | 分支门禁 |
| `.github/workflows/release.yml` | CI |
| `apps/centag-npm/` | npm 包定义 |
| `apps/wrap-npm/package.json` | 版本参考 |
