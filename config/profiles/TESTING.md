# Centag Deployment Profile — 统一验证手册

本手册覆盖 **4 种部署模式** 的标准化验证流程：

| 模式 | 启动命令 | 数据库 | 特点 |
|------|---------|--------|------|
| **本地二进制** | `./start.sh run be` | SQLite/PostgreSQL | 开发调试 |
| **Docker 单容器** | `./start.sh docker up` | SQLite/PostgreSQL | 向后兼容 |
| **personal Profile** | `./start.sh profile personal up` | SQLite | modular，可选 stack Ollama |
| **cached Profile** | `./start.sh profile cached up` | stack PostgreSQL + pgvector | modular 缓存加速 |
| **agent-memory Profile** | `./start.sh profile agent-memory up` | stack PG/Qdrant/Mem0/Ollama + Pi | modular Agent |

> 三个 Profile 均为 **modular**：`profile up` 先 `stack ensure` 再启应用层。容器名见 [`docs/guide/deployment-profiles-and-stack.md`](../docs/guide/deployment-profiles-and-stack.md)。

> 注：本地二进制 与 Docker 单容器 共享同一套后端配置，验证步骤合并为「基础模式」。

---

## 前置条件

- `curl` 已安装
- `jq` 已安装（可选，用于 JSON 格式化）
- 各 Profile 已按对应 `README.md` 完成 `.env` 配置

---

## 第一节：通用验证（所有模式必做）

### 1.1 健康检查

```bash
curl -s http://localhost:20060/health | jq .
```

**期望结果**：
```json
{"status":"ok"}
```

### 1.2 模型列表

```bash
curl -s http://localhost:20060/v1/models \
  -H "Authorization: Bearer ${ADMIN_API_KEY:-test-key}" | jq '.data[].id'
```

**期望结果**：返回已配置的后端模型 ID（如 `llama3.1`、`gpt-4o-mini`）。

### 1.3 基础对话（direct-backend / smart-scheduling）

```bash
curl -s http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_API_KEY:-test-key}" \
  -d '{
    "model": "llama3.1",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 50
  }' | jq '.choices[0].message.content'
```

**期望结果**：返回非空文本响应，HTTP 200。

### 1.4 WebUI 可访问

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:20060/
```

**期望结果**：`200`

---

## 第二节：Personal Profile 专属验证

### 2.1 SQLite 数据文件

```bash
docker exec centag-personal-app ls -lh /app/storage/centag.db
```

**期望结果**：文件存在且大小 > 0（首次启动后）。

### 2.2 直连模式默认生效

```bash
curl -s http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_API_KEY:-test-key}" \
  -d '{
    "model": "llama3.1",
    "messages": [{"role": "user", "content": "2+2="}]
  }' | jq -r '.choices[0].message.content'
```

**期望结果**：返回 `4` 或相关数字回答，验证请求直达 Ollama。

### 2.3 Ollama 可选验证（若 `OLLAMA_ENABLED=true`）

```bash
docker exec centag-ollama ollama list
./start.sh stack status
```

**期望结果**：stack 中 `centag-ollama` 运行中，且列出已拉取模型（如 `llama3.1`）。

---

## 第三节：Cached Profile 专属验证

### 3.1 PostgreSQL 连接（stack 中间件）

```bash
docker exec centag-postgresql pg_isready -U postgres
```

**期望结果**：`accepting connections`

### 3.2 pgvector 扩展

```bash
docker exec centag-postgresql psql -U postgres -d centag -c "SELECT * FROM pg_extension WHERE extname = 'vector';"
```

**期望结果**：返回 1 行，含 `extname = vector`。

### 3.3 L1 精确缓存命中

```bash
# 首次请求（写入缓存）
curl -s http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_API_KEY:-test-key}" \
  -H "X-Proxy-Mode: cache-mode" \
  -d '{
    "model": "llama3.1",
    "messages": [{"role": "user", "content": "什么是机器学习"}]
  }' | jq -r '.choices[0].message.content'

# 第二次相同请求（命中精确缓存）
curl -s http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_API_KEY:-test-key}" \
  -H "X-Proxy-Mode: cache-mode" \
  -d '{
    "model": "llama3.1",
    "messages": [{"role": "user", "content": "什么是机器学习"}]
  }' | jq -r '.choices[0].message.content'
```

**期望结果**：第二次响应时间显著缩短；查看日志应出现 `cache hit` 或 `HIT-EXACT`。

### 3.4 L2 语义缓存命中

```bash
# 首次请求（已缓存）
# 语义相似请求
curl -s http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_API_KEY:-test-key}" \
  -H "X-Proxy-Mode: cache-mode" \
  -d '{
    "model": "llama3.1",
    "messages": [{"role": "user", "content": "解释一下机器学习"}]
  }' | jq -r '.choices[0].message.content'
