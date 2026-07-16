# 流水线节点插件标准

Centag 流水线节点插件标准用于把流水线从固定内置策略扩展为开放节点生态。旧模板仍可使用 `type` 字段；新模板建议使用 `kind` 和 `implementation` 明确声明节点语义与具体实现。

## 核心设计思想

### 能力抽象而非具体实现

插件架构的核心价值在于**能力抽象**——不关注具体实现（如某一厂商的记忆服务、云记忆），而是关注**能力维度**（如记忆、Token 优化、提示词处理）。这使得：

1. **灵活替换**：记忆系统可在云记忆、远程 HTTP 节点、本地向量等实现之间切换，只需修改配置
2. **能力复用**：同一能力（如记忆召回）可在多个场景复用
3. **生态开放**：第三方可以实现标准接口接入

### 能力插件类型

| 能力类型 | 说明 | 典型实现 |
|---------|------|---------|
| `memory` | 记忆能力（召回/存储/搜索） | 云记忆、远程记忆插件、本地向量 |
| `token` | Token 优化（压缩/截断/摘要） | 智能截断、语义摘要、预算控制 |
| `prompt` | 提示词处理（模板/优化/增强） | 模板引擎、动态优化、多语言适配 |
| `security` | 安全能力（审核/过滤/脱敏） | 内容审核、敏感词过滤、PII 脱敏 |
| `router` | 路由能力（分类/选择/负载） | 意图分类、模型选择、负载均衡 |
| `monitor` | 监控能力（成本/延迟/质量） | 成本追踪、延迟分析、质量评估 |

## 模板字段

节点配置支持以下开放字段：

```json
{
  "id": "redact-pii",
  "kind": "content.transform",
  "implementation": "https://plugin.example.com/centag/pii-redactor",
  "depends_on": ["generator"],
  "timeout": 30,
  "inputs": {
    "content": "node.generator.content"
  },
  "config": {
    "custom_config": {
      "mode": "strict"
    }
  },
  "permissions": ["network.outbound"]
}
```

- `type`：兼容旧格式，内置类型会自动映射为 `builtin.generator`、`builtin.processor` 等实现。
- `kind`：节点语义分类，例如 `llm.generate`、`content.transform`、`quality.review`、`route.decide`、`aggregate.merge`、`flow.parallel`、`memory.recall`、`token.optimize`。
- `implementation`：节点实现引用。内置实现使用 `builtin.<type>`，远程实现使用 HTTP/HTTPS base URL。
- `inputs`：把上游输出或上下文路径绑定到当前节点输入。`content` 会覆盖主输入，其它键写入 `input.metadata`。
- `permissions`：声明插件需要的能力，当前用于发现与审计，后续可接入强制授权。

## 能力插件配置示例

### 记忆能力配置

```json
{
  "name": "智能客服代理（记忆 + Token优化）",
  "schema_version": "centag.pipeline/v1alpha1",
  
  "capabilities": {
    "memory": {
      "implementation": "builtin.memory",
      "config": {
        "top_k": 5,
        "threshold": 0.4,
        "token_budget": 1500
      }
    },
    "token_optimizer": {
      "implementation": "builtin.semantic_summarizer",
      "config": {
        "model": "qwen/qwen3.5-plus",
        "max_history": 10
      }
    }
  },
  
  "nodes": [
    {
      "id": "recall-memory",
      "kind": "memory.query",
      "implementation": "builtin.memory",
      "inputs": {
        "query": "input.content"
      },
      "outputs": {
        "context": "memory.context"
      }
    },
    {
      "id": "generator",
      "kind": "llm.generate",
      "implementation": "builtin.generator",
      "depends_on": ["recall-memory"],
      "config": {
        "backend": "ppinfra",
        "model": "qwen/qwen3.5-plus",
        "system_prompt": "{{memory.context}}"
      }
    },
    {
      "id": "capture-memory",
      "kind": "memory.capture",
      "implementation": "https://memory-plugin.example.com",
      "depends_on": ["generator"],
      "inputs": {
        "messages": "input.messages",
        "response": "node.generator.content"
      },
      "async": true
    }
  ]
}
```

### 多能力组合配置

```json
{
  "name": "安全增强翻译代理",
  "capabilities": {
    "security": {
      "implementation": "builtin.content_moderator",
      "config": {
        "level": "strict",
        "categories": ["pii", "sensitive", "harmful"]
      }
    },
    "token": {
      "implementation": "builtin.smart_truncation",
      "config": {
        "max_tokens": 4000,
        "preserve_recent": 5
      }
    },
    "prompt": {
      "implementation": "https://prompt-service.example.com",
      "config": {
        "enhancement": "translation_professional"
      }
    }
  },
  "nodes": [
    {
      "id": "security-check",
      "kind": "security.scan",
      "inputs": { "content": "input.content" }
    },
    {
      "id": "token-optimize",
      "kind": "token.optimize",
      "condition": "input.token_count > 4000",
      "inputs": { "messages": "input.messages" }
    },
    {
      "id": "prompt-enhance",
      "kind": "prompt.enhance",
      "inputs": { "template": "input.prompt" }
    },
    {
      "id": "generator",
      "kind": "llm.generate",
      "depends_on": ["security-check", "token-optimize", "prompt-enhance"]
    }
  ]
}
```

