# Centag Stack

与 [Centag](https://github.com/marmotcai/centag) 配套的**可选服务集合**：中间件、Mem0 记忆、Pi Agent 网关。编排逻辑在 `lib/`；与 centag 根目录 **`./start.sh stack`** 共用。

## 目录结构

```
centag-stack/
├── start.sh                    # 薄入口 → lib/stack.sh
├── lib/                        # stack-bootstrap / core / ensure / stack.sh
├── .env.example                # 全局环境变量
├── compose/
│   ├── fragments/              # 共享服务定义（PG、Redis、ES、Qdrant、Neo4j、Ollama、Mem0）
│   └── overrides/              # stack-middleware、stack-mem0、profile-agent-memory
├── docs/                       # 文档与操作手册
├── middleware/                 # include fragments + stack-middleware override
└── services/
    ├── mem0/
    └── pi-sandbox/
```

## 快速开始

### 在 centag 主仓库（推荐）

```bash
cd /path/to/centag
git submodule update --init

# Profile 一键（自动 stack ensure）
./start.sh profile personal up
./start.sh profile agent-memory up

# 仅管理 stack
./start.sh stack w
./start.sh stack ensure postgresql ollama
./start.sh stack start mem0
./start.sh stack status
```

架构说明：centag [`docs/guide/deployment-profiles-and-stack.md`](../docs/guide/deployment-profiles-and-stack.md)

### 在子模块内

```bash
cd centag-stack
cp .env.example .env
./start.sh w
./start.sh start mem0
./start.sh status
```

## 服务包

| 包 | 路径 | 端口 | 依赖 |
|----|------|------|------|
| middleware | `middleware/` + `compose/fragments/` | 见文档 | 无 |
| mem0 | `services/mem0/` | 20061 | middleware |
| pi-sandbox | `services/pi-sandbox/` | 20062 | 无（需 API Key） |

## 文档

| 文档 | 说明 |
|------|------|
| [docs/README.md](docs/README.md) | 文档索引 |
| [docs/getting-started.md](docs/getting-started.md) | 安装与首次启动 |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 架构、fragments、Profile 关系 |
| [docs/pi-sandbox-guide.md](docs/pi-sandbox-guide.md) | Pi 操作手册 |
| [docs/mem0-guide.md](docs/mem0-guide.md) | Mem0 操作手册 |
| [docs/middleware-guide.md](docs/middleware-guide.md) | 中间件说明 |
| [docs/AGENTS.md](docs/AGENTS.md) | AI Agent 入口 |

## 常用命令

```bash
# centag 根目录
./start.sh stack start <服务>
./start.sh stack ensure postgresql qdrant ollama mem0
./start.sh stack stop <服务>
./start.sh stack logs <服务>
./start.sh stack health

# 子模块内等价：./start.sh start <服务> 等
```