```

**期望结果**：响应内容与首次请求相似，日志出现 `HIT-SEMANTIC`。

---

## 第四节：Agent-Memory Profile 专属验证

### 4.1 容器健康状态（modular：Profile + Stack）

```bash
./start.sh profile agent-memory status
./start.sh stack status
```

**期望结果**：

| 来源 | 容器名 | 端口（宿主机） |
|------|--------|---------------|
| Profile | `centag-agent-memory-app` | 20060 |
| Profile | `centag-agent-memory-pi-client` | 8080 |
| Profile | `pi-sandbox`（pi-agent） | — |
| Stack | `centag-postgresql` | 5432 |
| Stack | `centag-qdrant` | 6333 |
| Stack | `centag-mem0` | 20061 |
| Stack | `centag-ollama` | 21434 |

### 4.2 Ollama 模型就绪

```bash
docker exec centag-ollama ollama list
```

**期望结果**：包含 `llama3.1` 和 `bge-m3`。

### 4.3 Mem0 服务健康

```bash
curl -s http://localhost:20061/health
```

**期望结果**：HTTP 200，返回 `{"status":"ok"}` 或类似健康响应。

### 4.4 Mem0 记忆注入验证

```bash
# 1. 发送带记忆的请求
curl -s http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_API_KEY:-test-key}" \
  -H "X-Proxy-Mode: mem0-memory" \
  -d '{
    "model": "llama3.1",
    "messages": [{"role": "user", "content": "我叫张三，请记住"}]
  }' | jq -r '.choices[0].message.content'

# 2. 验证 Mem0 中已存储记忆
curl -s http://localhost:20061/memories?user_id=0 \
  -H "Authorization: Bearer ${MEM0_ADMIN_API_KEY}" | jq '.memories | length'
```

**期望结果**：步骤 2 返回 `>= 1`（表示记忆已写入）。

### 4.5 记忆上下文注入验证

```bash
# 第二轮对话（应自动注入记忆上下文）
curl -s http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_API_KEY:-test-key}" \
  -H "X-Proxy-Mode: mem0-memory" \
  -d '{
    "model": "llama3.1",
    "messages": [{"role": "user", "content": "我叫什么名字？"}]
  }' | jq -r '.choices[0].message.content'
```

**期望结果**：响应中包含「张三」或相关记忆信息。

### 4.6 Pi Sandbox 工具执行验证

```bash
# 1. 创建 Session
curl -s http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{"id":"test-session"}' | jq -r '.id'

# 2. 发送提示（触发工具执行）
curl -s http://localhost:8080/api/prompt \
  -H "Content-Type: application/json" \
  -d '{
    "session_id":"test-session",
    "prompt":"创建一个 hello.txt 文件，内容为 Hello World"
  }' | jq -r '.status'

# 3. 查看 Session 事件
curl -s http://localhost:8080/api/sessions/test-session/events | jq '.events[0].type'
```

**期望结果**：步骤 1 返回 `test-session`；步骤 2 返回非错误状态；步骤 3 返回事件类型（如 `tool_call`）。

### 4.7 数据库隔离验证

```bash
# Centag DB
docker exec centag-postgresql psql -U postgres -d centag -c "\dt" | grep -c "public"

# Mem0 DB（与 Centag 隔离）
docker exec centag-postgresql psql -U postgres -d mem0_app -c "\dt" | grep -c "public"
```

**期望结果**：两个数据库均存在各自的表，无冲突报错。

---

## 第五节：基础模式验证（本地二进制 / Docker 单容器）

### 5.1 本地二进制启动

```bash
./start.sh run be
# 等待日志出现 "Server started on :20060"

# 执行通用验证（第一节 1.1 ~ 1.4）
```

### 5.2 Docker 单容器启动

```bash
./start.sh docker up

# 执行通用验证（第一节 1.1 ~ 1.4）
```

---

## 第六节：一键验证脚本

### 6.1 快速验证（所有模式通用）

```bash
#!/bin/bash
set -e

PROXY_URL="http://localhost:20060"
ADMIN_KEY="${ADMIN_API_KEY:-test-key}"

echo "=== Centag 通用验证 ==="

echo -n "[1/4] Health Check ... "
curl -sf "${PROXY_URL}/health" > /dev/null && echo "OK" || { echo "FAIL"; exit 1; }

echo -n "[2/4] Model List ... "
curl -sf "${PROXY_URL}/v1/models" -H "Authorization: Bearer ${ADMIN_KEY}" > /dev/null && echo "OK" || echo "FAIL"

echo -n "[3/4] Chat Completion ... "
RESPONSE=$(curl -sf "${PROXY_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_KEY}" \
  -d '{"model":"llama3.1","messages":[{"role":"user","content":"2+2="}],"max_tokens":10}')
[ -n "$RESPONSE" ] && echo "OK" || { echo "FAIL"; exit 1; }

echo -n "[4/4] WebUI Access ... "
HTTP_CODE=$(curl -sf -o /dev/null -w "%{http_code}" "${PROXY_URL}/")
[ "$HTTP_CODE" = "200" ] && echo "OK" || echo "FAIL"

echo "=== 通用验证通过 ==="
```

保存为 `test-profile-quick.sh`，执行：

```bash
chmod +x test-profile-quick.sh && ./test-profile-quick.sh
```

### 6.2 Agent-Memory 深度验证

```bash
#!/bin/bash
set -e

