# Centag Release — 正本

> 本文件为 **GitHub Release 发版与一键安装验收** 的唯一业务正本（路径：`docs/harness/skills/`）。  
> **交互入口**在各 Agent 目录（如 `.cursor/rules/centag-release.mdc`），只负责收参；收齐后交接本文件执行。  
> 分层原则见仓库根目录 **`AGENT.md`**。

---

## 范围（当前）

| 发布产物 | 资产名 | 说明 |
|----------|--------|------|
| personal CLI + WebUI static | `centag-personal-<goos>-<goarch>.tar.gz` | 默认安装的服务端 |
| proxyctl | `centag-proxyctl-<goos>-<goarch>.tar.gz` | 本机/进程代理辅助 |
| 校验和 | `checksums.txt` | SHA-256 |

**暂不发布**（以后再说）：`minimal`、`launcher` / `launcher-tray`。

| 消费者 | 路径 |
|--------|------|
| 一键安装脚本 | `scripts/install.sh` |
| 本地构建产物 | `scripts/release/build-artifacts.sh` → `bin/release/<version>/` |
| 本地上传 Release | `scripts/release/publish-binaries.sh` |
| CI 发版 | `.github/workflows/release.yml`（`v*` tag 或 workflow_dispatch） |
| 分支门禁 | `scripts/release/require-main-branch.sh` |
| 版本号真源（无 `--version` 时） | `apps/proxyctl-npm/package.json` → `version` |
| 默认仓库 | `atoml-ai/centag`（可用 `CENTAG_RELEASE_REPO` 覆盖） |

默认平台：`darwin-amd64`、`darwin-arm64`、`linux-amd64`、`linux-arm64`、`windows-amd64`、`windows-arm64`。

### 强制：仅 main 可发布

| 场景 | 规则 |
|------|------|
| 本地 `--release` / Path A 上传 | 当前分支必须是 `main`；若存在 `origin/main`，不得落后或分叉 |
| CI `workflow_dispatch` | 必须在 **main** 上 Run workflow |
| CI 推送 `v*` tag | tag 指向的 commit 必须是 `origin/main` 的祖先（即 tag 打在 main 历史上） |
| 仅构建不上传 | 任意分支可跑 `build-artifacts.sh` / `publish-binaries.sh`（不加 `--release`） |

紧急绕过（不推荐）：`CENTAG_RELEASE_ALLOW_NON_MAIN=1`。Agent **默认禁止**建议用户使用；仅用户明确要求时才设置。

---

## 前置条件（入口层注入）

| 参数 | 环境变量 | 必需 | 说明 |
|------|----------|------|------|
| 版本号 | `CENTAG_RELEASE_VERSION` | ✅ | 无 `v` 前缀，如 `0.2.7`；tag 为 `v0.2.7` |
| 发版路径 | `CENTAG_RELEASE_PATH` | ✅ | 见下方「发版路径」 |
| 是否草稿 | `CENTAG_RELEASE_DRAFT` | 可选 | `true`（默认建议）/ `false` |
| 仓库 | `CENTAG_RELEASE_REPO` | 可选 | 默认 `atoml-ai/centag` |
| 安装脚本分支 | `CENTAG_INSTALL_REF` | 可选 | 默认 `main` |

入口层必须确认：

1. 当前在 **main**（或即将在 main 上操作）：`bash scripts/release/require-main-branch.sh`。
2. `gh auth status` 成功（本地路径 A/B）。
3. 工作区含要发布的 `scripts/install.sh`（已合入 / 已推送到 **main**，因用户 curl 默认读 main）。
4. 版本与 `apps/proxyctl-npm/package.json` 的 `version` 对齐（或用户明确指定覆盖）。

---

## 发版路径

### Path A — 已有本地产物，仅上传（推荐：避免重编）

适用：已跑过构建，`bin/release/<version>/` 下已有 `centag-personal-*.tar.gz`、`centag-proxyctl-*.tar.gz`、`checksums.txt`。

