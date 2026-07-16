# Mem0 操作手册

Mem0 Server 提供长期记忆存储与检索 API，供 Centag `business.mem0` 插件或外部客户端调用。

## 依赖说明

Mem0 **需要** middleware 中的以下服务（`start.sh start mem0` 会自动拉起）：

| 中间件 | 容器名 | 用途 |
|--------|--------|------|
| PostgreSQL | `centag-postgresql` | 记忆元数据 |
| Qdrant | `centag-qdrant` | 向量检索 |
| Neo4j | `centag-neo4j` | 图关系 |
| Ollama | `centag-ollama` | 本地嵌入/LLM（默认 provider） |

Mem0 通过 Docker 网络 `centag-stack-network` 访问上述服务。网络由 `start.sh` 或 `start base` 自动创建。

## 启动

```bash
cd centag-stack
cp .env.example .env
./start.sh start mem0
```

启动前自动检查：

- `POSTGRES_PASSWORD`、`MEM0_JWT_SECRET`、`MEM0_ADMIN_API_KEY` 等仍为占位符 → 自动生成
- LLM 未配置 → 弹出向导选择 Ollama 本地 / OpenAI 兼容 API / 混合嵌入方案

验证：

```bash
curl http://localhost:20061/health
bash services/mem0/test-mem0-service.sh
```

## 环境变量

| 变量 | 说明 |
|------|------|
| `MEM0_PORT` | 宿主机端口（默认 20061） |
| `MEM0_ADMIN_API_KEY` | 管理 API 密钥 |
| `MEM0_JWT_SECRET` | JWT 签名密钥 |
| `MEM0_POSTGRES_HOST` | PG 主机（默认 `centag-postgresql`） |
| `MEM0_QDRANT_HOST` | Qdrant 主机（默认 `centag-qdrant`） |
| `MEM0_NEO4J_URI` | Neo4j Bolt URI |
| `LLM_PROVIDER` | `ollama` 或 `openai` |
| `EMBEDDER_PROVIDER` | 嵌入服务 |
| `OLLAMA_BASE_URL` | 默认 `http://centag-ollama:11434` |

## 独立调试

若 middleware 已在其他环境运行，可通过 `.env` 覆盖主机名：

```bash
MEM0_POSTGRES_HOST=my-postgres.example.com
MEM0_QDRANT_HOST=my-qdrant.example.com
```

然后确保 Mem0 容器能解析并访问这些主机（同一 Docker 网络或 host 网络）。

## 停止与清理

```bash
./start.sh stop mem0
./start.sh clean mem0    # 删除容器、卷、镜像
```