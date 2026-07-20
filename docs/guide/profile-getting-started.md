# Profile 入门手册

本文档是 Centag **三种 Deployment Profile** 的基础入门指南，说明各模式的策略、启动依赖、中间件连接，以及默认流水线与后端初始化配置。

适合第一次使用 `./start.sh profile <name> up` 的读者。架构与命令细节另见 [Deployment Profiles 与 Stack 中间件](./deployment-profiles-and-stack.md)。

---

## 1. 三种 Profile 不是「同一套部署的轻重量变体」

它们是**三层能力递增的独立场景**：

| 维度 | **personal** | **cached** | **agent-memory** |
|------|-------------|------------|------------------|
| **策略定位** | 最快跑通，线上 API 直连 | 高频问答降本，精确+语义缓存 | Agent 全栈，长记忆 + 工具沙盒 |
| **应用数据库** | SQLite（容器内文件） | PostgreSQL + pgvector | PostgreSQL（`auto`） |
| **STACK_DEPS** | 无（默认）/ `ollama`（可选） | `postgresql ollama` | `postgresql qdrant ollama mem0` |
| **Profile 容器** | 仅 Centag | 仅 Centag | Centag + Pi Agent + Pi Client |
| **stack .env 进容器** | **否** | 是 | 是 |
| **stack-network** | 仅 `OLLAMA_ENABLED=true` | 始终 | 仅 centag 服务 |
| **设计意图默认流水线** | `direct-backend` | `cache-mode` | `mem0-memory` |
| **典型内存** | ~200MB | ~500MB | ~2GB |

### 快速选型

| 你的场景 | 推荐 Profile | 启动命令 |
|---------|-------------|----------|
| 快速体验、只要线上大模型 | `personal` | `./start.sh profile personal up` |
| 客服/知识库、重复问题多 | `cached` | `./start.sh profile cached up` |
| 开发 AI Agent、需要长记忆 | `agent-memory` | `./start.sh profile agent-memory up` |

---

## 2. 启动编排（三种 Profile 共用）

```
./start.sh profile <name> up
    │
    ├─ load_profile_env（宿主机）
    │     deploy/stack/.env → config/profiles/<name>/.env → config/secrets/.env
    │
    ├─ profile_resolve_stack_deps（按 manifest + OLLAMA_ENABLED 过滤）
    │
    ├─ stack ensure（仅 modular 且有依赖时）
    │
    └─ profile_invoke_compose
          docker-compose.yaml
          + docker-compose.stack-network.yaml（按条件叠加）
```

**两层环境变量（易混淆）**：

| 层级 | 加载范围 | 用途 |
|------|----------|------|
| **宿主机 orchestration** | 三种 Profile 都链加载 `stack/.env` | `stack ensure`、过滤 Ollama 等 |
| **容器 runtime** | 各 Profile compose 的 `env_file` 自定 | 应用进程实际读到的变量 |

personal **故意不**把 `deploy/stack/.env` 注入容器，避免 `POSTGRES_*` 污染 SQLite 模式。这与 `LLM_PROXY_DB_DRIVER=sqlite` 是两条独立配置链。

`profile_resolve_stack_deps` 仅对 **ollama** 做运行时过滤：`OLLAMA_ENABLED=false` 时跳过；PostgreSQL / Qdrant / Mem0 在 manifest 中则始终 ensure。

---

## 3. personal — 个人全功能

### 3.1 策略

- **目标**：全功能二进制 + 单容器默认 SQLite，用线上大模型 API 快速跑通；需要时再外接 PG / 向量等。
- **原则**：默认零 stack 依赖；entrypoint 见 `sqlite` 驱动时不等待 PostgreSQL。
- **与 team**：插件集合对齐；差别在部署默认依赖。详见 [`dist-profiles.md`](dist-profiles.md)。

### 3.2 启动后依赖

| 层级 | 默认 | 可选（`OLLAMA_ENABLED=true`） |
|------|------|-------------------------------|
| Stack | **无** | `centag-ollama` |
| Profile | `centag-personal-app` | 同上 + `deploy/stack-network` |
| 数据库 | SQLite `./storage/centag.db` | — |
| Redis / 向量库 | 关闭 | — |

真源：`config/profiles/personal/manifest.conf`、`docker-compose.yaml`。

### 3.3 默认中间件连接

| 中间件 | 默认 | 说明 |
|--------|------|------|
| PostgreSQL | 不连接 | `POSTGRES_ENABLED=false` |
| Ollama | 不连接 | 无 stack-network |
| Mem0 / Qdrant / Redis | 不连接 | compose 显式关闭 |

