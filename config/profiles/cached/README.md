# Profile: cached — 缓存加速模式

**目标**：通过精确缓存 + 语义缓存大幅降低 API 调用成本，适合高频问答场景。

## 特点

- **PostgreSQL + pgvector**：缓存存储和向量检索一体化（stack 中间件）
- **可选本地 Embedding**：stack Ollama 提供本地 embedding 模型，也可使用线上服务
- **精简依赖**：无 Redis/ChromaDB；应用层 modular，中间件由 stack ensure

## 包含服务

| 服务 | 必需 | 说明 |
|------|------|------|
| Centag + WebUI | 是 | 端口 20060，容器 `centag-cached-app` |
| PostgreSQL + pgvector | 是 | stack 中间件 `centag-postgresql` |
| Ollama | 否 | stack 中间件 `centag-ollama`，本地 embedding 时使用 |

## 配置

```bash
cd /path/to/centag

cp config/profiles/cached/.env.example config/profiles/cached/.env
vim config/profiles/cached/.env  # 填 API Key 和 embedding 配置

# modular：自动 stack ensure postgresql + ollama（按 OLLAMA_ENABLED）
./start.sh profile cached up
```

## Embedding 方式选择

### 方式 A：本地 Ollama Embedding（推荐，零额外费用）

`.env` 中设置：
```bash
OLLAMA_ENABLED=true
EMBEDDING_BACKEND=ollama-local
EMBEDDING_MODEL=bge-m3
PG_HOST=centag-postgresql
```

启动后拉取 embedding 模型（stack 容器）：
```bash
docker exec -it centag-ollama ollama pull bge-m3
```

或使用 reset 自动拉取：
```bash
./start.sh profile cached reset
```

### 方式 B：线上 Embedding 服务

`.env` 中设置：
```bash
OLLAMA_ENABLED=false
EMBEDDING_BACKEND=openai
EMBEDDING_MODEL=text-embedding-3-small
```

此模式下 profile up 仅 ensure PostgreSQL，不启动 Ollama。

## 启动后验证

```bash
# 首次请求（会写入缓存）
curl http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-admin-api-key" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "什么是机器学习"}]
  }'

# 第二次相同请求（命中精确缓存，响应更快）
# 第三次相似问题（命中语义缓存）
```

## 缓存策略

| 层级 | 触发条件 | 存储后端 |
|------|---------|---------|
| L1 精确缓存 | 请求哈希完全匹配 | PostgreSQL |
| L2 语义缓存 | 向量相似度 > threshold | PostgreSQL + pgvector |
| L3 LLM 调用 | 前两 miss | 外部供应商 |

## 调优参数

在 `config/profiles/cached/initdata/pipeline-templates/pipeline-default.yaml` 中修改：

```yaml
config:
  custom_config:
    semantic_threshold: 0.85   # 语义相似度阈值（越高越严格）
    ttl: 3600                  # 缓存有效期（秒）
    semantic_top_k: 5          # 语义检索返回数量
```

## 默认行为

- **初始化默认模式**：`transparent-proxy`（全发行版统一；可改 `LLM_PROXY_DEFAULT_MODE=cache-mode`）
- **定制主流水线**：`cache-mode`（生成 → 缓存写入）
- **只读模式**：`cache-hit`（仅查缓存，不调用 LLM）
- **存储后端**：PostgreSQL（精确 + 语义）

## 中间件管理

cached 使用 **modular** 模式：

```bash
# 手动确保依赖（profile up 已自动调用）
./start.sh stack ensure postgresql ollama

# 查看 stack 中间件状态
./start.sh stack status
```