# Centag

LLM 统一网关：协议适配、后端路由、流水线/钩子/插件、token 计量与计费。核心无内置 business 插件树；可选桌面启动器见 `apps/launcher/`（菜单/托盘 + 浏览器，非 Wails）。

**许可证**：核心仓库以 [MIT](LICENSE) 开源。开源发行版仅 **`minimal` / `personal`**（完整独立构建，不依赖其它仓库）。**Team 商业版**仅在私有仓 [`centag-pro`](https://github.com/atoml-ai/centag-pro) 构建（`./scripts/build-team.sh`）；本仓已删除 `dist/team`，**不再提供** `./start.sh build team` 转调入口。

**分支约定**：`centag-pro` 必须与本仓**同名分支**开发（例如本仓 `feature/v0.2.7` ↔ pro `feature/v0.2.7`），见 [dist/README.md](dist/README.md)。

## 目录结构

```
centag/
├── cmd/centag/          # 本地开发入口
├── core/                # Go 核心库
├── plugins/             # protocol / backend / database / storage
├── dist/                # minimal | personal 发行版入口（Team 在 centag-pro）
├── web/                 # Vue 管理端
├── apps/launcher/       # 可选：桌面启动器（与核心解耦）
├── config/              # profiles / initdata / secrets
├── deploy/              # Docker / stack / fnos
├── scripts/             # 运维与校验脚本
├── sdk/                 # 外部插件 SDK
├── docs/
├── bin/                 # 本地构建产物（勿提交）
├── Makefile / start.sh
└── go.work
```

## 一键安装（`install.sh`）

默认安装 **personal CLI + wrap** 到 `~/.centag/bin`（并尝试写入 PATH）。需已发布的 GitHub Release 资产。

```bash
# 推荐：按 Release tag 拉取安装脚本（与发版 tag 一致，例如 v0.2.7）
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/v0.2.7/scripts/install.sh | bash

# 指定版本（脚本仍从 tag/分支取，二进制从对应 Release 下载）
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/v0.2.7/scripts/install.sh | bash -s -- --version 0.2.7

# 只装 personal 或只装 wrap
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/v0.2.7/scripts/install.sh | bash -s -- --only personal
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/v0.2.7/scripts/install.sh | bash -s -- --only wrap

# 等价写法（位置参数）
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/v0.2.7/scripts/install.sh | bash -s -- wrap

# 无可用 Release 时：克隆源码构建（需 Go / Node）
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/v0.2.7/scripts/install.sh | bash -s -- --from-source

# 自定义安装目录、不改 shell rc
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/v0.2.7/scripts/install.sh | bash -s -- \
  --prefix "$HOME/.centag" --no-modify-path
```

安装后常用命令：

```bash
centag                 # 启动 personal（默认端口 20060）
centag-personal        # 直接跑二进制
centag wrap run -- opencode   # 进程代理（不起网关）
centag wrap doctor
```

脚本说明见 `scripts/install.sh --help`。发版流程见 [docs/harness/skills/step6-release/SKILL.md](docs/harness/skills/step6-release/SKILL.md)（触发词：`step6-release` / 发版；须先过 Step 5 Gate 4）。

## 快速开始（开发机）

```bash
# 密钥模板 → 本地 secrets（勿提交）
cp config/secrets/.env.example config/secrets/.env

make build          # 后端 → ~/.centag/lib/personal/centag-personal
make frontend       # 前端 → ~/.centag/lib/personal/static
make run            # 或 ./start.sh run be
```

管理界面：http://localhost:20060

精简模式（无 DB、单密码管理台）：

```bash
./start.sh debug minimal
```

## 发行版

| 发行版 | 说明 |
|--------|------|
| minimal | 轻量，无 DB（开源） |
| personal | 个人全功能，默认 SQLite（开源） |
| team | **商业 SKU**，在 [`centag-pro`](https://github.com/atoml-ai/centag-pro) 构建 |

```bash
./start.sh build personal
# Team：cd ../centag-pro && ./scripts/build-team.sh
./start.sh docker build personal
```

见 [dist/README.md](dist/README.md)、[docs/guide/dist-profiles.md](docs/guide/dist-profiles.md)、[docs/guide/external-business-plugins.md](docs/guide/external-business-plugins.md)。

可选桌面启动器（默认 lite 无 CGO；`--launcher-tray` 为托盘版）：

```bash
./start.sh build personal                   # 普通个人版服务
./start.sh build personal --launcher        # 个人版 + lite 启动器
./start.sh build personal --launcher-tray   # 个人版 + 托盘启动器（CGO）
./start.sh run personal --launcher
./start.sh build minimal --launcher         # team 不支持 --launcher
```

详见 [apps/launcher/README.md](apps/launcher/README.md)。

环境变量使用 `CENTAG_*`（以及运行时 `LLM_PROXY_*`）。
