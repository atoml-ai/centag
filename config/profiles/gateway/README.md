# Profile: gateway — 个人全功能

**目标**：个人侧的**全功能** Centag（二进制与 team 插件集对齐）。默认内置 SQLite、单容器即可跑通；需要时再通过配置连接外部 PG / 向量 / Redis 等中间件。

> 与 **team** 的差别主要在**部署默认依赖与产品版本**（本 Profile 默认 SQLite + `CENTAG_EDITION=personal`），不在二进制裁剪。详见 [`docs/guide/dist-profiles.md`](../../../docs/guide/dist-profiles.md)。

## 发行包 ↔ 运行时

| 层 | 值 | 说明 |
|----|-----|------|
| Dist / Profile | `gateway` | 发行包名、compose 项目名 |
| `CENTAG_EDITION` | **`personal`** | Web/API/钩子语义；对话与计量落 SQLite |
| 勿用 | `team` | 会打开多租户/计费等团队面，且对话工厂偏 PG |

验证：

```bash
curl -s http://localhost:20060/api/v1/status | jq .edition
# 期望: "personal"
```

## 特点

- **全功能二进制**：含全部业务插件（路由、优化、审核、RAG、Mem0 等）与 sqlite/postgresql 驱动
- **产品版本 personal**：单用户语义；无 BillingHook / 租户管理面
- **默认零中间件**：`LLM_PROXY_DB_DRIVER=sqlite`，默认不启动 PostgreSQL / 向量等 stack 服务
- **可接外部中间件**：改配置即可连外部 PG / Redis / 向量，而无需换发行版
- **默认线上 API**：填入 API Key 即可跑通
- **可选本地模型**：`OLLAMA_ENABLED=true` 时通过 stack 共享 Ollama

## 包含服务

| 服务 | 必需 | 说明 |
|------|------|------|
| Centag + WebUI | 是 | 端口 20060，容器 `centag-gateway-app` |
| SQLite | 是 | 内置文件数据库，无需额外容器 |
| Ollama | 否 | 仅 `OLLAMA_ENABLED=true` 时 stack ensure `centag-ollama` |

## 配置

```bash
cd /path/to/centag

# 1. 复制环境变量模板
cp config/profiles/gateway/.env.example config/profiles/gateway/.env

# 2. 编辑 .env，填入至少一个外部供应商 API Key
vim config/profiles/gateway/.env

# 3. 一键启动（默认仅应用容器，不启动 Ollama）
./start.sh profile gateway up
```

## 后端配置方式

### 方式 A：外部供应商（默认，推荐）

`.env` 中保持默认并填入 Key：

```bash
OLLAMA_ENABLED=false
OPENAI_API_KEY=sk-xxx
# 或 DEEPSEEK_API_KEY / DASHSCOPE_API_KEY 等
```

此模式下 profile **仅启动应用容器**，不连接 `deploy/stack-network`，不拉取本地模型。

### 方式 B：本地 Ollama（可选）

`.env` 中设置：

```bash
OLLAMA_ENABLED=true
OLLAMA_HOST=http://centag-ollama:11434
```

启动后拉取模型（stack 容器）：

```bash
docker exec -it centag-ollama ollama pull llama3.1
```

或使用 reset 自动拉取（需 `OLLAMA_ENABLED=true`）：

```bash
./start.sh profile gateway reset
```

同时在 WebUI 或 `config/initdata/initial-backends.yaml` 中启用 `ollama-local` 后端。

## 启动后验证

```bash
# 查看服务状态（应只有 centag-gateway-app）
./start.sh profile gateway status

# 测试直连模式（默认 gpt-4o-mini）
curl http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-admin-api-key" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## 默认行为

- **产品版本**：`CENTAG_EDITION=personal`（compose 与 `.env.example` 已固定）
- **默认模式**：`transparent-proxy`（透明模式，不注入 system prompt；全发行版统一）
- **默认后端**：OpenAI `gpt-4o-mini`（需在 `.env` 配置 `OPENAI_API_KEY`）
- **数据库**：SQLite（`./storage/centag.db`）
- **对话 / Token 计量**：持久化到同一 SQLite（重启不丢；对比 minimal 进程内/文件临时）
- **stack 依赖**：无（`OLLAMA_ENABLED=false` 时）

## 与其他 Profile 的关系

| 能力 | gateway | cached | agent-memory |
|------|---------|--------|-------------|
| 数据库 | SQLite | PostgreSQL + pgvector | PostgreSQL + pgvector |
| stack 中间件 | 无（默认） | PG + 可选 Ollama | PG + Qdrant + Ollama + Mem0 |
| 初始化默认模式 | 透明 | 透明（可改 cache） | 透明（可改 mem0） |
| 本地模型 | 可选 | 可选 | 默认 |
| 内存占用 | ~200MB | ~500MB | ~1.5GB |

## 故障排查

### 启动或访问很慢

- **首次 `profile up --build`** 会构建镜像，Docker Desktop 下可能需数分钟，属正常。
- 日常启动应只起单容器；若仍慢，检查是否误开 `OLLAMA_ENABLED=true` 或 Docker 资源不足。
- API 本身通常毫秒级：`curl http://localhost:20060/health`。

### `docker logs` 看不到访问日志

默认日志写入 **文件** 且路径相对可执行文件（`/app/bin/logs`），与挂载卷 `/app/logs` 不一致时 `docker logs` 只有 entrypoint 输出。

gateway compose 已设置：

```bash
LLM_PROXY_LOG_PATH=/app/logs
LLM_PROXY_LOG_OUTPUT=both    # 同时写文件 + stdout
```

查看日志：

```bash
./start.sh profile gateway logs
docker exec centag-gateway-app tail -f /app/logs/centag.log
```

### 保存后端配置长时间无响应

常见原因：旧版保存会重写全部 `system_config`（十余次 SQLite 写），在 Docker 卷上易超过 WebUI 30s 超时。

- 升级后后端保存仅写 `admin_backends` 一行。
- 若仍慢，检查是否点了「测试连接」——会探测外部 API（默认最长约 120s）。
- 无 API Key 时保存 openai 后端本身应秒级完成；测试连接才会卡住。

## 中间件管理

gateway 默认 **无 stack 依赖**。仅在启用 Ollama 时：

```bash
# profile up 已自动 stack ensure（需 OLLAMA_ENABLED=true）
./start.sh stack ensure ollama
./start.sh stack status
```