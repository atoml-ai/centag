# Centag Release — 发版操作正本

> 本文件为 **GitHub Release 发版与一键安装验收** 的操作正本。  
> **工作流入口**：`docs/harness/skills/step6-release/SKILL.md`（Step 6；**必须先过 Gate 4**）。  
> **交互入口**：`.cursor/rules/step6-release.mdc`（仅收参）。  
> 分层原则见仓库根目录 **`AGENT.md`**。

---

## 工作流前置（强制）

执行本文件任意上传 / 公开 Release 步骤前，Agent 必须已按 `step6-release/SKILL.md` 确认 **Gate 4（CR 准出 / 发版许可）** 通过。  
未通过 → **禁止** `gh release create/upload`、禁止 `publish-binaries.sh --release`、禁止推送发版 tag。

---

## 范围（当前）

| 发布产物 | 资产名 | 说明 |
|----------|--------|------|
| personal CLI + WebUI static | `centag-personal-<goos>-<goarch>.tar.gz` | 默认安装的服务端 |
| wrap | `centag-wrap-<goos>-<goarch>.tar.gz` | 本机/进程代理辅助 |
| 校验和 | `checksums.txt` | SHA-256 |

**暂不发布**（以后再说）：`minimal`、`launcher` / `launcher-tray`。

| 消费者 | 路径 |
|--------|------|
| 一键安装脚本 | `scripts/install.sh` |
| 本地构建产物 | `scripts/release/build-artifacts.sh` → `~/.centag/var/release/<version>/`（可用 `CENTAG_INSTALL_ROOT` 覆盖） |
| 本地上传 Release | `scripts/release/publish-binaries.sh` |
| CI 发版 | `.github/workflows/release.yml`（`v*` tag 或 workflow_dispatch） |
| 分支门禁 | `scripts/release/require-release-branch.sh` |
| 版本号真源（无 `--version` 时） | `apps/wrap-npm/package.json` → `version` |
| 默认仓库 | `atoml-ai/centag`（可用 `CENTAG_RELEASE_REPO` 覆盖） |

默认平台：`darwin-amd64`、`darwin-arm64`、`linux-amd64`、`linux-arm64`、`windows-amd64`、`windows-arm64`。

### 强制：仅版本分支可发布

对版本 `0.2.7`（tag `v0.2.7`），允许的分支名：

- `v0.2.7`
- `feature/v0.2.7`（本仓当前习惯）
- `release/v0.2.7`

| 场景 | 规则 |
|------|------|
| 本地 `--release` / Path A 上传 | 当前分支必须是上表之一，且与 `--version` 一致 |
| CI `workflow_dispatch` | 必须在对应版本分支上 Run workflow |
| CI 推送 `v*` tag | tag 版本与门禁一致，且 commit 落在上述某一 `origin/<branch>` 历史上 |
| 仅构建不上传 | 任意分支可跑 `build-artifacts.sh` / `publish-binaries.sh`（不加 `--release`） |
| `main` | **不用于**发版上传（可合并，但发版在版本分支完成） |

紧急绕过（不推荐）：`CENTAG_RELEASE_ALLOW_ANY_BRANCH=1`。Agent **默认禁止**建议；仅用户明确要求时才设置。

---

## 前置条件（入口层注入）

| 参数 | 环境变量 | 必需 | 说明 |
|------|----------|------|------|
| 版本号 | `CENTAG_RELEASE_VERSION` | ✅ | 无 `v` 前缀，如 `0.2.7`；tag 为 `v0.2.7` |
| 发版路径 | `CENTAG_RELEASE_PATH` | ✅ | 见下方「发版路径」 |
| 是否草稿 | `CENTAG_RELEASE_DRAFT` | 可选 | `true`（默认建议）/ `false` |
| 仓库 | `CENTAG_RELEASE_REPO` | 可选 | 默认 `atoml-ai/centag` |
| 安装脚本 ref | `CENTAG_INSTALL_REF` | 可选 | 默认 tag `v<version>`（与 Release 对齐） |

