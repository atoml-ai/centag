# Profile: agent-memory — 智能体记忆模式

> 注：内置已改为透明代理默认流水线。Mem0 / `business.mem0` 需通过外部业务插件按需接入。

**目标**：完整的 AI Agent 运行环境，提供长记忆、安全工具执行和本地模型支持。

## 特点

- **modular 部署**：`profile up` 自动 `stack ensure` PostgreSQL、Qdrant、Ollama、Mem0；应用层（Centag + Pi）独立 compose
- **中间件统一入口**：与 `./start.sh stack start|ensure|status` 共用 `deploy/stack/lib/stack.sh`
- **重量级但完整**：一次启动获得 Agent 所需的全部基础设施
- **legacy embedded**：`manifest.conf` 设 `STACK_MODE=embedded` 可回退全栈内嵌（`docker-compose.embedded.yaml`）

## 包含服务

| 服务 | 部署位置 | 说明 |
|------|---------|------|
| Centag + WebUI | Profile | 端口 20060，容器 `centag-agent-memory-app` |
| Pi Sandbox Agent | Profile | 工具执行沙盒 |
| Pi Go Client | Profile | Agent HTTP API，端口 8080 |
| PostgreSQL + pgvector | Stack | `centag-postgresql` |
| Qdrant | Stack | `centag-qdrant` |
| Mem0 Server | Stack | `centag-mem0`，端口 20061 |
| Ollama | Stack | `centag-ollama`，本地 LLM + Embedding |

## 配置

```bash
cd /path/to/centag

cp config/profiles/agent-memory/.env.example config/profiles/agent-memory/.env
vim config/profiles/agent-memory/.env  # 填 Mem0 密钥等

# modular：自动 stack ensure 中间件 + 启动应用层
./start.sh profile agent-memory up
```

## 首次启动准备

Mem0 和 Pi Sandbox 首次构建镜像可能需要几分钟：

```bash
# 提前确保 stack 依赖（可选）
./start.sh stack ensure postgresql qdrant ollama mem0

# 提前构建应用镜像（可选）
docker compose -f config/profiles/agent-memory/docker-compose.yaml build
```

## 启动后验证

```bash
# 1. 检查应用容器
./start.sh profile agent-memory status

# 2. 检查 stack 中间件
./start.sh stack status

# 3. 拉取 Ollama 模型（stack 容器）
docker exec -it centag-ollama ollama pull llama3.1
docker exec -it centag-ollama ollama pull bge-m3

# 或使用 reset 自动拉取
./start.sh profile agent-memory reset

# 4. 测试记忆注入
curl http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-admin-api-key" \
  -H "X-Proxy-Mode: mem0-memory" \
  -d '{
    "model": "llama3.1",
    "messages": [{"role": "user", "content": "我叫张三，请记住"}]
  }'
```

## 默认行为

- **初始化默认模式**：`transparent-proxy`（全发行版统一；可改 `LLM_PROXY_DEFAULT_MODE=mem0-memory`）
- **定制主流水线**：`mem0-memory`（自动注入长记忆上下文）
- **LLM provider**：Ollama 本地模型（llama3.1）
- **Embedding**：Ollama bge-m3
- **记忆存储**：Mem0 → PostgreSQL + Qdrant（stack）
- **工具执行**：Pi Sandbox 隔离环境（profile）

## 中间件管理

```bash
# 手动确保依赖（profile up 已自动调用）
./start.sh stack ensure postgresql qdrant ollama mem0

# 查看 stack 状态
./start.sh stack status
```

## 回退 embedded 模式

若需单 compose 内嵌全部服务（不与 stack 并行）：

```bash
# 编辑 manifest.conf
STACK_MODE=embedded

./start.sh profile agent-memory up
```

> embedded 与 stack 默认端口互斥，请勿同机并行运行 `./start.sh stack start all`。

## 资源需求

| 资源 | 需求 |
|------|------|
| CPU | 4 核以上 |
| 内存 | 4GB 以上 |
| 磁盘 | 10GB 以上 |
| GPU | 可选（Ollama 可用 GPU 加速） |