```bash
# 0) 必须在 main + 登录
bash scripts/release/require-main-branch.sh
gh auth login   # 若尚未登录
gh auth status

# 1) 确认产物
ls -lh "bin/release/${CENTAG_RELEASE_VERSION}/"

# 2) 创建草稿 Release 并上传
gh release create "v${CENTAG_RELEASE_VERSION}" \
  --repo "${CENTAG_RELEASE_REPO:-atoml-ai/centag}" \
  --draft \
  --title "Centag ${CENTAG_RELEASE_VERSION}" \
  --notes "personal CLI + proxyctl. Install: curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash" \
  "bin/release/${CENTAG_RELEASE_VERSION}"/centag-*.tar.gz \
  "bin/release/${CENTAG_RELEASE_VERSION}/checksums.txt"
```

若 tag/release 已存在，改为上传：

```bash
gh release upload "v${CENTAG_RELEASE_VERSION}" \
  --repo "${CENTAG_RELEASE_REPO:-atoml-ai/centag}" \
  --clobber \
  "bin/release/${CENTAG_RELEASE_VERSION}"/centag-*.tar.gz \
  "bin/release/${CENTAG_RELEASE_VERSION}/checksums.txt"
```

草稿确认无误后：在 GitHub Release 页 **Publish release**，或：

```bash
gh release edit "v${CENTAG_RELEASE_VERSION}" --repo "${CENTAG_RELEASE_REPO:-atoml-ai/centag}" --draft=false
```

### Path B — 脚本构建并上传

会重新交叉编译（耗时长）。需 Go、Node、网络。

```bash
gh auth login   # 若尚未登录

# 仅构建
./scripts/release/publish-binaries.sh --version "${CENTAG_RELEASE_VERSION}"

# 构建 + 草稿 Release
./scripts/release/publish-binaries.sh --version "${CENTAG_RELEASE_VERSION}" --release

# 干跑（构建但不上传）
DRY_RUN=1 ./scripts/release/publish-binaries.sh --version "${CENTAG_RELEASE_VERSION}" --release
```

等价拆步：

```bash
./scripts/release/build-artifacts.sh --version "${CENTAG_RELEASE_VERSION}" --components personal,proxyctl
# 再按 Path A 用 gh release create/upload
```

### Path C — 推 tag，走 GitHub Actions

适用：改动已合入 **main**；希望 CI 交叉编译。

```bash
git checkout main
git pull origin main
bash scripts/release/require-main-branch.sh

# tag 必须打在 main 的 commit 上，再推送
git tag "v${CENTAG_RELEASE_VERSION}"
git push origin "v${CENTAG_RELEASE_VERSION}"
```

或：Actions → **release** → 选择分支 **main** → **Run workflow**（非 main 会被 `guard-main` 拒绝）。

CI 默认组件：`personal,proxyctl`。产物写入草稿（或按 input）Release。

**注意**：workflow 里 `run: |` 块禁止出现**顶格**的 heredoc 正文（否则 YAML 解析失败）。改 notes 时保持缩进或用 `echo`。

---

## 验收（发版后必做）

### 1. Release 资产齐全

```bash
gh release view "v${CENTAG_RELEASE_VERSION}" --repo "${CENTAG_RELEASE_REPO:-atoml-ai/centag}"
```

期望至少包含（每平台一对 personal + proxyctl，外加 checksums）：

- `centag-personal-<goos>-<goarch>.tar.gz` × 支持平台
- `centag-proxyctl-<goos>-<goarch>.tar.gz` × 支持平台
- `checksums.txt`

**草稿 Release 的资产 URL 对匿名 `curl` 不可用**；验收安装前必须 **Publish**（或本地用已登录的 `gh release download`）。

### 2. 本机安装冒烟（不依赖 main）

```bash
PREFIX="${TMPDIR:-/tmp}/centag-install-smoke"
rm -rf "$PREFIX"
bash scripts/install.sh \
  --version "${CENTAG_RELEASE_VERSION}" \
  --prefix "$PREFIX" \
  --bin-dir "$PREFIX/bin" \
  --no-modify-path

test -x "$PREFIX/bin/centag" || test -x "$PREFIX/bin/centag-personal"
test -x "$PREFIX/bin/centag-proxyctl"
"$PREFIX/bin/centag-proxyctl" --help >/dev/null || "$PREFIX/bin/centag-proxyctl" -h >/dev/null || true
```

