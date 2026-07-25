# 环境变量配置说明

## 架构设计原则

本项目采用 **「域名 + 按环境解析」** 的方式连接中间件，同一份配置在本地与远程都能正确连到宿主机服务：

- **配置层**：默认使用固定域名（`pg.atoml.net`、`es.atoml.net`、`redis.atoml.net`、`chromadb.atoml.net`、`ollama.atoml.net`），首次 seed 写入 DB 后不再依赖环境变量。
- **解析层**：通过「域名解析」区分环境，不改配置即可切换：
  - **本地调试**：在宿主机 `/etc/hosts` 将上述域名指向 `127.0.0.1`，即可连到本机中间件。
  - **Drone/远程部署**：在 Docker Compose 中通过 `extra_hosts` 将上述域名解析为 `HOST_IP`（宿主机 IP），容器内即可连到宿主机上的中间件。

这样本地用「域名 → 127.0.0.1」，远程用「域名 → 宿主机 IP」，无需在代码或 DB 里写死 IP。

## 环境变量列表

### 域名解析相关

- `HOST_IP`: 宿主机IP地址（默认值：`localhost`，远程部署时设置为实际IP，如 `192.168.1.5`）

### 中间件配置

#### PostgreSQL
- `POSTGRES_HOST`: PostgreSQL主机地址（默认值：`$HOST_IP` 或 `localhost`）
- `POSTGRES_PORT`: PostgreSQL端口（默认值：`5432`）
- `POSTGRES_USER`: PostgreSQL用户名（默认值：`postgres`）
- `POSTGRES_PASSWORD`: PostgreSQL密码
- `POSTGRES_DB`: PostgreSQL数据库名（默认值：`centag`）

#### Elasticsearch
- `ELASTICSEARCH_ADDR`: Elasticsearch地址（默认值：`http://$HOST_IP:29200` 或 `http://localhost:29200`）
- `ELASTICSEARCH_USERNAME`: Elasticsearch用户名（默认值：`elastic`）
- `ELASTICSEARCH_PASSWORD`: Elasticsearch密码

#### Redis
- `REDIS_ADDR`: Redis地址（默认值：`$HOST_IP:26379` 或 `localhost:26379`）
- `REDIS_PASSWORD`: Redis密码

#### ChromaDB
- `CHROMADB_ADDR`: ChromaDB地址（默认值：`$HOST_IP:28000` 或 `localhost:28000`）

#### Ollama
- `OLLAMA_HOST`: Ollama服务地址（默认值：`http://$HOST_IP:21434` 或 `http://localhost:21434`），程序优先读取此变量
- `OLLAMA_API_KEY`: Ollama API密钥
- `LLM_PROXY_INIT_BACKEND_URL`: 已合并到 `OLLAMA_HOST`，保留仅为向后兼容

## 使用方法

### 场景1：本地开发（域名指 127.0.0.1）

默认 seed 使用域名（如 `pg.atoml.net`）。本地调试时在 **宿主机** 的 `/etc/hosts` 中把这些域名指到本机，即可连到本机中间件：

```text
127.0.0.1  pg.atoml.net es.atoml.net redis.atoml.net chromadb.atoml.net ollama.atoml.net
```

然后照常启动：

```bash
# 启动 Docker 中间件（端口映射到本机）
docker-compose up -d

# 运行后端（连接 pg.atoml.net 等时解析为 127.0.0.1）
./start.sh run
```

前端本地调试时默认连 `127.0.0.1:20060`（见 `web/.env.development`），无需改 hosts。

### 场景2：Drone/远程部署（域名指宿主机 IP）

使用 `docker-compose.prod.yaml` 时，已通过 **extra_hosts** 将上述域名解析为 `HOST_IP`。只需在 Drone 或部署环境中设置 `HOST_IP` 为宿主机实际 IP（如 `192.168.1.5`），容器内访问 `pg.atoml.net` 等即会连到宿主机上的中间件。