PROXY_URL="http://localhost:20060"
MEM0_URL="http://localhost:20061"
PI_URL="http://localhost:8080"
ADMIN_KEY="${ADMIN_API_KEY:-test-key}"
MEM0_KEY="${MEM0_ADMIN_API_KEY}"

echo "=== Agent-Memory Profile 深度验证 ==="

# 容器检查（profile 3 + stack 4）
echo -n "[1/7] Profile + stack containers up ... "
PROFILE_COUNT=$(docker ps --format "{{.Names}}" | grep -cE "centag-agent-memory|pi-sandbox")
STACK_COUNT=$(docker ps --format "{{.Names}}" | grep -cE "centag-postgresql|centag-qdrant|centag-mem0|centag-ollama")
[ "$PROFILE_COUNT" -ge 2 ] && [ "$STACK_COUNT" -ge 4 ] && echo "OK (profile=$PROFILE_COUNT stack=$STACK_COUNT)" || { echo "FAIL (profile=$PROFILE_COUNT stack=$STACK_COUNT)"; exit 1; }

# Mem0 健康
echo -n "[2/7] Mem0 Health ... "
curl -sf "${MEM0_URL}/health" > /dev/null && echo "OK" || echo "FAIL"

# Ollama 模型
echo -n "[3/7] Ollama Models ... "
docker exec centag-ollama ollama list 2>/dev/null | grep -q "llama3.1" && echo "OK" || echo "FAIL (need: docker exec centag-ollama ollama pull llama3.1)"

# Mem0 记忆注入
echo -n "[4/7] Mem0 Memory Injection ... "
curl -sf "${PROXY_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_KEY}" \
  -H "X-Proxy-Mode: mem0-memory" \
  -d '{"model":"llama3.1","messages":[{"role":"user","content":"测试记忆"}]}' > /dev/null
sleep 2
MEM_COUNT=$(curl -sf "${MEM0_URL}/memories?user_id=0" -H "Authorization: Bearer ${MEM0_KEY}" | jq '.memories | length')
[ "${MEM_COUNT:-0}" -ge 1 ] && echo "OK ($MEM_COUNT memories)" || echo "FAIL"

# Pi Sandbox
echo -n "[5/7] Pi Sandbox Session ... "
curl -sf "${PI_URL}/api/sessions" -H "Content-Type: application/json" -d '{"id":"test"}' > /dev/null && echo "OK" || echo "FAIL"

# DB 隔离
echo -n "[6/7] DB Isolation ... "
docker exec centag-postgresql psql -U postgres -d centag -c "SELECT 1" > /dev/null 2>&1 && \
docker exec centag-postgresql psql -U postgres -d mem0_app -c "SELECT 1" > /dev/null 2>&1 && \
echo "OK" || echo "FAIL"

# 通用验证
echo -n "[7/7] Centag Chat ... "
curl -sf "${PROXY_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_KEY}" \
  -d '{"model":"llama3.1","messages":[{"role":"user","content":"Hello"}]}' > /dev/null && echo "OK" || echo "FAIL"

echo "=== 深度验证完成 ==="
```

---

## 第七节：故障排查速查表

| 现象 | 可能原因 | 排查命令 |
|------|---------|---------|
| `connection refused` | 服务未启动或端口错误 | `docker ps` / `lsof -i :20060` |
| `model not found` | 后端未配置或模型未拉取 | `docker exec <ollama> ollama list` |
| `pipeline not found` | 模板未注册 | `docker logs <centag> \| grep pipeline` |
| Mem0 记忆未写入 | `business.mem0` 未注册 / API Key 错误 | `docker logs <centag> \| grep -i mem0` |
| Pi Sandbox 无响应 | `pi-agent` 容器未就绪 | `docker logs centag-agent-memory-pi-client` |
| PostgreSQL 连接失败 | 环境变量名不匹配 | 检查 `.env` 中 `POSTGRES_HOST` 非 `PG_HOST` |
| `users` 表冲突 | Mem0 与 Centag 共用数据库 | 确认 `APP_DB_NAME=mem0_app` 且 `POSTGRES_DB=centag` |
| 缓存始终 MISS | embedding 未配置 / 相似度过高 | `docker logs <centag> \| grep -i semantic` |

---

## 第八节：CI/CD 集成

```yaml
# .github/workflows/profile-test.yml（示例片段）
- name: Personal Profile Test
  run: |
    ./start.sh profile personal up
    sleep 15
    bash config/profiles/TESTING.md | grep "通用验证通过"
    ./start.sh profile personal down

- name: Cached Profile Test
  run: |
    ./start.sh profile cached up
    sleep 15
    bash config/profiles/TESTING.md | grep "通用验证通过"
    ./start.sh profile cached down

- name: Agent-Memory Profile Test
  run: |
    ./start.sh profile agent-memory up
    sleep 30
    bash config/profiles/TESTING.md | grep "深度验证完成"
    ./start.sh profile agent-memory down
```

---

*本手册随 modular Profile 架构维护；架构说明见 [`docs/guide/deployment-profiles-and-stack.md`](../docs/guide/deployment-profiles-and-stack.md)。*
