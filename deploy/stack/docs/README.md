# Centag Stack 文档

Centag Stack 是 **与 Centag 主项目配套的可选服务集合**，每个服务包可独立部署。编排逻辑位于 `lib/`，由 centag 根目录 **`./start.sh stack`** 与 stack 内 **`./start.sh`** 共用。

## 文档索引

| 文档 | 说明 |
|------|------|
| [getting-started.md](getting-started.md) | 首次安装与启动 |
| [middleware-guide.md](middleware-guide.md) | PostgreSQL、Redis、Qdrant、Ollama 等中间件 |
| [mem0-guide.md](mem0-guide.md) | Mem0 记忆服务 |
| [pi-sandbox-guide.md](pi-sandbox-guide.md) | Pi Agent HTTP 网关 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 架构、fragments、与 Profile 关系 |
| [AGENTS.md](AGENTS.md) | AI 编码智能体入口 |
| [CONVENTIONS.md](CONVENTIONS.md) | 编码规范 |

**Centag 侧（Profile + Stack 协作）**：[`../../docs/guide/profile-getting-started.md`](../../docs/guide/profile-getting-started.md)

## 服务包一览

```bash
./start.sh list    # 打印所有服务包及启动命令
```

| 包 | 路径 | 端口 | 依赖 |
|----|------|------|------|
| middleware | `middleware/` + `compose/fragments/` | 见各组件 | 无 |
| mem0 | `services/mem0/` | 20061 | middleware（PG/Qdrant/Neo4j/Ollama） |
| pi-sandbox | `services/pi-sandbox/` | 20062 | 无（仅需 LLM API Key） |

## 常用命令

在 **centag 根目录**（推荐）：

```bash
./start.sh stack w                    # 交互式向导
./start.sh stack ensure postgresql    # 仅确保依赖
./start.sh stack start mem0           # 启动 Mem0（自动拉 middleware）
./start.sh stack status
./start.sh stack health
./start.sh stack logs ollama
```

在 **centag-stack/** 子模块内命令等价（薄 `start.sh` → `lib/stack.sh`）。