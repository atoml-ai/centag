# Centag Stack — Agent 入口

## 1. 项目是什么

**Centag Stack** 是与 Centag 主项目配套的**可选服务集合**（中间件、Mem0、Pi Sandbox）。编排逻辑在 `lib/`；centag 根目录 **`./start.sh stack`** 与子模块 **`./start.sh`** 同源。Compose 服务定义在 `compose/fragments/`。

## 2. 服务端口（宿主机，真源见 `.env.example`）

| 服务 | 宿主机端口 | 容器 |
|------|-----------|------|
| PostgreSQL | 5432 | centag-postgresql |
| Redis | **26379** | centag-redis |
| Elasticsearch | **29200** | centag-elasticsearch |
| Qdrant | 6333 | centag-qdrant |
| Neo4j | 7474 / 7687 | centag-neo4j |
| Ollama | **21434** | centag-ollama |
| Mem0 | 20061 | centag-mem0 |
| Pi Sandbox | 20062 | centag-pi-sandbox |

## 3. 文档索引

| 任务 | 文档 |
|------|------|
| 首次启动 | [getting-started.md](getting-started.md) |
| Pi Sandbox | [pi-sandbox-guide.md](pi-sandbox-guide.md) |
| Mem0 | [mem0-guide.md](mem0-guide.md) |
| 中间件 | [middleware-guide.md](middleware-guide.md) |
| 架构 | [ARCHITECTURE.md](ARCHITECTURE.md) |

## 4. 常用命令

```bash
# centag 根目录（推荐，与 profile 同源）
./start.sh stack list
./start.sh stack ensure postgresql ollama
./start.sh stack start mem0
./start.sh stack status

# 子模块内等价
cd centag-stack && ./start.sh start mem0
```

## 5. 约束

- ❌ 禁止硬编码密钥
- ❌ 禁止提交 `.env`
- ✅ 改 `models.json` 后 `./start.sh restart pi-sandbox`