## 内置实现

当前内置节点都会注册为可发现插件：

### LLM 核心节点
- `builtin.generator`：调用 LLM 后端生成内容。
- `builtin.processor`：优化、翻译、摘要等内容转换。
- `builtin.reviewer`：审核上游回答并输出评分。

### 流程控制节点
- `builtin.router`：按规则选择下游路径。
- `builtin.aggregator`：合并、摘要、投票或择优多个上游结果。
- `builtin.parallel`：保留并行分发语义。

### 能力插件节点（新增）
- `builtin.memory`：内置记忆查询（走能力代理 / 云记忆等已注册实现）。
- `builtin.cloud_memory_recall`：从云记忆召回。
- `builtin.token_optimizer`：Token 优化（智能截断）。
- `builtin.semantic_summarizer`：语义摘要。
- `builtin.content_moderator`：内容审核。
- `builtin.pii_redactor`：PII 脱敏。

## 远程插件协议

远程插件的 `implementation` 是服务 base URL。服务至少实现：

- `GET /.well-known/centag-node-plugin.json`：返回 `NodePluginDescriptor`。
- `POST /validate`：可选，校验配置。
- `POST /execute`：执行节点并返回 `NodeExecutionResponse`。

**`POST /validate` 请求体：**

```json
{
  "schema_version": "centag.pipeline.node/v1alpha1",
  "config": {
    "backend": "ollama-local",
    "model": "qwen2.5:7b"
  }
}
```

**`POST /validate` 响应示例（成功）：**

```json
{
  "valid": true
}
```

**`POST /validate` 响应示例（失败——结构化错误）：**

```json
{
  "valid": false,
  "errors": [
    {
      "code": "MISSING_FIELD",
      "message": "field 'model' is required",
      "field": "config.model",
      "retryable": false,
      "details": {}
    }
  ]
}
```

**`POST /validate` 响应示例（失败——简单格式）：**

```json
{
  "valid": false,
  "code": "INVALID_CONFIG",
  "message": "backend 'unknown' not found",
  "retryable": false
}
```

`/execute` 响应示例：

```json
{
  "output": {
    "content": "处理后的内容",
    "metadata": {
      "plugin": "example"
    }
  }
}
```

## 管理 API

- `GET /api/v1/pipelines/node-plugins`：列出已注册节点插件及其 Schema。
- `POST /api/v1/pipelines/node-plugins/discover`：传入 `{ "base_url": "https://..." }`，发现并注册远程节点插件。
- `GET /api/v1/pipelines/capabilities`：列出所有可用能力类型及其实现。

## 兼容策略

旧模板无需修改即可运行。执行时，未声明 `implementation` 的旧节点会按 `type` 自动映射为对应内置实现。新模板可以逐步迁移到 `schema_version: centag.pipeline/v1alpha1`、`kind`、`implementation` 和 `inputs`。

---

# 能力插件架构设计与实现指南

## 架构设计原则

### 1. 能力抽象原则

```
┌─────────────────────────────────────────────────────────────────┐
│                    Centag 能力插件生态                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   记忆能力    │  │   Token能力   │  │  提示词能力   │          │
│  │  (Memory)    │  │  (Token)     │  │  (Prompt)    │          │
│  ├──────────────┤  ├──────────────┤  ├──────────────┤          │
│  │ • 记忆服务   │  │ • 压缩算法    │  │ • 模板引擎    │          │
│  │ • 云记忆     │  │ • 智能截断    │  │ • 动态优化    │          │
│  │ • 本地向量   │  │ • 语义摘要    │  │ • 上下文压缩  │          │
│  │ • 混合存储   │  │ • 预算控制    │  │ • 多语言适配  │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   安全能力    │  │   路由能力    │  │   监控能力    │          │
│  │  (Security)  │  │  (Router)    │  │ (Monitoring) │          │
│  ├──────────────┤  ├──────────────┤  ├──────────────┤          │
│  │ • 内容审核    │  │ • 意图分类    │  │ • 成本追踪    │          │
│  │ • 敏感词过滤  │  │ • 模型选择    │  │ • 延迟分析    │          │
│  │ • PII 脱敏   │  │ • 负载均衡    │  │ • 质量评估    │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 2. 关键设计原则

| 原则 | 说明 | 示例 |
|------|------|------|
| **能力抽象** | 不关注具体实现，关注能力维度 | 记忆能力可由云记忆、远程插件或本地向量提供 |
| **声明式配置** | 用户声明"我要记忆能力"，系统自动解析最佳实现 | `capabilities.memory.implementation` |
| **钩子挂载** | 插件挂载到标准钩子点 | `pre_generate`、`post_generate` |
| **条件触发** | 支持条件表达式 | `condition: "input.token_count > 4000"` |
| **异步执行** | 非关键路径可异步执行 | `async: true` |
| **远程发现** | 支持远程插件自动发现和注册 | 通过 `.well-known/centag-node-plugin.json` |

## 核心接口设计

### 1. 能力插件接口

```go
// internal/pipeline/capability_plugin.go

package pipeline

// CapabilityType 能力类型
type CapabilityType string

const