### 3.4 initdata 与默认流水线

挂载路径：`config/profiles/personal/initdata/` → 容器 `/app/initdata-profile`。

**后端**（`initial-backends.yaml`，Profile 为唯一种子源）：

首启 **`backends: []`**（无预置 OpenAI 等占位）。请在 WebUI「添加 Provider」配置；供应商参考见 `config/profiles/_shared/initdata/backends-catalog.yaml`（非运行时 seed）。

**流水线**（`pipeline-templates/pipeline-default.yaml`，覆盖全局 `direct-backend`）：

| 项 | 值 |
|----|-----|
| 模板 ID | `direct-backend` |
| 代理模式 | `direct-backend`（快捷码 `#d`） |
| 节点 | 单节点 `generator` |
| 后端 / 模型 | 需先在 WebUI 添加后端后再指向具体模型 |
| 链路 | 请求 → 直连后端 → 返回（无缓存、无记忆） |

### 3.5 环境变量要点

```bash
# config/profiles/personal/.env
OLLAMA_ENABLED=false          # 默认：不启 stack
LLM_PROXY_DB_DRIVER=sqlite
# 后端在 WebUI 添加；若模板仍用 ${OPENAI_API_KEY} 占位，再按需填写
```

### 3.6 验证

```bash
./start.sh profile personal up
curl http://localhost:20060/health

curl http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-api-key>" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'
```

---

## 4. cached — 缓存加速

### 4.1 策略

- **目标**：重复/相似问题命中缓存，降低 LLM API 成本。
- **原则**：缓存存 stack PostgreSQL（pgvector）；无 Redis/ChromaDB；LLM 走线上 API，Embedding 默认走本地 Ollama。

### 4.2 启动后依赖

| 层级 | 默认（`OLLAMA_ENABLED=true`） | `OLLAMA_ENABLED=false` |
|------|------------------------------|------------------------|
| Stack | `centag-postgresql` + `centag-ollama` | 仅 `centag-postgresql` |
| Profile | `centag-cached-app` | 同上 |
| 网络 | 始终 `deploy/stack-network` | 同上 |

entrypoint：`LLM_PROXY_DB_DRIVER=postgresql` → 等待 `centag-postgresql`。

### 4.3 默认中间件连接

| 中间件 | 容器内地址 | 用途 |
|--------|------------|------|
| PostgreSQL | `centag-postgresql:5432` | 应用 DB + 缓存（`pg` 存储） |
| Ollama | `centag-ollama:11434` | Embedding（`bge-m3`） |
| Redis / Chroma | 不连接 | `REDIS_ENABLED=false` |

compose 注入：`deploy/stack/.env` + `config/profiles/cached/.env` + `config/secrets/.env`。

### 4.4 initdata 与默认流水线

挂载路径：`config/profiles/cached/initdata/`。

**后端**：Profile **没有** `initial-backends.yaml`，首轮 seed 为空（不再并入通用供应商全集）。流水线中 LLM 使用 `openai`，Embedding 使用 `ollama-local`——请在 WebUI 下拉添加并启用对应后端，或在 Profile 中提供自己的 `initial-backends.yaml`；`.env` 中配置 `OPENAI_API_KEY`。

**流水线**（覆盖全局 `cache-mode`）：

| 项 | 值 |
|----|-----|
| 模板 ID | `cache-mode` |
| 代理模式 | `cache-mode`（`#cm`） |
| 节点拓扑 | `cache_read → generator → cache_write` |

| 节点 | 配置 |
|------|------|
| `cache_read` | 存储 `pg`，策略 `exact_and_semantic`，阈值 `0.85` |
| `generator` | `openai` / `gpt-4o-mini`（缓存未命中时调 LLM） |
| `cache_write` | 存储 `pg`，embedding `ollama-local` / `bge-m3` |

### 4.5 环境变量要点

```bash
# config/profiles/cached/.env
LLM_PROXY_DB_DRIVER=postgresql
PG_HOST=centag-postgresql
OLLAMA_ENABLED=true              # 默认开，供 embedding
EMBEDDING_BACKEND=ollama-local
EMBEDDING_MODEL=bge-m3
OPENAI_API_KEY=sk-xxx            # LLM 必填
```

线上 Embedding 时可设 `OLLAMA_ENABLED=false`、`EMBEDDING_BACKEND=openai`。

### 4.6 验证

