# Docker Compose 快速参考

> **2026-05 真源**：本仓库 `docker/docker-compose.yaml` **仅含 Centag**。PostgreSQL、Redis、Elasticsearch、ChromaDB、Ollama、Mem0 等请使用 **`deploy/stack`**（`cd deploy/stack && ./start.sh help`）。以下 `./start.sh docker up <profile>` 旧写法仍会被接受，但 **`start.sh` 只会启动 Centag 并打印警告**。

## 命令速查表

### 启动服务

| 命令 | 说明 | 实际行为（主仓库） |
|------|------|-------------------|
| `./start.sh docker up` | 启动应用容器（默认） | 仅 Centag |
| `./start.sh docker up <任意旧 profile>` | 已废弃写法 | 仍只启动 Centag；多余参数会告警，请删 profile |
| `cd deploy/stack && ./start.sh start base` | 启动栈内基础依赖 | 以 stack 文档为准 |
| `cd deploy/stack && ./start.sh start ollama` | 启动 Ollama | 以 stack 文档为准 |

### 管理服务

| 命令 | 说明 |
|------|------|
| `./start.sh docker down` | 停止所有服务 |
| `./start.sh docker status` | 查看服务状态 |
| `./start.sh docker logs` | 查看所有日志 |
| `./start.sh docker logs es` | 查看 ES 日志 |
| `./start.sh docker logs redis` | 查看 Redis 日志 |
| `./start.sh docker logs centag` | 查看 Proxy 日志 |

### 构建和清理

| 命令 | 说明 |
|------|------|
| `./start.sh docker build` | 构建 Docker 镜像 |
| `./start.sh docker clean` | 清理所有资源（容器、镜像、数据卷） |

## 服务端口

| 服务 | 内部端口 | 外部端口 | 说明 |
|------|---------|---------|------|
| Elasticsearch | 9200 | 9200 | HTTP API |
| Elasticsearch | 9300 | 9300 | 节点通信 |
| Kibana | 5601 | 5601 | Web 界面 |
| Redis | 6379 | 6379 | KV 存储 |
| ChromaDB | 8000 | 8001 | 向量 API |
| Ollama | 21434 | 21434 | 向量模型 API |
| Centag | 20060 | 20060 | 主服务 |

## 存储方案配置

### Elasticsearch 统一存储（推荐）

```bash
ELASTICSEARCH_ENABLED=true
REDIS_ENABLED=false
VECTOR_ENABLED=false
```

### Redis + Elasticsearch 混合

```bash
ELASTICSEARCH_ENABLED=true
REDIS_ENABLED=true
VECTOR_ENABLED=false
```

### Redis + ChromaDB 传统方案

```bash
ELASTICSEARCH_ENABLED=false
REDIS_ENABLED=true
VECTOR_ENABLED=true
VECTOR_TYPE=chromadb
```

## 验证命令

```bash
# Elasticsearch
curl http://localhost:29200/_cluster/health

# Redis
docker exec -it centag-redis redis-cli ping

# ChromaDB
curl http://localhost:8001/api/v2/heartbeat

# Ollama
curl http://localhost:21434/api/tags

# Centag
curl http://localhost:20060/health
```

## 常见场景

### 场景 1：仅测试 Elasticsearch

在 **deploy/stack** 启动 Elasticsearch 后，于 **`config/secrets/.env`** 配置 `ELASTICSEARCH_ADDR` 等，再 `./start.sh docker up` 启动 Centag。

### 场景 2：开发环境（ES + Redis）

在 **deploy/stack** 拉起依赖后，本仓库 `./start.sh docker up` 启动 Centag。

### 场景 3：生产环境（全栈）

`deploy/stack` 起中间件 + 数据面，本仓库 `./start.sh docker up`（或 `deploy.sh`）起 Centag。

### 场景 4：Ollama 向量模型

```bash
cd deploy/stack && ./start.sh start ollama
# 本仓库 config/secrets/.env 中配置 OLLAMA_HOST 指向可达的 Ollama
curl "${OLLAMA_HOST:-http://localhost:21434}/api/embeddings" -d '{"model":"bge-m3","prompt":"Hello"}'
```

## 故障排查

### 端口占用

```bash
# 检查端口
lsof -i :9200
lsof -i :6379

# 修改端口（在 docker-compose.yaml 中）
ports:
  - "9201:9200"
```

### 内存不足

```bash
# 检查内存
free -h

# 减少 ES 堆大小
# 在 docker-compose.yaml 中修改：
environment:
  - "ES_JAVA_OPTS=-Xms1g -Xmx1g"
```

### 查看日志

```bash
# 所有服务
./start.sh docker logs

# 特定服务
docker logs -f centag-es
docker logs -f centag-redis
docker logs -f centag
```

## 迁移到 Elasticsearch

```bash
# 1. 启动 ES
./start.sh docker up es

# 2. 修改配置
# 在 config/secrets/.env 中设置：
ELASTICSEARCH_ENABLED=true

# 3. 重启主服务
./start.sh docker up service

# 4. 验证
curl http://localhost:29200/_cluster/health
```

## 清理

```bash
# 停止服务
./start.sh docker down

# 清理所有资源（包括数据卷）
./start.sh docker clean

# 清理特定数据卷
docker volume rm centag_es_data
docker volume rm centag_redis_data
```

## 参考文档

- [Docker Compose 部署指南](./Docker-Compose部署指南.md)
- [ElasticSearch 插件使用指南](./ElasticSearch插件使用指南.md)
- [ElasticSearch 缓存优化方案](./ElasticSearch缓存优化方案.md)