**Drone 中**（已在 `.drone.yml` 中注入）：

```yaml
environment:
  HOST_IP: "192.168.1.5"
```

**本地手动部署时**：

```bash
export HOST_IP=192.168.1.5   # 或 source config/secrets/.env
docker compose -f deploy/docker/docker-compose.prod.yaml up -d
```

容器内解析结果：
- `pg.atoml.net` → HOST_IP（如 192.168.1.5）
- `es.atoml.net`、`redis.atoml.net`、`chromadb.atoml.net`、`ollama.atoml.net` 同理

### 场景3：自定义地址（覆盖默认配置）

也可以直接使用环境变量指定完整的地址，优先级高于 `HOST_IP`：

```bash
docker run -d \
  -e POSTGRES_HOST=192.168.1.10 \
  -e POSTGRES_PORT=5432 \
  -e ELASTICSEARCH_ADDR=http://192.168.1.11:9200 \
  -e REDIS_ADDR=192.168.1.12:6379 \
  -e CHROMADB_ADDR=192.168.1.13:8000 \
  -e LLM_PROXY_INIT_BACKEND_URL=http://192.168.1.14:21434 \
  -p 20060:20060 \
  centag
```

## 端口配置

各中间件的默认端口（Docker 映射端口）：

| 服务 | 本地端口 | 容器内端口 | 环境变量 |
|------|----------|------------|----------|
| PostgreSQL | 5432 | 5432 | `POSTGRES_PORT` |
| Elasticsearch | 29200 | 9200 | `ELASTICSEARCH_ADDR` |
| Redis | 26379 | 6379 | `REDIS_ADDR` |
| ChromaDB | 28000 | 8000 | `CHROMADB_ADDR` |
| Ollama | 21434 | 21434 | `OLLAMA_HOST` |
| Centag | 20060 | 20060 | `LLM_PROXY_SERVER_PORT` |

## 流水线模板模式开关

通过环境变量启用插件化的流水线模板模式（替代 legacy 硬编码实现）：

| 环境变量 | 说明 | 默认值 |
|---------|------|--------|
| `LLM_PROXY_MODE_A_TEMPLATE_ENABLED` | 启用 #a 审核模式的流水线模板 | `false` |
| `LLM_PROXY_MODE_O_TEMPLATE_ENABLED` | 启用 #o 优化模式的流水线模板 | `false` |
| `LLM_PROXY_MODE_D_TEMPLATE_ENABLED` | 启用 #d 直接后端模式的流水线模板 | `false` |
| `LLM_PROXY_MODE_T_TEMPLATE_ENABLED` | 启用 #t 透明代理模式的流水线模板 | `false` |
| `LLM_PROXY_MODE_F_TEMPLATE_ENABLED` | （历史）#f 降级模式模板开关；模板已不再预置 | `false` |
| `LLM_PROXY_MODE_M_TEMPLATE_ENABLED` | 启用 #m 模型匹配模式的流水线模板 | `false` |
| `LLM_PROXY_MODE_C_TEMPLATE_ENABLED` | 启用 #c 意图分类模式的流水线模板 | `false` |
| `LLM_PROXY_MODE_P_TEMPLATE_ENABLED` | 启用 #p 流水线模式的流水线模板 | `false` |

**使用示例**：
```bash
# 启用所有流水线模板
export LLM_PROXY_MODE_A_TEMPLATE_ENABLED=true
export LLM_PROXY_MODE_O_TEMPLATE_ENABLED=true
# ... 其他模式

# 启动服务
./start.sh run
```

**验证方式**：启用后，响应头会包含 `X-*-Mode-Path: pipeline-template` 标记。

## 安全相关环境变量（透明代理 / CORS）

