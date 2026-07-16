# Docker Compose 部署指南

> **2026-06 真源**：本仓库 **`docker/docker-compose.yaml` 仅包含 Centag**。中间件由 **`deploy/stack`** 编排。  
> **推荐**：场景化部署用 **`./start.sh profile <name> up`**（自动 `stack ensure` + 应用层），见 [`guide/deployment-profiles-and-stack.md`](../guide/deployment-profiles-and-stack.md)。  
> 本文档描述 **路径 A**：手动 stack + `./start.sh docker up`。

## 概述

### 路径 B — Profile（推荐）

```bash
./start.sh profile gateway up      # 或 cached / agent-memory
```

### 路径 A — 手动分层（本文档主体）

1. 在 **centag 根目录** 执行 **`./start.sh stack ensure …`**（或 `cd deploy/stack && ./start.sh start …`）。
2. 配置 **`config/secrets/.env`**（连接地址指向 stack 容器名或宿主机端口）。
3. 执行 **`./start.sh docker up`** 启动 Centag 容器。

### 本 compose 拉起的服务（主仓库）

| 服务 | 默认端口 | 说明 |
|------|-----------|------|
| Proxy Claw | 20060 | 唯一由本仓库 compose 定义的服务 |

Elasticsearch、Kibana、Redis、ChromaDB、Ollama、PostgreSQL、Mem0 等见 **deploy/stack**；下文「配置说明」中的变量仍用于 **让 Centag 连接** 这些外部依赖。

## 快速开始

### 1. 启动依赖（stack）

```bash
cd /path/to/centag
./start.sh stack ensure postgresql ollama   # 按需组合服务
./start.sh stack status
# 或进入子模块：cd deploy/stack && ./start.sh start base
```

### 2. 启动 Centag（本仓库）

```bash
cd /path/to/centag
./start.sh docker up
```

### 3. 验证

```bash
curl "http://localhost:${LLM_PROXY_SERVER_PORT:-20060}/health"
```

## 配置说明

### 环境变量配置

在 **`config/secrets/.env`** 中配置运行时参数（与 compose `env_file` 一致）：

```bash
# Elasticsearch 配置（推荐：统一存储方案）
ELASTICSEARCH_ENABLED=true
ELASTICSEARCH_ADDR=elasticsearch:9200
ELASTICSEARCH_USERNAME=
ELASTICSEARCH_PASSWORD=
ELASTICSEARCH_INDEX_EXACT=cache_exact_index
ELASTICSEARCH_INDEX_SEMANTIC=cache_semantic_index
ELASTICSEARCH_VECTOR_DIMENSION=1536

# Redis 配置（传统方案：KV 缓存）
REDIS_ENABLED=true
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0

# 向量数据库配置（传统方案：ChromaDB）
VECTOR_ENABLED=false
VECTOR_ADDR=chromadb:8000
VECTOR_TYPE=chromadb

# Ollama 向量模型配置
OLLAMA_ENABLED=false
OLLAMA_ADDR=ollama:21434
OLLAMA_DEFAULT_MODEL=bge-m3
OLLAMA_AUTOLOAD_MODEL=true
```

### Elasticsearch 推荐配置

启用 Elasticsearch 作为统一存储后端：

```bash
ELASTICSEARCH_ENABLED=true
VECTOR_ENABLED=false  # 禁用 ChromaDB
REDIS_ENABLED=false  # 可选：如果只需要 ES，可以禁用 Redis
```

### 混合配置

Redis + Elasticsearch 组合：

```bash
ELASTICSEARCH_ENABLED=true  # 向量存储
REDIS_ENABLED=true          # KV 缓存
VECTOR_ENABLED=false       # 禁用 ChromaDB
```

## 存储方案对比

### 方案 1：Elasticsearch 统一存储（推荐）

**优点：**
- 架构简单，单数据源
- 支持高性能向量搜索（HNSW 索引）
- 内置 TTL 和数据过期
- 完善的监控和分析（Kibana）
- 成本低（节省 30-40%）

**配置：**
```bash
ELASTICSEARCH_ENABLED=true
REDIS_ENABLED=false
VECTOR_ENABLED=false
```

### 方案 2：Redis + ChromaDB（传统）

**优点：**
- Redis 极快的 KV 查询性能
- ChromaDB 专为向量搜索优化

**缺点：**
- 架构复杂，多数据源
- 需要多节点部署
- 数据一致性难以保证

**配置：**
```bash
ELASTICSEARCH_ENABLED=false
REDIS_ENABLED=true
VECTOR_ENABLED=true
VECTOR_TYPE=chromadb
```

### 方案 3：Elasticsearch + Redis（混合）

**优点：**
- 利用 Redis 的高速 KV 查询
- 利用 Elasticsearch 的向量搜索
- 保留原有 Redis 缓存层

**配置：**
```bash
ELASTICSEARCH_ENABLED=true
REDIS_ENABLED=true
VECTOR_ENABLED=false
```

## 常用命令

### 查看服务状态

```bash
./start.sh docker status
```

### 查看日志

```bash
# 查看所有服务日志
./start.sh docker logs

# 查看特定服务日志
./start.sh docker logs elasticsearch
./start.sh docker logs redis
./start.sh docker logs centag
```