入口层必须确认：

1. 当前在**版本分支**上：`bash scripts/release/require-release-branch.sh --version <ver>`。
2. `gh auth status` 成功（本地路径 A/B）。
3. 工作区含要发布的 `scripts/install.sh`（已推到该版本分支；用户 curl 可用 tag `v<ver>`）。
4. 版本与 `apps/wrap-npm/package.json` 的 `version` 对齐（或用户明确指定覆盖）。

---

## 发版路径

### Path A — 已有本地产物，仅上传（推荐：避免重编）

适用：已跑过构建，`${CENTAG_INSTALL_ROOT:-$HOME/.centag}/var/release/<version>/` 下已有 `centag-personal-*.tar.gz`、`centag-wrap-*.tar.gz`、`checksums.txt`。

```bash
# 0) 必须在版本分支（例：feature/v0.2.7）+ 登录
git checkout "feature/v${CENTAG_RELEASE_VERSION}"   # 或 v${CENTAG_RELEASE_VERSION}
bash scripts/release/require-release-branch.sh --version "${CENTAG_RELEASE_VERSION}"
gh auth login   # 若尚未登录
gh auth status

# 1) 确认产物
RELEASE_OUT="${CENTAG_INSTALL_ROOT:-$HOME/.centag}/var/release/${CENTAG_RELEASE_VERSION}"
ls -lh "${RELEASE_OUT}/"

# 2) 创建草稿 Release 并上传
# Release notes MUST be English. Required sections — template真源:
#   scripts/release/publish-binaries.sh（NOTES）
#   Install / Default login / centag-wrap auth (CENTAG_WRAP_TOKEN) / Uninstall / Artifacts
# 推荐直接 Path B（自动写 notes，勿手写过期正文）:
./scripts/release/publish-binaries.sh --version "${CENTAG_RELEASE_VERSION}" --release
```


若 tag/release 已存在，改为上传（**不会**改 notes；正文过期时另跑 `gh release edit … --notes`）：

```bash
RELEASE_OUT="${CENTAG_INSTALL_ROOT:-$HOME/.centag}/var/release/${CENTAG_RELEASE_VERSION}"
gh release upload "v${CENTAG_RELEASE_VERSION}" \
  --repo "${CENTAG_RELEASE_REPO:-atoml-ai/centag}" \
  --clobber \
  "${RELEASE_OUT}"/centag-*.tar.gz \
  "${RELEASE_OUT}/checksums.txt"
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
./scripts/release/build-artifacts.sh --version "${CENTAG_RELEASE_VERSION}" --components personal,wrap
# 再按 Path A 用 gh release create/upload
```

### Path C — 推 tag，走 GitHub Actions

适用：改动已在**版本分支**上；希望 CI 交叉编译。

```bash
git checkout "feature/v${CENTAG_RELEASE_VERSION}"
git pull origin "feature/v${CENTAG_RELEASE_VERSION}"
bash scripts/release/require-release-branch.sh --version "${CENTAG_RELEASE_VERSION}"

# tag 打在版本分支的 commit 上，再推送
git tag "v${CENTAG_RELEASE_VERSION}"
git push origin "v${CENTAG_RELEASE_VERSION}"
```

或：Actions → **release** → 选择分支 **`feature/v0.2.7`**（或 `v0.2.7`）→ **Run workflow**（在 `main` 上跑会被 `guard-branch` 拒绝）。

CI 默认组件：`personal,wrap`。产物写入草稿（或按 input）Release。

**注意**：workflow 里 `run: |` 块禁止出现**顶格**的 heredoc 正文（否则 YAML 解析失败）。改 notes 时保持缩进或用 `echo`。

---

## 验收（发版后必做）

### 1. Release 资产齐全（Agent 默认只做这一项）

