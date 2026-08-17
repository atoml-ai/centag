# Proxy Modes Documentation

## Overview

LLM Proxy supports **12 different proxy modes** to handle various routing, caching, and processing scenarios. Each mode is designed for specific use cases and can be controlled via request headers or configuration files.

> **🆕 New in 2026.05**: All proxy modes now support **Pipeline Template** execution! Enable via environment variables for declarative, configurable, and extensible proxy behavior.

## Table of Contents

- [Proxy Modes](#proxy-modes)
  - [Smart Scheduling Mode](#1-smart-scheduling-mode-default)
  - [Direct Backend Mode](#2-direct-backend-mode)
  - [Transparent Proxy Mode](#3-transparent-proxy-mode)
  - [Audit Mode](#4-audit-mode)
  - [Optimize Mode](#5-optimize-mode)
  - [Fallback Mode](#6-fallback-mode)
  - [Model Matching Mode](#7-model-matching-mode)
  - [Intent Classification Mode](#8-intent-classification-mode)
  - [Pipeline Mode](#9-pipeline-mode)
  - [Aggregator Mode](#10-aggregator-mode)
  - [Router Mode](#11-router-mode)
  - [Translate Mode](#12-translate-mode)
  - [RAG Knowledge Gateway Mode](#13-rag-knowledge-gateway-mode)
  - [Security Firewall Mode](#14-security-firewall-mode)
  - [Multilingual Customer Support Mode](#15-multilingual-customer-support-mode)
  - [Geo Routing Mode](#16-geo-routing-mode)
  - [CI/CD Webhook Trigger](#17-cicd-webhook-trigger)
- [Pipeline Template Migration](#pipeline-template-migration)
- [Streaming Behavior](#streaming-behavior)
- [Cache Control](#cache-control)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Examples](#examples)

## Proxy Modes

### 1. Smart Scheduling Mode (Default)

**Description**: Intelligently routes requests based on model compatibility, backend capabilities, and load balancing.

**When to use**:
- Default use case
- When you want automatic routing optimization
- When you have multiple backends with different capabilities
- When you need load balancing

**How it works**:
1. Analyzes the requested model
2. Matches model with the most suitable backend
3. Considers backend weights and priorities
4. Routes to the optimal backend

**Request Example**:

```bash
# Default behavior (no headers needed)
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# Explicitly specify smart scheduling
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: smart-scheduling" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**Response Headers**:
```
X-Proxy-Mode: smart-scheduling
X-Cache-Read: true
X-Cache-Write: true
X-Cache: HIT-EXACT
```

### 2. Direct Backend Mode (`#d`)

**Description**: Fixed egress via `transparent_forward` (`route_policy=fixed`) with `system_prompt_strategy: replace` (or legacy `inject_system_prompt: true`). Gateway `system_prompt` **replaces** client `system` messages.

**When to use**:
- Default chat / personal gateway with a hosted assistant persona
- You want Centag to shape replies (style, safety tone, structure)

**How it works**:
1. Resolve pipeline `direct-backend` (or `#d`)
2. `transparent_forward` to system default (or pinned) backend
3. Apply system strategy `replace` when gateway prompt is non-empty (see [prompt-strategy.md](./prompt-strategy.md))

**Shortcut**: `#d`  
**Template**: `config/initdata/pipeline-templates/common/transparent.yaml` (direct-backend 为别名，经兼容层映射)

**Request Example** (standard OpenAI client — only `base_url` / API key needed):

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: direct-backend" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

Optional: `X-Backend-ID` / `X-Backend-Name` still work for forcing a backend when supported by the dispatcher.

### 3. Transparent Mode (`#t` / `#tf`)

**Description**: Pass through the client request as much as possible. Uses `transparent_forward` with `system_prompt_strategy: passthrough` (or legacy `inject_system_prompt: false`). Client `messages` (including their system) are kept as-is.

**When to use**:
- Standard chat clients where you must not rewrite prompts
- Bring-your-own system prompt from the client

**How it works**:
1. Resolve `transparent-proxy` (`#t`) or `transparent-fast` (`#tf`) — same semantics
2. `transparent_forward` + `route_policy=match_model`, **no** system prompt injection
3. Optional: attach `user_prompt_ops` / `output_post_ops` nodes for inbound/outbound normalization (see [prompt-strategy.md](./prompt-strategy.md))

**Shortcuts**: `#t`, `#tf`  
**Template**: `config/initdata/pipeline-templates/common/transparent.yaml` (transparent-proxy 为别名，经兼容层映射)

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: transparent-proxy" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "system", "content": "Answer in one sentence."},
      {"role": "user", "content": "Hello"}
    ]
  }'
```

**Breaking change (v2.1)**: `#t` is no longer HTTP raw forward. Caching belongs to `cache-mode`. For fixed-egress jump board see **Jump Board** below.

### 3b. Jump Board Mode (`#j` / fixed-egress)

**Description**: Centag acts as a fixed-egress jump board via `builtin.transparent_forward` with `route_policy=fixed`. Uses the system default backend/model (or explicit `X-Backend-ID`); no cross-backend model matching; does not inject system prompt. (Formerly `raw-forward` / `#raw` — retired.)

**Shortcut**: `#j`  
**Template**: `config/initdata/pipeline-templates/common/transparent.yaml` (fixed-egress 为别名，经兼容层映射)

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: fixed-egress" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'
```

**Semantic Cache Configuration** (cache-mode / legacy examples):

Transparent proxy mode now supports semantic caching, allowing cache hits on semantically similar queries (not just exact matches).

**Recall backend vs hit strategies** (v0.3.3+; see [Cache-Guide.md](./Cache-Guide.md)):

| Layer | Config | Values |
|-------|--------|--------|
| Recall backend | `cache.backend` (global) / node `strategy` | `exact` (S1, default), `semantic` (S2), `external` (S3) |
| Hit strategies | `cache.hit_strategies` | e.g. `normalize`, `expand`, custom plugins |
| Stacking (legacy hybrid) | `cache.allow_backend_stacking` | default `false`; when `true`, exact miss → semantic |

Legacy `strategy: hybrid` maps to `backend=exact` + stacking (not the default).

**Example: Enable Semantic Cache**:

To enable semantic caching, set global `cache.backend: semantic` (or pipeline node strategy) and ensure embedding/vector store are ready:

```json
{
  "cache_read": {
    "config": {
      "custom_config": {
        "strategy": "semantic",
        "vector_storage_name": "default-vector",
        "embedding_model": "text-embedding-3-small",
        "semantic_threshold": 0.85,
        "semantic_top_k": 5
      }
    }
  }
}
```

**Semantic Cache Parameters**:
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `strategy` | string | `exact` | Node-level recall: `exact`/`semantic` (prefer global `cache.backend`) |
| `vector_storage_name` | string | `default-vector` | Vector storage name (for semantic) |
| `embedding_model` | string | `text-embedding-3-small` | Embedding model for vector generation |
| `semantic_threshold` | float | `0.85` | Similarity threshold (0-1, higher = stricter) |
| `semantic_top_k` | int | `5` | Number of top results to return |

**Environment Variables for pgvector**:
```bash
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
VECTOR_STORE_PROVIDER=pgvector
VECTOR_STORE_COLLECTION=cache_embeddings
VECTOR_STORE_DIMENSION=1536
```

For detailed configuration examples, see `../../archive/deprecated/examples/semantic-cache-config.json` and `../../archive/deprecated/docs/guide/cache-node-storage-config.md` (both archived).

### 10. Aggregator Mode

**Description**: Parallel multi-model generation with aggregated results. Multiple generators run concurrently, and their outputs are merged by an aggregator node.

**When to use**:
- When you need responses from multiple models simultaneously
- For ensemble approaches to improve answer quality
- When you want to compare outputs from different backends

**How it works**:
1. Parallel execution of N generator nodes
2. Each generator uses a different backend/model
3. Aggregator node merges all outputs
4. Returns unified response

**Request Example**:

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: aggregator-mode" \
  -d '{
    "messages": [{"role": "user", "content": "Explain quantum computing"}]
  }'
```

### 11. Router Mode

**Description**: Intent-aware routing with conditional branch execution. Routes requests to different processing paths based on content classification or keyword matching. **Note**: Intent Classification (`#c`) has been merged into Router Mode (`#r`).

**When to use**:
- When you need to route requests by intent or keywords
- For multi-domain applications with different processing pipelines
- When different question types need different backends

**How it works**:
1. Router node classifies the request
2. Selects the matching branch based on classification
3. Executes the corresponding generator node
4. Returns the branch output

**Routing Strategies** (`routing_strategy` in `custom_config`):

| 策略 | 名称 | 匹配方式 | 延迟成本 | 适用场景 |
|------|------|----------|----------|----------|
| `keyword_contains`（默认） | 关键词包含 | 子串包含（忽略大小写） | 零 | 关键词覆盖充分、延迟敏感 |
| `keyword_prefix` | 关键词前缀 | 前缀匹配 | 零 | 命令式输入（如 `/help`） |
| `ordered` | 有序规则 | 按 `route_rules` 顺序 | 零 | 优先级明确的复杂规则 |
| `regex_only` | 正则匹配 | 按 `route_rules` 正则 | 零 | 复杂模式（电话、URL） |
| `keyword_then_intent` | 关键字 + 轻量意图 | 先规则，未命中再 IntentResolver；可选小模型 | 通常零；开启 LLM 时 +1 | 推荐：关键字优先且需类别回落 |
| `llm_classify` | LLM 意图分类 | LLM 语义分类 | +1 次 LLM 调用 | 表达多样、关键词维护成本高 |

`keyword_then_intent` 示例配置：

```yaml
routing_strategy: keyword_then_intent
default_route: generator_default
routes:
  code: generator_code
  chat: generator_chat
intent:
  enable_fast_matcher: true
  enable_llm_classifier: false   # 默认关闭重型 LLM 分类
  confidence_threshold: 0.55
```

`llm_classify` 模式下，`routes` 的 key 是**类别名**（如 `code`）而非常规关键词（如 `python`）；LLM 调用失败时自动 fallback 到 `default_route`。

#### 能力槽：新增分类 + 配置模型

多分类流水线（`router-mode` / `education-scene` / `coding-agent` 等样板，以及用户自建带 router 的 Agent）遵循职责拆分：

| 能力 | 入口 | 说明 |
|------|------|------|
| 加/改分类（拓扑） | 画布 **「新增分类」** | 关键词 → 执行节点；写入 `routes` + `route_config` |
| 绑后端/模型 | **「配置模型」** 面板 | 不改路由；保存热加载 |

模板种子各分支默认使用 `{{system.default_backend}}` / `{{system.default_model}}`。声明见 `metadata.capability_slots`（迁移期可并存 `route_model_targets`）。

**用户自建 Agent 建议路径**：

1. 添加后端并设置系统默认。
2. 创建流水线 → 画布添加路由节点 → 多次 **新增分类**（或从样板起步再扩展）。
3. 保存后在概览 / 策略管理 / 编辑器顶栏打开 **配置模型**，按分类绑定或「按标签重新推荐」后确认保存。
4. 用关键词请求验证命中节点与模型。

**样板说明**：`router-mode`（`#r`）、教育（`#edu`）、编程多阶段（`#code`）仅作验证与起步；库内旧版单节点 `coding-agent` 不会自动改写，需从模板重建。

**验收**：

| 项 | 期望 |
|----|------|
| 首装 | 可不进画布完成已有槽位模型绑定 |
| 热更新 | 保存后下一请求使用新 backend/model |
| 跟随默认 | 勾选后该分支随系统默认变化 |
| 职责边界 | 「配置模型」内无 routes 编辑；加分类只在画布 |

**Request Example**:

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: router-mode" \
  -d '{
    "messages": [{"role": "user", "content": "Write a Python function"}]
  }'
```

### 12. Translate Mode

**Description**: Two-stage pipeline: generate a response, then translate it to a target language using a dedicated translator node.

**When to use**:
- When you need responses in a specific language
- For multilingual applications
- When the generator model has limited language support

**How it works**:
1. Generator node produces the response
2. Translator node translates the output
3. Returns the translated response

**Request Example**:

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: translate-mode" \
  -d '{
    "messages": [{"role": "user", "content": "Hello, how are you?"}]
  }'
```

### 13. RAG Knowledge Gateway Mode

**Description**: Cache-first retrieval-augmented generation. On cache miss, searches the `agentmemory` knowledge base, injects retrieved context into the generator prompt, and writes the synthesized answer back to cache.

**When to use**:
- Internal knowledge-base Q&A (HR policies, finance docs, runbooks)
- High-frequency repeated questions where cache saves cost
- Answers that should cite source document paths

**Profile requirement**: `cached` or `agent-memory` (PostgreSQL + pgvector recommended for semantic retrieval; SQLite falls back to keyword search).

**How it works**:
1. `cache_read` — return immediately on L1/L2 hit
2. On miss: `question_splitter` → `rag_retrieval` (namespace from `/backend:` or metadata) → `generator` → `answer_synthesizer` (citations enabled) → `cache_write` → `token_usage`

**Shortcut**: `#rag`

**Local E2E** (stack PostgreSQL + local daemon): `./scripts/profile-rag-cache-demo.sh` — validates L1 exact cache on repeated `#rag` queries; L2 semantic requires paraphrased second query + embedding backend.

**Request Example**:

```bash
# Shortcut (works in any OpenAI-compatible client)
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "#rag /backend:finance 年假有多少天？"}]
  }'

# Header mode
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: rag-mode" \
  -H "X-Backend-ID: finance" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "年假有多少天？"}]
  }'
```

**Per-request cache control** (`X-Cache-Read` / `X-Cache-Write`) is honored inside the DAG — same semantics as `#ch`.

**Cache storage**: default template uses `redis` (L1 exact) + `chromadb-main` (L2 semantic vector store name). The `cached` Profile overlays `config/profiles/cached/initdata/pipeline-templates/cache-pipeline.yaml` with PostgreSQL (`pg`) + pgvector. Enable the matching storage backend in WebUI **设置 → 存储** before expecting cache hits.

**Template**: `config/initdata/pipeline-templates/common/cache-pipeline.yaml` (`rag-mode`, shortcut `#rag`; `18-rag-mode` 为别名，经兼容层映射)

### 14. Security Firewall Mode

**Description**: Inbound safety audit → generate → quality audit → PII redaction. Uses `business.pii_redactor` for production PII masking.

**When to use**:
- Compliance gateways requiring content filtering
- Scenarios where responses must be scrubbed before leaving the perimeter

**Shortcut**: `#sec`

**Pipeline**: `security_check` → `generator` → `audit` → `pii_redact` → `token_usage`

**Template**: `config/initdata/pipeline-templates/21-security-mode.yaml`

### 15. Multilingual Customer Support Mode

**Description**: Cache-first customer support with Mem0 memory retrieval, answer generation, and translation to the target language.

**When to use**:
- Multilingual helpdesk / chatbot scenarios
- Repeated FAQ with per-user memory context

**Shortcut**: `#cs`

**Pipeline**: `cache_read` → [miss] `mem0_retrieve` → `generator` → `translator` → `cache_write` → `token_usage`

**Template**: `config/initdata/pipeline-templates/22-multilingual-support.yaml`

### 16. Geo Routing Mode

**Description**: Selects a backend using **rule-based routing** (`business.geo_router`): configured IP prefix rules, optional `X-Geo-Region` header, and `region_backends` map. This is **not** a GeoIP/maxmind database lookup.

**When to use**:
- Multi-region compliance (data residency) with explicit region → backend mapping
- Routing internal vs external traffic by subnet prefix

**Shortcut**: `#geo`

**Headers**: `X-Geo-Region` (optional override, e.g. `CN`, `US`, `EU`)

**Routing precedence**:
1. `ip_prefix_rules` (from `client_ip`)
2. `X-Geo-Region` / `metadata.geo_region`
3. `default_backend`

**Pipeline**: `geo_router` (`business.geo_router`) → `generator`

**Template**: `config/initdata/pipeline-templates/23-geo-routing-mode.yaml`

**Integration note**: Production routing quality depends on correct `region_backends` / `ip_prefix_rules` in the node config and request metadata (`client_ip`, `geo_region`). No offline GeoIP DB is bundled.

### 16b. Pi Agent Mode (`#pi` / `#agent`)

**Description**: `tasktype_detector` branches code-oriented tasks to `business.pi_agent` (Pi sandbox HTTP API) and general Q&A to Mem0 + LLM.

**Shortcut**: `#pi` or `#agent`

**Template**: `config/initdata/pipeline-templates/19-agent-mode.yaml`

**Availability**: Requires Pi Agent service reachable at `LLM_PROXY_PI_AGENT_BASE_URL` (default `http://localhost:20062`). When Pi is **not** running, the Pi branch fails or degrades per template `bypass_on_error` — treat as **unavailable / degraded**, not production-ready without the agent-memory stack.

### 17. CI/CD Webhook Trigger

**Description**: Trigger a registered pipeline from external CI/CD systems (GitHub Actions, GitLab CI, Jenkins, etc.) without going through `/v1/chat/completions`.

**Endpoint**: `POST /api/v1/webhooks/pipeline/:id`

**Authentication** (required — secure default):
- `X-Webhook-Secret` matching `WEBHOOK_SECRET` env (or handler constructor secret), **or**
- Bearer / API key on the protected `/api/v1` route

Requests with **no** `WEBHOOK_SECRET` configured **and** no authenticated caller are rejected with `401`.

**Request body**:

```json
{
  "content": "deploy finished on main",
  "metadata": {"repo": "centag", "branch": "main"}
}
```

**Response**: `{"success": true, "data": <PipelineOutput>}`

Metadata `webhook_trigger=true` is injected automatically.

## Cache Control

### Cache Read Control

Control whether to read from cache:

```bash
# Disable cache read
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Cache-Read: disable" \
  -d '{...}'

# Enable cache read
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Cache-Read: enable" \
  -d '{...}'
```

### Cache Write Control

Control whether to write responses to cache:

```bash
# Disable cache write
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Cache-Write: disable" \
  -d '{...}'

# Enable cache write
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Cache-Write: enable" \
  -d '{...}'
```

### Cache Status Headers

- `HIT-EXACT`: Exact match in cache
- `HIT-SEMANTIC`: Semantic similarity match
- `MISS`: No match in cache
- `BYPASS-READ-DISABLED`: Cache read was disabled

## Configuration

### Proxy Configuration

```yaml
proxy:
  enabled: true
  default_mode: "transparent-proxy"  # 初始化默认；可选 direct-backend / smart-scheduling / fixed-egress 等
  allow_header_override: true  # Allow overriding via X-Proxy-Mode header
```

### Cache Control Configuration

```yaml
cache_control:
  enabled: true
  default_read: true   # Default cache read behavior
  default_write: true  # Default cache write behavior
  default_qa_split: true  # Default QA split behavior
```

## API Reference

### Request Headers

| Header | Description | Values | Required |
|---------|-------------|---------|----------|
| `X-Proxy-Mode` | Proxy mode selection | `transparent-proxy`, `direct-backend`, `smart-scheduling`, `fixed-egress`, … | No（默认 `transparent-proxy`） |
| `X-Backend-ID` | Backend ID for direct/transparent/fixed-egress | Backend ID string | Optional |
| `X-Backend-Name` | Backend name for direct/transparent | Backend name string | Optional |
| `X-Target-URL` | Optional target override for transparent_forward | Valid HTTP/HTTPS URL | Optional (hostproxy / advanced) |
| `X-Cache-Read` | Cache read control | `enable`, `disable`, `true`, `false`, `1`, `0` | No (default: config) |
| `X-Cache-Write` | Cache write control | `enable`, `disable`, `true`, `false`, `1`, `0` | No (default: config) |

### Response Headers

| Header | Description |
|---------|-------------|
| `X-Proxy-Mode` | Proxy mode used for the request |
| `X-Cache-Read` | Cache read status (`true`/`false`) |
| `X-Cache-Write` | Cache write status (`true`/`false`) |
| `X-Cache` | Cache status (`HIT-EXACT`, `HIT-SEMANTIC`, `MISS`, `BYPASS-READ-DISABLED`) |

### API Endpoints

| Method | Endpoint | Description |
|--------|-----------|-------------|
| GET | `/api/v1/backends` | List all available backends |
| GET | `/v1/models` | List available models |
| POST | `/v1/chat/completions` | Chat completions endpoint |
| POST | `/api/v1/webhooks/pipeline/:id` | Trigger pipeline from CI/CD webhook |
| GET | `/health` | Health check |

## Examples

### Example 1: Smart Scheduling with Cache

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: smart-scheduling" \
  -H "X-Cache-Read: enable" \
  -H "X-Cache-Write: enable" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "stream": false
  }'
```

### Example 2: Direct Backend with Cache Disabled

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: direct-backend" \
  -H "X-Backend-ID: ollama-Ollama" \
  -H "X-Cache-Read: disable" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Tell me a joke"}
    ],
    "stream": false
  }'
```

### Example 3: Streaming with Cache Control

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: smart-scheduling" \
  -H "X-Cache-Write: disable" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Count to 10"}
    ],
    "stream": true
  }'
```

### Example 4: Transparent mode (no system prompt inject)

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: transparent-proxy" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "Hello"}
    ]
  }'
```

### Example 4b: Jump board / fixed-egress (`#j`)

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: fixed-egress" \
  -d '{
    "model": "custom-model",
    "messages": [
      {"role": "user", "content": "Hello"}
    ]
  }'
```

## Streaming Behavior

> **新行为 (2026-06-05)**：流式与否**仅由客户端请求决定**，与流水线模板/模式配置无关。
> 详见 [`docs/exec-plans/active/2026-06-05-pipeline-stream-decoupling.md`](../exec-plans/active/2026-06-05-pipeline-stream-decoupling.md)。

### 决策模型

```
客户端请求 req.Stream
        │
        ├─ false ──→ 单 chunk 完整响应
        │
        └─ true  ──→ 代理层 StreamAdapter 分块响应
                          │
                          └─ 默认分块大小: 16 chars（按空白字符边界优先切分）
```

### 关键约束

| 维度 | 说明 |
|------|------|
| 决策来源 | **唯一** 来自 `req.Stream`（OpenAI 兼容 body 字段或 `X-Stream` 头） |
| 流水线配置 | `global_config.stream_mode` 已**废弃移除**（不再读取） |
| 节点间数据传递 | 全部走非流式 `Execute()`，节点间统一 `NodeOutput` |
| 优化/翻译等多层 pipeline | 同样按 `req.Stream=true` 输出流式 chunk（最后节点是 processor 时也支持） |

### 示例

**非流式**：

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: optimize-mode" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'
# → 单 JSON 响应，含优化后的完整内容
```

**流式（即使是 optimize-mode / translate-mode）**：

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: optimize-mode" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
# → SSE 流式响应（Content-Type: text/event-stream）
# → 每个 chunk 来自 StreamAdapter 的分块（默认 16 字符）
```

### 性能说明

所有 pipeline（包括 `smart-scheduling`）的首 token 延迟都会受 StreamAdapter 适配影响（需等待 LLM 完整生成后再分块）。
后续可在 generator 节点内部保留一个**可选的底层流式透传路径**（不暴露到节点间接口），用于延迟敏感场景。

## Pipeline Template Migration

### Overview

All 12 proxy modes now support **Pipeline Template** execution, providing declarative configuration and extensible processing workflows.

### Enabling Pipeline Templates

Set environment variables to enable pipeline templates for specific modes:

```bash
# Enable audit mode pipeline template
export LLM_PROXY_MODE_A_TEMPLATE_ENABLED=true

# Enable optimize mode pipeline template
export LLM_PROXY_MODE_O_TEMPLATE_ENABLED=true

# Enable other modes
export LLM_PROXY_MODE_D_TEMPLATE_ENABLED=true   # Direct backend
export LLM_PROXY_MODE_T_TEMPLATE_ENABLED=true   # Transparent proxy
# LLM_PROXY_MODE_F_TEMPLATE_ENABLED 已废弃：fallback-mode 模板不再预置
export LLM_PROXY_MODE_M_TEMPLATE_ENABLED=true   # Model matching
export LLM_PROXY_MODE_C_TEMPLATE_ENABLED=true   # Intent classification
export LLM_PROXY_MODE_P_TEMPLATE_ENABLED=true   # Pipeline mode
```

### Available Pipeline Templates

| Mode | Template File | Description |
|------|--------------|-------------|
| `#ag` (Aggregator) | `00-aggregator-mode.json` | Multi-generator parallel → aggregator merge |
| `#a` (Audit) | `01-audit-mode.json` | Generator + business.reviewer |
| `#o` (Optimize) | `07-optimize-mode.json` | Generator + business.optimizer |
| `#d` (Direct) | `03-direct-backend.json` | Simple single-node pipeline |
| `#t` (Transparent) | `14-transparent-proxy.json` | Remote node proxy |
| `#f` (Fallback) | —（不再预置） | Use `#d`/`#t`/`#j` built-in billing + FallbackGroup |
| `#m` (Model Matching) | `06-model-matching.json` | Pattern-based backend routing |
| `#c` (Intent Classification) | *(merged into router-mode)* | Now equivalent to `#r` |
| `#r` (Router) | `11-router-mode.json` | Router → N×generator (includes intent classification) |
| `#l` (Translate) | `13-translate-mode.json` | Generator → business.translator |
| `#p` (Pipeline) | `10-pipeline-mode.json` | Generic multi-stage pipeline |
| `#s` (Smart Scheduling) | `12-smart-scheduling.json` | Router-based intelligent scheduling |
| `#mem0` (Mem0 Memory) | `17-mem0-memory.json` | Generator → business.mem0 storage |
| `#rag` (RAG Gateway) | `cache-pipeline.yaml` | cache_read → rag_retrieval → generator → cache_write |

### Dynamic Configuration via Headers

When using pipeline templates, you can dynamically configure backends and models:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: #a" \
  -H "X-Executor-Backend-ID: openai-gpt4" \
  -H "X-Executor-Model: gpt-4" \
  -H "X-Auditor-Backend-ID: ollama-llama3" \
  -H "X-Auditor-Model: llama3:8b" \
  -d '{
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

### Backward Compatibility

- **Default behavior**: All modes use pipeline template execution via ModeDispatcher (Phase 3 completed 2026-05-21)
- **Legacy fallback removed**: The old direct handler paths (smartScheduleBackend, handleDirectBackendModePipeline, etc.) have been removed
- **ModeDispatcher is the sole entry point**: All requests now flow through `ModeDispatcher → PipelineEngine`
- **Pipeline templates required**: Each mode must have a corresponding pipeline template registered

### Migration Notes (2026-05-21)

**What changed:**
- `HandleChatCompletions` simplified from ~2800 lines to ~300 lines
- All legacy mode-specific handlers removed (handle*ModePipeline functions)
- `ModeDispatcher.Dispatch()` is now the only request routing path
- If `modeDispatcher` is nil or pipeline not found, returns 500 error (no fallback)

**Required pipeline templates:**
Ensure these pipeline templates are registered in `config/initdata/pipeline-templates/`:

| Mode | Required Template ID | File |
|------|---------------------|------|
| `smart-scheduling` | `smart-scheduling` | Built-in or custom |
| `direct-backend` | `direct-backend` | `15-direct-backend-v2.yaml` |
| `transparent-proxy` | `transparent-proxy` | `16-transparent-proxy-v2.yaml` |
| `audit-mode` | `audit-mode` | `13-audit-mode-v2.yaml` |
| `optimize-mode` | `optimize-mode` | `14-optimize-mode-v2.yaml` |
| `fallback` | —（不再预置） | — |
| `model-matching` | `model-matching` | `19-model-matching-v2.yaml` |
| `intent-classification` | `router-mode` | `20-intent-classification-v2.yaml` |

**Verification:**
```bash
# Check if pipelines are loaded
curl http://localhost:20060/api/v1/pipelines

# Test a specific mode
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "X-Proxy-Mode: direct-backend" \
  -H "X-Backend-ID: your-backend-id" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "test"}]}'
```

**Troubleshooting:**
- `ModeDispatcher is not initialized` → Pipeline engine failed to start, check logs
- `pipeline not found: X` → Missing template file, add to `config/initdata/pipeline-templates/`
- `no pipeline configured for mode: X` → Mode not in `defaultModeMappings`, check `internal/proxy/mode_dispatcher.go`

## Best Practices

1. **Use Transparent mode** as the default for standard OpenAI-compatible clients（不注入 system prompt）
2. **Use Direct Backend** when the gateway should inject a managed system prompt / persona
3. **Use Jump Board (`#j` / fixed-egress)** when you need fixed egress without cross-backend model matching
4. **Enable Cache** for frequently repeated queries（`cache-mode`，勿与透明模式混淆）
5. **Disable Cache Write** for unique or sensitive requests
6. **Monitor Cache Hit Rates** to optimize performance
7. **Consider Pipeline Templates** for complex multi-stage processing workflows

## Troubleshooting

### Backend Not Found

If you see "backend with id X not found" in logs:
- Check the backend ID using `GET /api/v1/backends`
- Ensure the backend is enabled
- Verify the backend configuration

### Cache Not Working

If cache is not being used:
- Check if cache is enabled in config
- Verify `X-Cache-Read` header is not set to `disable`
- Check cache storage configuration (Redis, ChromaDB, etc.)

### Jump Board / Fixed Egress Not Working

If `#j` / `fixed-egress` fails:
- Ensure a system default backend/model is configured（或显式 `X-Backend-ID`）
- Confirm the `fixed-egress` pipeline template is loaded
- For model matching / client-model routing use `transparent-proxy` instead

## Related Documentation

- [Main README](../README.md)
- [API Documentation](API_DOCUMENTATION.md)
- [Configuration Guide](CONFIGURATION_GUIDE.md)
- [Cache Documentation](CACHE_DOCUMENTATION.md)