### 停止服务

```bash
./start.sh docker down
```

### 清理所有资源

```bash
./start.sh docker clean
```

**警告：此命令将删除所有容器、镜像和数据卷！**

## 验证部署

### 1. 检查 Elasticsearch

```bash
curl http://localhost:29200/_cluster/health
```

返回示例：
```json
{
  "cluster_name" : "centag-cluster",
  "status" : "green",
  "number_of_nodes" : 1,
  "active_primary_shards" : 5,
  "active_shards" : 5
}
```

### 2. 检查 Kibana

访问：http://localhost:25601

### 3. 检查 Redis

```bash
docker exec -it centag-redis redis-cli ping
# 应返回 PONG
```

### 4. 检查 ChromaDB

```bash
curl http://localhost:8001/api/v2/heartbeat
```

### 5. 检查 Ollama

```bash
# 查看已安装的模型
curl http://localhost:21434/api/tags

# 测试向量生成
curl http://localhost:21434/api/embeddings -d '{"model":"bge-m3","prompt":"Hello"}'
```

### 6. 检查 Proxy Claw

```bash
curl http://localhost:20060/health
```

## 性能调优

### Elasticsearch

调整 JVM 堆大小（在 `docker-compose.yaml` 中）：

```yaml
environment:
  - "ES_JAVA_OPTS=-Xms4g -Xmx4g"  # 4GB 堆大小
```

建议：
- 开发环境：2GB
- 生产环境：4-8GB
- 大规模数据：8-16GB

### Redis

配置最大内存（在 `docker-compose.yaml` 中）：

```yaml
command: >
  redis-server --appendonly yes --requirepass "$${REDIS_PASSWORD}" --maxmemory 2gb
```

### ChromaDB

ChromaDB 默认自动优化，无需额外配置。

## 故障排查

### Elasticsearch 启动失败

1. 检查内存：
   ```bash
   free -h
   ```
   至少需要 4GB 可用内存。

2. 查看日志：
   ```bash
   docker logs centag-es
   ```

3. 重置数据：
   ```bash
   docker-compose down -v
   docker-compose up -d
   ```

### 端口冲突

检查端口占用：
```bash
lsof -i :9200
lsof -i :6379
```

修改端口（在 `docker-compose.yaml` 中）：
```yaml
ports:
  - "9201:9200"  # 将 9200 改为 9201
```

### GPU 不工作

检查 NVIDIA Container Toolkit：
```bash
docker run --rm --gpus all nvidia/cuda:11.0.3-base-ubuntu20.04 nvidia-smi
```

如果失败，安装 NVIDIA Container Toolkit：
```bash
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list
sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker
```

## 生产环境部署建议

### 1. 使用 Elasticsearch 作为主存储

在 **deploy/stack** 启动 Elasticsearch（及可选 Kibana、Ollama）后，于 **`config/secrets/.env`** 启用 `ELASTICSEARCH_ENABLED` 并填写可达地址，再执行：

```bash
./start.sh docker up
```

### 2. 配置持久化存储

中间件数据卷由 **deploy/stack** 各服务的 compose 定义；本仓库 `docker/docker-compose.yaml` 仅包含 Centag 相关卷（若有）。

### 3. 备份策略

定期备份 Docker volumes：

```bash
# 备份 ES 数据
docker run --rm -v centag_es_data:/data -v $(pwd):/backup ubuntu tar czf /backup/es-backup.tar.gz /data

# 备份 Redis 数据
docker run --rm -v centag_redis_data:/data -v $(pwd):/backup ubuntu tar czf /backup/redis-backup.tar.gz /data

# 备份 ChromaDB 数据
docker run --rm -v centag_chromadb_data:/data -v $(pwd):/backup ubuntu tar czf /backup/chromadb-backup.tar.gz /data
```

### 4. 监控

使用 Kibana 监控 Elasticsearch：
- http://localhost:25601

监控 Redis：
```bash
docker exec -it centag-redis redis-cli info
```

## 迁移指南

### 从 Redis + ChromaDB 迁移到 Elasticsearch

1. **启动 Elasticsearch**（在 **deploy/stack** 中启动 ES 服务，命令以该仓库为准）

2. **配置 Proxy Claw 启用 ES**
   ```bash
   ELASTICSEARCH_ENABLED=true
   ```

3. **双写验证**
   - 同时写入 Redis/ChromaDB 和 ES
   - 验证数据一致性

4. **切换读取**
   - 将读取流量切换到 ES
   - 监控性能和错误率

5. **清理旧数据**
   ```bash
   # 停止 Redis 和 ChromaDB
   ./start.sh docker down
   # 删除 volumes（谨慎！）
   docker volume rm centag_redis_data centag_chromadb_data
   ```

## 参考

- [Elasticsearch 插件使用指南](./ElasticSearch插件使用指南.md)
- [ElasticSearch 缓存优化方案](../../archive/deprecated/docs/ElasticSearch缓存优化方案.md)（已归档）
- [系统架构分析报告](../../archive/deprecated/docs/guide/系统架构分析报告.md)（已归档）
