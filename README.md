# Centag

LLM 统一网关：协议适配、后端路由、流水线/钩子/插件、token 计量与计费。由 Centag 精简迁移而来（无桌面壳、无内置 business 插件树）。

## 目录结构

```
centag/
├── cmd/centag/          # 本地开发入口
├── core/                # Go 核心库
├── plugins/             # protocol / backend / database / storage
├── dist/                # minimal | gateway | team 发行版入口（仅源码）
├── web/                 # Vue 管理端
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
./start.sh debug --minimal
```

## 发行版

| 发行版 | 说明 |
|--------|------|
| minimal | 轻量，无 DB |
| gateway | 个人全功能，默认 SQLite |
| team | 与 gateway 对齐，默认外挂 PG |

```bash
./start.sh dist build gateway
./start.sh dist docker-build gateway
```

业务插件外置：见 [docs/guide/external-business-plugins.md](docs/guide/external-business-plugins.md)。

环境变量使用 `CENTAG_*`（以及运行时 `LLM_PROXY_*`）。
