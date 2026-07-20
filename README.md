# Centag

LLM 统一网关：协议适配、后端路由、流水线/钩子/插件、token 计量与计费。核心无内置 business 插件树；可选桌面启动器见 `apps/launcher/`（菜单/托盘 + 浏览器，非 Wails）。

**许可证**：核心仓库以 [MIT](LICENSE) 开源。开源发行版仅 **`minimal` / `personal`**（完整独立构建，不依赖其它仓库）。**Team 商业版**由私有仓 [`centag-pro`](https://github.com/atoml-ai/centag-pro) 构建（`cmd/centag-team` + 插件包）；本仓已删除 `dist/team`。本地可 `./start.sh build team`（转调并列 `centag-pro`，或设 `CENTAG_PRO_PATH`）。

## 目录结构

```
centag/
├── cmd/centag/          # 本地开发入口
├── core/                # Go 核心库
├── plugins/             # protocol / backend / database / storage
├── dist/                # minimal | personal | team 发行版入口（仅源码）
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

## 快速开始

```bash
# 密钥模板 → 本地 secrets（勿提交）
cp config/secrets/.env.example config/secrets/.env

make build          # 后端 → bin/server/
make frontend       # 前端 → bin/server/static
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
./start.sh build team          # 需 centag-pro
./start.sh dist docker-build personal
```

见 [dist/README.md](dist/README.md)、[docs/guide/dist-profiles.md](docs/guide/dist-profiles.md)、[docs/guide/external-business-plugins.md](docs/guide/external-business-plugins.md)。

可选桌面启动器（菜单/托盘 + 默认浏览器，非 Wails；`--launcher` 辅助开关）：

```bash
./start.sh build personal              # 普通个人版服务
./start.sh build personal --launcher   # 个人版 + 当前系统启动器
./start.sh run personal --launcher
./start.sh build minimal --launcher    # team 不支持 --launcher
```

详见 [apps/launcher/README.md](apps/launcher/README.md)。

环境变量使用 `CENTAG_*`（以及运行时 `LLM_PROXY_*`）。
