# Deployment Profiles 与 Stack 中间件

本文档说明 Centag **三种 Profile** 与 **deploy/stack** 的分层部署架构（Plan C/D，2026-06 落地）。

> **新手推荐先读**：[Profile 入门手册](./profile-getting-started.md) — 各 Profile 策略、依赖、中间件与 initdata 对照。

---

## 1. 架构概览

```
centag/start.sh
├── stack …          → deploy/stack/lib/stack.sh（中间件 / Mem0 / Pi 编排）
└── profile <name> … → profile-stack.sh → stack ensure → 应用层 compose
```

| 层级 | 位置 | 职责 |
|------|------|------|
| 编排库 | `deploy/stack/lib/` | `stack_ensure_services`、健康检查、日志 |
| 共享定义 | `deploy/stack/compose/fragments/` | PG、Redis、ES、Qdrant、Neo4j、Ollama、Mem0 |
| 场景覆盖 | `deploy/stack/compose/overrides/` | stack-middleware、stack-mem0、profile-agent-memory |
| Profile 应用层 | `config/profiles/<name>/docker-compose.yaml` | Centag 应用容器（+ Pi，agent-memory） |
| 网络叠加 | `docker-compose.stack-network.yaml` | 应用接入 `deploy/stack-network` |

**环境变量链**（后者覆盖前者）：

```
deploy/stack/.env  →  config/profiles/<name>/.env  →  config/secrets/.env
```

---

## 2. 三种启动路径

| 路径 | 命令 | 适用场景 |
|------|------|----------|
| **B — Profile（推荐）** | `./start.sh profile <name> up` | 场景化一键部署 |
| **A — Stack 手动** | `./start.sh stack ensure …` + `./start.sh docker up` | 精细控制中间件 |
| **C — embedded 回退** | agent-memory 设 `STACK_MODE=embedded` | 单 compose 内嵌全栈（勿与 stack 默认端口并行） |

### Profile 命令速查

```bash
./start.sh profile list
./start.sh profile gateway up
./start.sh profile cached up
./start.sh profile agent-memory up
./start.sh profile <name> status|logs|down|reset
```

### Stack 命令速查（与 Profile 同源 lib）

```bash
./start.sh stack ensure postgresql ollama
./start.sh stack ensure postgresql qdrant ollama mem0
./start.sh stack status
./start.sh stack logs mem0
./start.sh stack start all    # 等价于 stack 子模块完整编排
```

---

## 3. Profile 对照表（设计差异）

| 维度 | gateway | cached | agent-memory |
|------|---------|--------|--------------|
| 应用 DB | **SQLite** | PostgreSQL | PostgreSQL（`auto`） |
| STACK_DEPS | 无（默认）/ `ollama`（可选） | `postgresql ollama` | `postgresql qdrant ollama mem0` |
| 应用容器 | Centag | Centag | Centag + Pi |
| stack .env 进容器 | **否** | 是 | 是 |
| entrypoint 等 PG | **否** | 是 | 是 |

**为何 gateway 日志里曾出现 PostgreSQL？** 旧版共用 entrypoint 默认 `POSTGRES_ENABLED=true`，且 gateway compose 注入了 `deploy/stack/.env`。现已分离：gateway 容器只用 `config/profiles/gateway/.env` + `config/secrets/.env`；entrypoint 按 `LLM_PROXY_DB_DRIVER=sqlite` 跳过 PG 等待。

`manifest.conf` 中 `STACK_DEPS` **必须加引号**，例如 `STACK_DEPS="postgresql ollama"`。

---

## 4. 各 Profile 说明

### gateway

- SQLite 内置，无 PG 依赖；**`CENTAG_EDITION=personal`**（发行包 gateway → 运行时 personal）
- **默认** `OLLAMA_ENABLED=false`：仅启动应用容器，使用线上 API Key
- `OLLAMA_ENABLED=true` 时自动 `stack ensure ollama`，应用通过 `deploy/stack-network` 访问 `centag-ollama`

详见 [`config/profiles/gateway/README.md`](../../config/profiles/gateway/README.md)。

### cached

- 缓存存储依赖 stack PostgreSQL（pgvector）
- `OLLAMA_ENABLED=false` 时仍 ensure PostgreSQL，但不启动 Ollama
- 宿主机连接 PG：`centag-postgresql:5432`（容器内）/ `localhost:5432`（宿主机映射）

详见 [`config/profiles/cached/README.md`](../../config/profiles/cached/README.md)。

### agent-memory

- **modular（默认）**：中间件走 stack，Pi Sandbox 留在 profile compose
- Centag 连接：`centag-postgresql`、`centag-qdrant`、`centag-ollama`、`centag-mem0`
- Mem0 宿主机端口：`20061`（映射容器 `8000`）
- **embedded 回退**：`config/profiles/agent-memory/docker-compose.embedded.yaml`

详见 [`config/profiles/agent-memory/README.md`](../../config/profiles/agent-memory/README.md)。

---

## 5. Compose 文件结构（单 Profile）

以 agent-memory modular 为例：

```
config/profiles/agent-memory/
├── docker-compose.yaml              # 应用层：centag + pi-agent + pi-client
├── docker-compose.stack-network.yaml # centag 接入 stack 网络 + 主机名
├── docker-compose.embedded.yaml     # legacy 全栈内嵌
└── manifest.conf                    # STACK_MODE / STACK_DEPS
```

`profile_invoke_compose` 根据 `manifest.conf` 自动选择 compose 文件组合。

---

## 6. 注意事项

1. **端口互斥**：embedded agent-memory 与 `./start.sh stack start all` 不可同机默认端口并行运行。
2. **Mem0 密钥**：`MEM0_JWT_SECRET`、`MEM0_ADMIN_API_KEY` 需在 profile `.env` 或 `config/secrets/.env` 配置。
3. **容器间 Mem0 端口**：应用连 Mem0 用内部端口 `8000`（`MEM0_HOST=centag-mem0`），宿主机用 `20061`。
4. **Submodule**：`deploy/stack` 为 git submodule，克隆后执行 `git submodule update --init`。

---

## 7. 相关文档

| 文档 | 说明 |
|------|------|
| [`profile-getting-started.md`](./profile-getting-started.md) | **Profile 入门手册**（策略 / 依赖 / initdata） |
| [`config/profiles/README.md`](../../config/profiles/README.md) | Profile 选择与对比 |
| [`config/profiles/TESTING.md`](../../config/profiles/TESTING.md) | 验证手册 |
| [`deploy/stack/docs/ARCHITECTURE.md`](../../deploy/stack/docs/ARCHITECTURE.md) | Stack 目录与 fragments |
| [`deploy/stack/docs/getting-started.md`](../../deploy/stack/docs/getting-started.md) | Stack 首次启动 |
| [`docs/docker/Docker-Compose部署指南.md`](../deploy/docker/Docker-Compose部署指南.md) | 传统 docker up 路径 |