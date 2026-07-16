# 中间件操作手册

`middleware/` 提供 Centag 生态常用的基础设施，**各组件可独立启动**，无相互硬依赖。Compose 服务定义与 `compose/fragments/` 共享，由 `middleware/docker-compose.yml` include 后叠加 stack 专用覆盖（端口、`centag-*` 容器名、`base` profile）。

## 组件与端口

| 服务 | 命令 | 宿主机端口（默认） | 说明 |
|------|------|-------------------|------|
| PostgreSQL | `./start.sh start pg` | 5432 | pgvector |
| Redis | `./start.sh start rd` | **26379** | 缓存 |
| Elasticsearch | `./start.sh start es` | **29200** | 可选向量/KV |
| Qdrant | `./start.sh start qd` | 6333 | 向量库 |
| Neo4j | `./start.sh start n4j` | 7474 / 7687 | 图数据库 |
| Ollama | `./start.sh start ol` | **21434** | 本地模型 |

> 注意：Redis/ES/Ollama 的**宿主机端口**与容器内端口不同，以 `.env` 为准。

## 启动

```bash
./start.sh start base     # PG + Redis + ES + Kibana + Qdrant + Neo4j
./start.sh start pg       # 仅 PostgreSQL
./start.sh status
./start.sh health
```

`start base` 会创建 Docker 网络 `centag-stack-network`，供 Mem0 等需要访问中间件的服务加入。

## 环境变量

见根目录 `.env.example` 第一节至第七节。

## 与 Mem0 的关系

Mem0 是 middleware 的**消费者**，不是 middleware 的一部分。仅在使用 Mem0 时需要同时运行相关中间件。