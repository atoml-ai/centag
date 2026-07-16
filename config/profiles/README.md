# Centag Deployment Profiles（部署模式）

Centag 功能强大但配置复杂。Deployment Profiles 将常见使用场景归纳为 **3 个一键部署模式**，让你根据目标场景选择即可，无需从零配置。

> **入门必读**：[Profile 入门手册](../docs/guide/profile-getting-started.md) — 三种模式的策略、启动依赖、中间件连接、默认流水线与 initdata 说明。

---

## 快速选择

| 你的场景 | 推荐 Profile | 一句话说明 | 内存需求 |
|---------|-------------|-----------|---------|
| 想快速跑起来，用线上 API | [`gateway`](gateway/) | 个人全功能，默认 SQLite 单容器，可外接中间件 | ~200MB |
| 客服/知识库，重复问题多 | [`cached`](cached/) | 精确+语义缓存，PG + 可选 Ollama，无 Redis/ChromaDB | ~500MB |
| 开发 AI Agent，需要长记忆 | [`agent-memory`](agent-memory/) | Mem0 + Pi + stack 中间件（modular） | ~2GB |

---

## 前置要求

- Docker & Docker Compose
- 磁盘空间：至少 5GB 空闲（agent-memory 模式需要 10GB）
- 如需外部供应商，准备一个 API Key

---

## 与 Stack 中间件的关系

三种 Profile **不是同一套部署的变体**，而是三层能力递增：

| 维度 | gateway | cached | agent-memory |
|------|---------|--------|--------------|
| **定位** | 个人全功能 | 缓存加速 | Agent 全栈 |
| **应用数据库** | SQLite（容器内文件） | stack PostgreSQL | stack PostgreSQL |
| **STACK_DEPS** | 无（默认）/ `ollama`（可选） | `postgresql ollama` | `postgresql qdrant ollama mem0` |
| **应用容器** | 仅 Centag | 仅 Centag | Centag + Pi |
| **stack .env 进容器** | **否**（避免 PG 污染） | 是 | 是 |
| **stack-network** | 仅 `OLLAMA_ENABLED=true` | 始终 | 仅 centag 服务 |

**两层环境变量（易混淆，需分清）**：

1. **宿主机 orchestration**（`./start.sh profile up`）：`load_profile_env` 链加载 `stack/.env` → `profile/.env` → `config/secrets/.env`，供 `stack ensure` 使用。
2. **容器 runtime**（compose `env_file`）：各 Profile 的 `docker-compose.yaml` 自行定义；**gateway  deliberately 不包含 `deploy/stack/.env`**。

entrypoint 按 `LLM_PROXY_DB_DRIVER` 决定是否等待 PostgreSQL：`sqlite` 不等待，`postgresql`/`auto` 等待 `centag-postgresql`。

- **modular（默认）**：`profile up` 先 `stack ensure` 再启应用层。
- **中间件统一入口**：`./start.sh stack …` 与 profile 共用 `deploy/stack/lib/stack.sh`。
- **embedded 回退**：仅 `agent-memory` 支持 `STACK_MODE=embedded`。

## 快速开始

```bash
# 1. 进入项目根目录
cd /path/to/centag

# 2. 查看可用的 Profile
./start.sh profile list

# 3. 一键启动（以 gateway 为例）
#    首次会自动从 .env.example 复制 .env，无需手动准备
./start.sh profile gateway up

# 4. 验证
open http://localhost:20060          # WebUI
curl http://localhost:20060/health   # API 健康检查
```

---

## 命令速查

### 基础操作

```bash
# 列出所有 Profile
./start.sh profile list

# 启动（首次自动复制 .env.example → .env）
./start.sh profile <name> up

# 停止
./start.sh profile <name> down

# 停止并清理所有数据卷（慎用）
./start.sh profile <name> down --volumes

# 查看日志
./start.sh profile <name> logs

# 只看某个服务的日志
./start.sh profile <name> logs <service>
# 例如：./start.sh profile agent-memory logs mem0

# 查看容器状态
./start.sh profile <name> status
```

### 高级参数

`up` / `down` / `logs` 后的参数会**原样透传给 `docker compose`**：

```bash
# 强制重新构建镜像
./start.sh profile agent-memory up --build

# 只启动特定服务
./start.sh profile agent-memory up ollama

# 后台启动+强制重建
./start.sh profile cached up -d --build

# 停止时一并删除匿名卷
./start.sh profile gateway down --volumes
```

---

## Profile 对比

| 维度 | gateway | cached | agent-memory |
|------|---------|--------|-------------|
| 外部 API Key | 可选（可用 Ollama） | 需要（LLM） | 可选（可用 Ollama） |
| 数据库 | SQLite（内置） | PostgreSQL + pgvector | PostgreSQL + pgvector |
| 缓存 | 无（可配置） | PG 精确+语义缓存 | 无（可配置） |
| 向量存储 | 不需要 | PG + pgvector | Qdrant |
| 记忆服务 | 不需要 | 不需要 | Mem0（stack） |
| Agent 服务 | 不需要 | 不需要 | Pi Sandbox（profile） |
| Ollama | 可选 stack | 可选 stack | 默认 stack |
| 默认模式 | transparent-proxy（全 Profile 统一初始化） | 可切 cache-mode | 可切 mem0-memory |
| 可切换模式 | 全部 | 全部 | 全部 |

---

## 渐进式体验路线

```
Level 0: ./start.sh profile gateway up
         └── 2 分钟内看到 Centag 跑起来，用本地模型聊天

Level 1: 准备一个 API Key，在 WebUI 中添加外部供应商
         └── 多后端切换，团队共享一个入口

Level 2: 切换到 cached，体验缓存命中
         └── 重复问题直接返回，成本降低 80%+

Level 3: 启动 agent-memory，接入 Mem0 长记忆
         └── Agent 能记住多轮对话的上下文

Level 4: 自定义流水线模板
         └── 发挥 Centag 完整插件能力
```

---

## 自定义 Profile

如果你需要组合多个 Profile 的能力（例如 gateway + cached），可以：

1. 复制某个 Profile 目录：`cp -r config/profiles/gateway config/profiles/my-custom`
2. 修改 `docker-compose.yaml` 添加所需服务
3. 修改 `config/initdata/` 中的流水线模板
4. 启动：`./start.sh profile my-custom up`

---

## 与现有启动方式的关系

| 方式 | 适用场景 |
|------|---------|
| `./start.sh profile <name> up` | **推荐**：场景化一键部署（本文档） |
| `./start.sh run be` | 本地二进制开发调试 |
| `./start.sh debug` | 本地前后端联调 |
| `./start.sh docker up` | 仅起 Centag 容器，依赖自管 |

---

## 相关文档

| 文档 | 说明 |
|------|------|
| [Profile 入门手册](../docs/guide/profile-getting-started.md) | 三种模式策略、依赖、流水线与 initdata |
| [Deployment Profiles 与 Stack](../docs/guide/deployment-profiles-and-stack.md) | 分层架构与 stack 协作 |
| [config/profiles/TESTING.md](TESTING.md) | 验证手册 |

---

## 故障排查

各 Profile 目录下均有 `README.md`，包含该模式的详细配置和排障说明。

通用排查：

```bash
# 检查容器状态
docker compose -f config/profiles/<name>/docker-compose.yaml ps

# 查看详细日志
docker compose -f config/profiles/<name>/docker-compose.yaml logs -f centag

# 验证配置合成
docker compose -f config/profiles/<name>/docker-compose.yaml config
```