```bash
gh release view "v${CENTAG_RELEASE_VERSION}" --repo "${CENTAG_RELEASE_REPO:-atoml-ai/centag}"
```

期望至少包含（每平台一对 personal + wrap，外加 checksums）：

- `centag-personal-<goos>-<goarch>.tar.gz` × 支持平台
- `centag-wrap-<goos>-<goarch>.tar.gz` × 支持平台
- `checksums.txt`

**草稿 Release 的资产 URL 对匿名 `curl` 不可用**；用户侧安装前必须 **Publish**。

资产列表齐全即可将 Gate 5 标 ✅。**安装 / 部署验收由用户手动完成**；Agent **默认不跑**本机 `install.sh` 冒烟、curl 一行安装、起服务探测（耗时长）。

### 2. 可选：本机安装冒烟（仅用户明确要求时）

仅当用户在本轮明确要求「做冒烟 / 安装验收」时才执行；否则跳过并在汇报中写「安装验收：用户手动」。

```bash
PREFIX="${TMPDIR:-/tmp}/centag-install-smoke"
rm -rf "$PREFIX"
bash scripts/install.sh \
  --version "${CENTAG_RELEASE_VERSION}" \
  --prefix "$PREFIX" \
  --bin-dir "$PREFIX/bin" \
  --no-modify-path

test -x "$PREFIX/bin/centag" || test -x "$PREFIX/bin/centag-personal"
test -x "$PREFIX/bin/centag-wrap"
"$PREFIX/bin/centag-wrap" --help >/dev/null || "$PREFIX/bin/centag-wrap" -h >/dev/null || true
```

一行命令 / 起服务等同理：仅用户点名时做，不作为 Step 6 默认路径。

---

## Agent 执行清单（交接后按序）

1. **复核 Gate 4**（见 `step6-release/SKILL.md`）；未通过则中止，不进入上传。
2. 读取入口注入的 `CENTAG_RELEASE_*`；缺版本则停，要求入口补齐。
3. **先跑** `bash scripts/release/require-release-branch.sh --version …`；不在版本分支则中止并提示切换分支。
4. 按 `CENTAG_RELEASE_PATH` 选 A / B / C 执行；禁止擅自扩大组件集（不加 minimal/launcher）。
5. Path A/B：确认 `gh auth`；失败则只汇报认证错误，不编造 token。
6. 上传完成后执行「验收」§1（资产齐全）；**默认跳过** §2 安装冒烟。
7. 汇报：Release URL、资产列表摘要、安装验收=用户手动；更新 `workflow_state` Step 6 / Gate 5。
8. **禁止**在 skill 执行中改业务代码；workflow/脚本缺陷单独提修，不塞进发版步骤。

---

## 常见失败

| 现象 | 处理 |
|------|------|
| `only from version branch` | `git checkout feature/v0.2.7`（或 `v0.2.7`）后再发 |
| CI：`tag … is not on a version branch` | 在 `feature/vX` 上重新打 tag |
| CI：`workflow_dispatch … only from version branch` | Run workflow 时选 `feature/vX`，不要选 main |
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
| `scripts/release/publish-binaries.sh` | 构建并可选 `gh release`（`--release` 时强制版本分支） |
| `scripts/release/require-release-branch.sh` | 版本分支门禁 |
| `.github/workflows/release.yml` | tag / 手动触发 CI 发版（`guard-branch`） |
| `apps/wrap-npm/package.json` | 版本号对齐参考 |
| `apps/wrap-npm/lib/download.js` | npm 侧下载同名 `centag-wrap-*.tar.gz` |

---

## 以后扩展（非当前步骤）

- `--components` 增加 `minimal`、`launcher`、`launcher-tray` 时：先改 `build-artifacts.sh` / `install.sh` / 本正本「范围」表，再改 CI。
- 短链 `https://centag.ai/install`：CDN/站点配置，不在本 skill 内实现。