```bash
./start.sh profile cached up

# 需显式指定缓存模式（见第 7 节「默认模式说明」）
curl http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-api-key>" \
  -H "X-Proxy-Mode: cache-mode" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"什么是机器学习"}]}'
```

---

## 5. agent-memory — Agent 全栈

### 5.1 策略

- **目标**：多轮长记忆 + 本地 LLM/Embedding + Pi 工具沙盒。
- **原则**：中间件全走 stack；**Pi Sandbox 留在 Profile compose**（不在 stack）。

### 5.2 启动后依赖

| 层级 | 服务 |
|------|------|
| Stack（始终 ensure） | `centag-postgresql`、`centag-qdrant`、`centag-ollama`、`centag-mem0` |
| Profile | `centag-agent-memory-app`、`pi-sandbox`（pi-agent）、`centag-agent-memory-pi-client` |

依赖链：`pi-agent` → `pi-client` → `centag`。

### 5.3 默认中间件连接

仅 **centag** 接入 `deploy/stack-network`（Pi 服务在 profile 内网）：

| 中间件 | 容器内地址 | 宿主机 |
|--------|------------|--------|
| PostgreSQL | `centag-postgresql:5432` | `localhost:5432` |
| Qdrant | `centag-qdrant:6333` | stack 映射端口 |
| Ollama | `centag-ollama:11434` | stack 映射端口 |
| Mem0 | `centag-mem0:8000` | `localhost:20061` |
| Pi Sandbox | `pi-client:8080` | `localhost:8080` |

`docker-compose.stack-network.yaml` 为 centag 注入 `MEM0_HOST=centag-mem0`、`PG_HOST=centag-postgresql` 等。

entrypoint：`LLM_PROXY_DB_DRIVER=auto` → 等待 PostgreSQL。

### 5.4 initdata 与默认流水线

挂载路径：`config/profiles/agent-memory/initdata/`。

**后端**（`initial-backends.yaml`）：

| 后端 ID | 默认状态 | 说明 |
|---------|----------|------|
| `ollama-local` | **enabled** | 本地 LLM + Embedding，无需 API Key |
| `openai` | disabled | 填 Key 后可切换 |

**流水线**（覆盖全局 `mem0-memory`）：

| 项 | 值 |
|----|-----|
| 模板 ID | `mem0-memory` |
| 代理模式 | `mem0-memory`（`#mem0`） |
| 节点拓扑 | `mem0_retrieve → generator → mem0_storage` |

| 节点 | 配置 |
|------|------|
| `mem0_retrieve` | Mem0 search，`limit: 3` |
| `generator` | `ollama-local` / `llama3.1`，注入 `{{.mem0_retrieve_content}}` |
| `mem0_storage` | Mem0 store，embedding `ollama-local` / `bge-m3:latest` |

### 5.5 环境变量要点

```bash
# config/profiles/agent-memory/.env
OLLAMA_ENABLED=true
MEM0_JWT_SECRET=...
MEM0_ADMIN_API_KEY=...
LLM_PROVIDER=ollama
EMBEDDER_PROVIDER=ollama
```

Mem0 与 Centag 使用独立 DB 名（`MEM0_APP_DB_NAME=mem0_app`），防止表冲突。

### 5.6 验证

```bash
./start.sh profile agent-memory up
./start.sh stack status

curl http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-api-key>" \
  -H "X-Proxy-Mode: mem0-memory" \
  -d '{"model":"llama3.1","messages":[{"role":"user","content":"我叫张三，请记住"}]}'
```

**legacy 回退**：`manifest.conf` 设 `STACK_MODE=embedded` 可使用 `docker-compose.embedded.yaml`（单 compose 内嵌全栈，勿与默认 stack 端口并行）。

---

## 6. initdata 加载机制

Docker 中 `INITDATA_PATH=/app/initdata-profile`（挂载 `config/profiles/<name>/initdata`）。

```
config/profiles/<name>/initdata（发行版 / 客户种子，优先）
        ↓ 若无 initial-backends.yaml|json
镜像内 /app/config/initdata（仅作回退，如自定义 --initdata zip）
```

| 资源 | 规则 |
|------|------|
| `initial-backends.yaml` | **Profile 优先、不与全局并集**；无 Profile 文件时回退全局；无文件则空种子。供应商参考目录在 `config/profiles/_shared/initdata/backends-catalog.yaml`（非运行时 seed） |
| `pipeline-templates/*.yaml` | 按模板 ID 合并；各 Profile 可有 `pipeline-default.yaml` |

