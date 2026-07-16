# Centag Stack — 架构设计

## 1. 设计原则

1. **服务包独立** — `middleware/`、`services/mem0/`、`services/pi-sandbox/` 可单独部署
2. **统一编排** — `lib/stack.sh` 负责启动、健康检查、日志；根目录 `start.sh` 为薄入口，centag `./start.sh stack` 同源加载
3. **配置集中** — 根目录 `.env` 为默认配置源（`--env-file` 注入各 compose）
4. **按需组网** — 仅 Mem0 等消费者加入共享网络 `centag-stack-network`

## 2. 目录结构

```
centag-stack/
├── start.sh                 # 薄入口 → lib/stack.sh
├── lib/
│   ├── stack-bootstrap.sh
│   ├── stack-core.sh
│   ├── stack-ensure.sh      # stack_ensure_middleware / stack_ensure_services
│   └── stack.sh
├── .env.example             # 全局配置模板
├── compose/
│   ├── fragments/           # 共享服务定义（PG/Redis/ES/Qdrant/Neo4j/Ollama/Mem0）
│   └── overrides/           # 部署场景覆盖（stack-middleware、stack-mem0、profile-agent-memory）
├── docs/                    # 文档（操作手册、架构）
├── middleware/              # 基础中间件包（独立）
│   ├── docker-compose.yml   # include fragments + stack-middleware override
│   └── README.md
└── services/
    ├── mem0/                # Mem0 服务包（依赖 middleware）
    │   ├── docker-compose.yml
    │   ├── Dockerfile
    │   └── README.md
    └── pi-sandbox/          # Pi Agent 网关（独立）
        ├── docker-compose.yml
        ├── Dockerfile
        ├── go-client/
        ├── pi-config/
        ├── docker/          # 开发双容器
        ├── scripts/
        └── README.md
```

## 3. 服务依赖关系

```
middleware/          services/mem0/          services/pi-sandbox/
(PG, Redis, ...)   (Mem0 Server)           (pi-server HTTP)
      │                    │                        │
      │    centag-stack-network (仅 mem0 加入)   │
      └────────────────────┘                        │
                                                     无 stack 内依赖
```

| 服务包 | 可独立启动 | 运行时依赖 |
|--------|-----------|-----------|
| middleware | ✅ | 无 |
| pi-sandbox | ✅ | LLM API Key |
| mem0 | ⚠️ 需中间件 | PG、Qdrant、Neo4j、Ollama |

## 4. 部署模式

```bash
# centag 根目录（推荐）
./start.sh stack start base
./start.sh stack ensure postgresql ollama
./start.sh stack start mem0
./start.sh stack start all

# 或 centag-stack 子模块内等价命令
./start.sh start base
```

### 与 Centag Profile 的关系

| Profile | STACK_DEPS | 说明 |
|---------|------------|------|
| gateway | `ollama` | 应用 SQLite，可选 stack Ollama |
| cached | `postgresql ollama` | 应用连 stack PG |
| agent-memory | `postgresql qdrant ollama mem0` | 应用 + Pi；中间件全走 stack |

Profile `up` 调用 `stack_ensure_services`（见 `profiles/_shared/profile-stack.sh`）。详见 centag [`docs/guide/deployment-profiles-and-stack.md`](../../docs/guide/deployment-profiles-and-stack.md)。

## 5. 网络

| 网络 | 创建者 | 使用者 |
|------|--------|--------|
| `centag-stack-network` | middleware compose / `ensure_middleware_network` | middleware、mem0、modular Profile 应用层 |
| `pi-network` | `services/pi-sandbox/docker/` 开发 compose | 开发双容器 |
| pi-sandbox 生产 compose 默认 bridge | Docker 自动 | 单容器生产 |

## 6. 数据卷

| Volume | 服务 |
|--------|------|
| `centag-stack_postgresql_data` 等 | middleware |
| `mem0_history_data` | mem0 |
| `pi-sandbox-sessions` / `pi-sandbox-workspace` | pi-sandbox |