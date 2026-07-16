# Docker Compose 常见问题

> **2026-05**：本仓库 compose **仅含 Centag**。中间件与 Ollama 的 compose 行为以 **`deploy/stack`** 为准；下文若出现「按 profile 只起 ES/Ollama」等描述，主要指 **历史布局** 或 **stack** 侧，请勿与主仓 `./start.sh docker up es` 等旧命令混为一谈（主仓会警告并只起 Centag）。

## "No services to build" 警告

### 问题描述

```bash
WARN[0000] No services to build
```

### 原因

这个警告是正常的，表示当前启动的 profile 中不包含需要构建的服务。

例如：
- `es` profile 只包含 `elasticsearch` 和 `kibana`（官方镜像，无需构建）
- `ollama` profile 只包含 `ollama`（官方镜像，无需构建）
- `service` profile 包含 `centag`（需要构建）

### 是否影响使用？

**完全不影响。** 这个警告只是提示信息，服务会正常启动。

### 如何消除？

这个警告无法消除，因为它反映的是实际情况。如果你构建了 `centag` 镜像，使用 `service` profile 时就不会看到此警告。

## "ASYNC_INIT_PID" 警告

### 问题描述

```bash
WARN[0000] The "ASYNC_INIT_PID" variable is not set. Defaulting to a blank string.
```

### 原因

Docker Compose 在解析配置时会读取所有服务的 entrypoint 脚本。Ollama 服务的 entrypoint 脚本中使用了 `ASYNC_INIT_PID` 变量来跟踪异步初始化进程的 PID，但这个变量只在运行时使用，不在 environment 中定义。

### 是否影响使用？

**完全不影响。** 这个警告只在配置解析阶段出现，不影响 Ollama 服务的正常运行。

### 是否会出现？

- **主仓库** `./start.sh docker up`：仅解析当前 compose（通常只有 Centag）；是否出现取决于 compose 内容。
- **deploy/stack** 中若 Ollama 服务 entrypoint 仍引用未在 environment 声明的变量，解析阶段可能出现类似提示；以 stack 内实际 compose 为准。

## "No services to build" 与 "ASYNC_INIT_PID" 的关系

这两个警告都是正常的：

1. **"No services to build"**: 说明当前 profile 不包含需要构建的服务
2. **"ASYNC_INIT_PID"**: 说明解析到了 Ollama 配置，但变量未在 environment 中定义

它们都不影响功能，可以安全忽略。

## 单容器 / 多服务启动说明

### 主仓库（centag）当前行为

| 命令 | 说明 |
|------|------|
| `./start.sh docker up` | 启动 **Centag** 应用容器 |
| `./start.sh docker up <多余参数>` | **仍只启动 Centag**；打印告警并忽略该参数 |

### 中间件与多服务

在 **`deploy/stack`** 中按需启动 PostgreSQL、Redis、Elasticsearch、Ollama 等，并在本仓库 **`config/secrets/.env`** 中按需填写 `PG_*`、`REDIS_*`、`ELASTICSEARCH_*`、`OLLAMA_HOST` 等，使 Centag 容器能访问依赖。

```bash
cd deploy/stack && ./start.sh help
cd ../centag && ./start.sh docker up
```

## 验证服务状态

### 查看所有容器

```bash
docker ps --filter "name=centag-"
```

### 检查特定服务

```bash
# ES
curl http://localhost:29200/_cluster/health

# Kibana
curl http://localhost:25601/api/status

# Redis
docker exec -it centag-redis redis-cli ping

# ChromaDB
curl http://localhost:8001/api/v2/heartbeat

# Ollama
curl http://localhost:21434/api/tags

# Proxy
curl http://localhost:20060/health
```

## 常见问题排查

### 1. 服务启动失败

```bash
# 查看日志
docker logs centag-es
docker logs centag-ollama
docker logs centag-redis
```

### 2. 端口被占用

```bash
# 检查端口
lsof -i :9200
lsof -i :21434

# 停止占用进程或修改端口
```

### 3. 内存不足

```bash
# 检查内存
free -h

# 减少 ES 堆大小（在 docker-compose.yaml 中）
environment:
  - "ES_JAVA_OPTS=-Xms1g -Xmx1g"
```

### 4. GPU 不工作

```bash
# 检查 GPU
nvidia-smi

# 测试 NVIDIA Container Toolkit
docker run --rm --gpus all nvidia/cuda:11.0.3-base-ubuntu20.04 nvidia-smi
```

## 总结

- ✅ **"No services to build"**: 正常，不影响使用
- ✅ **"ASYNC_INIT_PID"**: 正常，不影响使用
- ✅ **单独启动**: 支持单个 profile 启动
- ❌ **多参数启动**: 不支持，使用组合 profile 或分别启动

所有功能正常工作，这些警告可以安全忽略！