只测 proxyctl：

```bash
bash scripts/install.sh --only proxyctl --version "${CENTAG_RELEASE_VERSION}" \
  --prefix "${TMPDIR:-/tmp}/centag-proxyctl-smoke" --no-modify-path
```

### 3. 一行命令（需 install.sh 已在目标 ref）

```bash
# main 已合入 install.sh 且 Release 已公开后：
curl -fsSL "https://raw.githubusercontent.com/${CENTAG_RELEASE_REPO:-atoml-ai/centag}/${CENTAG_INSTALL_REF:-main}/scripts/install.sh" \
  | bash -s -- --version "${CENTAG_RELEASE_VERSION}" --no-modify-path --prefix "${TMPDIR:-/tmp}/centag-curl-smoke"
```

未合入 `main` 时：把 `CENTAG_INSTALL_REF` 设为功能分支名，或只用第 2 步本地 `bash scripts/install.sh`。

### 4. 可选：起服务

```bash
# 默认端口 20060；STATIC_PATH 由 wrapper 指向 lib/personal/static
"$PREFIX/bin/centag" &
# 探测健康（以实际 /health 或管理页为准）
curl -sS -o /dev/null -w "%{http_code}\n" "http://127.0.0.1:20060/" || true
```

---

## Agent 执行清单（交接后按序）

1. 读取入口注入的 `CENTAG_RELEASE_*`；缺版本则停，要求入口补齐。
2. **先跑** `bash scripts/release/require-main-branch.sh`（上传/CI 路径）；非 main 则中止并提示先合入 main。
3. 按 `CENTAG_RELEASE_PATH` 选 A / B / C 执行；禁止擅自扩大组件集（不加 minimal/launcher）。
4. Path A/B：确认 `gh auth`；失败则只汇报认证错误，不编造 token。
5. 上传完成后执行「验收」§1–§2；§3 仅在用户需要 curl 一行命令时做。
6. 汇报：Release URL、资产列表摘要、安装冒烟通过/失败（含 HTTP/命令退出码）。
7. **禁止**在 skill 执行中改业务代码；workflow/脚本缺陷单独提修，不塞进发版步骤。

---

## 常见失败

| 现象 | 处理 |
|------|------|
| `local release upload only from main` | `git checkout main && git pull` 后再发 |
| CI：`tag … is not on origin/main` | 在 main 上重新打 tag / 删错 tag |
| CI：`workflow_dispatch … only from main` | Run workflow 时分支选 main |
| `gh`：not logged in | `gh auth login` |
| `EXTRA_BUILD_ARGS[@]: unbound variable` | 已修；确保使用当前 `publish-binaries.sh` |
| Actions：`Invalid workflow file` near notes | `run: \|` 内勿顶格 heredoc；见 Path C 注意 |
| install 下载 404 | Release 仍是 draft，或版本/资产名不对 |
| `curl …/main/install.sh` 404 | `install.sh` 未合入 main；改用分支 raw 或本地脚本 |
| checksum mismatch | 重新上传对应 tar.gz + 更新 checksums.txt |

---

## 相关文件

| 文件 | 职责 |
|------|------|
| `scripts/install.sh` | 用户侧一键安装 |
| `scripts/release/build-artifacts.sh` | 交叉编译 + checksums |
| `scripts/release/publish-binaries.sh` | 构建并可选 `gh release`（`--release` 时强制 main） |
| `scripts/release/require-main-branch.sh` | 仅 main 可发布门禁 |
| `.github/workflows/release.yml` | tag / 手动触发 CI 发版（`guard-main`） |
| `apps/proxyctl-npm/package.json` | 版本号对齐参考 |
| `apps/proxyctl-npm/lib/download.js` | npm 侧下载同名 `centag-proxyctl-*.tar.gz` |

---

## 以后扩展（非当前步骤）

- `--components` 增加 `minimal`、`launcher`、`launcher-tray` 时：先改 `build-artifacts.sh` / `install.sh` / 本正本「范围」表，再改 CI。
- 短链 `https://centag.ai/install`：CDN/站点配置，不在本 skill 内实现。