实现见 `core/pkg/bootstrap/initdata.go`、`backend_config_loader.go`、`pipeline_template_files_loader.go`。

---

## 7. 运行时「默认模式」说明

**策略（全发行版统一）**：首轮初始化 / 无显式配置时，系统默认流水线为 **`transparent-proxy`（透明模式，不注入 system prompt）**。

代码真源：`core/pkg/config.DefaultSystemPipelineID`、`DefaultProxyConfig()`；可用环境变量覆盖：

```
LLM_PROXY_DEFAULT_MODE=transparent-proxy   # 默认
# 可选：按 Profile 体验切到定制主模式
# LLM_PROXY_DEFAULT_MODE=cache-mode
# LLM_PROXY_DEFAULT_MODE=mem0-memory
# LLM_PROXY_DEFAULT_MODE=direct-backend
```

| Profile | 定制主流水线模板 | 无头请求默认（未改 env） |
|---------|------------------|--------------------------|
| personal / minimal / team | `transparent-proxy` | 透明模式 |
| cached | `cache-mode`（可选） | 仍为透明；可设 `LLM_PROXY_DEFAULT_MODE=cache-mode` |
| agent-memory | `mem0-memory`（可选） | 仍为透明；可设 `LLM_PROXY_DEFAULT_MODE=mem0-memory` |

> 已初始化的数据库不会自动改写 `proxy.default_mode`；清空库或改 WebUI「默认流水线」后生效。

---

## 8. 架构一览

```
personal（默认）
  [Centag] ──SQLite──┐
       │                │
       └──HTTP──► 线上 API（OpenAI 等）

cached（默认）
  [Centag] ──► [PG/pgvector] 缓存读写
       │              ▲
       ├──embedding──► [Ollama bge-m3]
       └──LLM miss───► 线上 API

agent-memory（默认）
  [Centag] ──► [Mem0] ──► [PG + Qdrant]
       │              ▲
       ├──LLM/Embed──► [Ollama]
       └──tools──────► [Pi Sandbox]
```

---

## 9. 命令速查

```bash
./start.sh profile list
./start.sh profile personal up
./start.sh profile cached up
./start.sh profile agent-memory up
./start.sh profile <name> status|logs|down|reset

./start.sh stack status    # 查看 stack 中间件
./start.sh stack ensure postgresql ollama   # 手动 ensure
```

---

## 10. personal 常见问题

### 启动或页面访问慢

- **首次 `profile up --build`** 构建镜像在 Docker Desktop 下可能需 5–15 分钟，属环境限制而非应用逻辑。
- 日常 `profile up`（无 `--build`）应只重建/启动单容器，秒级完成。
- HTTP API（`/health`）通常为毫秒级；若浏览器慢，检查是否为首次加载静态资源或 Docker 资源配额。

### `docker logs` 看不到访问日志

应用默认 `LLM_PROXY_LOG_OUTPUT=file`，且日志路径相对可执行文件（`/app/bin/logs`），与挂载卷 `/app/logs` 不一致时，`docker logs` 只有 entrypoint 输出。

personal compose 已配置：

```bash
LLM_PROXY_LOG_PATH=/app/logs
LLM_PROXY_LOG_OUTPUT=both
```

查看方式：

```bash
./start.sh profile personal logs
docker exec centag-personal-app tail -f /app/logs/centag.log
```

### 保存后端配置长时间无响应

- **旧行为**：每次保存重写全部 `system_config`（十余次 SQLite 写），在 Docker 卷上易超过 WebUI 30s 超时。
- **现行为**：仅写 `admin_backends` 一行（`SaveBackendsToDB`）。
- **注意**：点击「测试连接」会探测外部 API，默认最长约 120s；无 Key 时会一直等到超时。纯「保存」应秒级返回。

---

## 11. 相关文档

| 文档 | 说明 |
|------|------|
| [config/profiles/README.md](../../config/profiles/README.md) | Profile 选择与命令速查 |
| [deployment-profiles-and-stack.md](./deployment-profiles-and-stack.md) | 与 stack 的分层架构 |
| [config/profiles/TESTING.md](../../config/profiles/TESTING.md) | 验证手册 |
| [config/profiles/personal/README.md](../../config/profiles/personal/README.md) | personal 详细配置 |
| [config/profiles/cached/README.md](../../config/profiles/cached/README.md) | cached 详细配置 |
| [config/profiles/agent-memory/README.md](../../config/profiles/agent-memory/README.md) | agent-memory 详细配置 |
| [proxy-modes.md](./proxy-modes.md) | 代理模式与流水线说明 |