- `LLM_PROXY_TRANSPARENT_PROXY_ALLOW_PRIVATE`: 是否允许透明代理目标为私网/回环地址（默认 `false`）
- `LLM_PROXY_TRANSPARENT_PROXY_ALLOWED_HOSTS`: 透明代理允许的目标主机白名单，逗号分隔（示例：`api.openai.com,*.anthropic.com`）
- `LLM_PROXY_CORS_ALLOW_ORIGINS`: CORS 允许来源，逗号分隔（默认 `*`，保持历史兼容）
- `LLM_PROXY_CORS_ALLOW_CREDENTIALS`: 是否允许 CORS 凭证（默认 `false`）

## 推理与流式增强

### StreamFake（非流式 → 流式管道转发）

对于客户端 `stream: false` 的请求，Centag 可在内部走完整流式链路（SSE → 聚合），等价于真实流式请求的通道能力，最终将完整文本返回给客户端。

- **行为**：`stream: false` 请求 → 内部强制走流式管道（后端/协议/流水线节点）→ 聚合为完整响应
- **非流式请求等价性**：通过 StreamFake 保证了 `stream: false` 与 `stream: true` 经过同样的后端/协议/流水线逻辑
- **环境变量控制**：

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `CENTAG_STREAM_FAKE` | `true` | 设为 `0`/`false`/`off` 可关闭 StreamFake |
| `CENTAG_STREAM_FAKE_MAX_BYTES` | `33554432` (32MB) | 聚合阶段最大缓冲字节数（防 OOM） |
| `CENTAG_PPROF` / `LLM_PROXY_PPROF_ENABLED` | `false`（`./start.sh debug` 默认 `true`） | 为 `true` 时在 `127.0.0.1:6060` 暴露 pprof（仅 loopback）；debug 可 `CENTAG_PPROF=false` 关闭 |

- **关闭效果**：`stream: false` 请求直接以真实非流式方式调用后端，不走流式管道聚合

### ThinkSplit（推理过程提取与分流）

ThinkSplit 是**始终启用的内置功能**（无需环境变量开关）。它从模型流式输出中提取 `reasoning_content`（或厂商等价推理块），与正文分流输出，兼容 Claude Code / Gemini CLI / Codex 的推理展示需求。

- **行为**：自动识别通道中的推理块（`thinking` / `reasoning`），分流到独立 channel
- **下游兼容**：通过 `X-Proxy-Thinking` 标记确保 Claude Code / Gemini CLI 等客户端正确展示推理过程
- **配置**：无需配置；推理分流逻辑在协议层和处理管道中透明执行

## 注意事项

1. **本地开发**：使用 `localhost`，无需额外配置
2. **远程部署**：设置 `HOST_IP` 为宿主机实际IP（不是 `127.0.0.1` 或 `localhost`）
3. **网络连通性**：确保容器能够访问宿主机上的各中间件服务
4. **防火墙设置**：确保各端口的防火墙规则允许访问
5. **端口映射**：如果使用Docker，记得将容器端口映射到主机端口
6. **优先级**：显式指定的环境变量（如 `POSTGRES_HOST`）优先级高于 `HOST_IP` 自动替换

## 示例：不同服务器的配置

### 本地开发环境

```bash
# 无需设置 HOST_IP，默认使用 localhost
./start.sh run
```

### 服务器A（IP: 192.168.1.5）

```bash
export HOST_IP=192.168.1.5
./start.sh restart
```

或在 Docker Compose 中：

```yaml
environment:
  - HOST_IP=192.168.1.5
```

### 服务器B（IP: 10.0.0.10）

```bash
export HOST_IP=10.0.0.10
./start.sh restart
```

## 迁移说明

如果您之前使用了 `USE_HOST_IP` 环境变量，现在已不再需要：

- **旧方式**：`USE_HOST_IP=true` + `HOST_IP=192.168.1.5`
- **新方式**：只需 `HOST_IP=192.168.1.5`

本地开发时不设置任何变量，远程部署时只设置 `HOST_IP` 即可。
