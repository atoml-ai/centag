# Pi Agent 集成指南

## 概述

Pi Agent 是 Centag 支持的 Agent 工具之一，基于 [Pi coding agent](https://github.com/badlogic/pi-mono)（npm 包 `@mariozechner/pi-coding-agent`）。本指南描述 Pi Agent 在 Centag 中的架构、配置与使用方式。

## 架构

```
客户端 (OpenAI 兼容)
    │
    │ X-Proxy-Mode: pi-agent (#pi)
    ▼
┌─────────────────────────────────┐
│         Centag 主进程         │
│  ┌───────────────────────────┐  │
│  │   PiAgentExecutor         │  │
│  │   (internal/proxy/)       │  │
│  └──────────┬────────────────┘  │
│             │                   │
│  ┌──────────▼────────────────┐  │
│  │   Pi 客户端库              │  │
│  │   (internal/pi/)          │  │
│  └──────────┬────────────────┘  │
└─────────────┼───────────────────┘
              │ HTTP / JSONL
┌─────────────▼───────────────────┐
│      Pi Sandbox 服务            │
│   (deploy/stack:20062)       │
│  ┌───────────────────────────┐  │
│  │  Go HTTP Server           │  │
│  │  ├─ 会话管理              │  │
│  │  ├─ 安全策略              │  │
│  │  └─ 审计日志              │  │
│  └──────────┬────────────────┘  │
│             │ stdin/stdout      │
│  ┌──────────▼────────────────┐  │
│  │  Pi 容器 (Docker)         │  │
│  │  @mariozechner/pi-...     │  │
│  └───────────────────────────┘  │
└─────────────────────────────────┘
```

## 代理模式

### X-Proxy-Mode: pi-agent

通过设置 `X-Proxy-Mode: pi-agent`（快捷 `#pi`），将请求路由到 Pi Agent 处理。

**请求头**：

| 头部 | 必填 | 说明 |
|------|------|------|
| `X-Proxy-Mode` | 是 | `pi-agent` 或 `#pi` |
| `X-Pi-Session-Id` | 否 | 会话 ID，不传则自动创建 |
| `X-Pi-Model` | 否 | 覆盖默认模型（如 `claude-sonnet-4-20250514`） |
| `X-Pi-Workspace` | 否 | 工作目录路径 |

**请求体**：与 OpenAI Chat Completions 兼容，`messages` 中最后一条作为 prompt 发送。

**示例**：

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Proxy-Mode: pi-agent" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "messages": [
      {"role": "user", "content": "帮我分析这个项目的代码结构"}
    ]
  }'
```

## 配置

### 环境变量

在 `config/secrets/.env` 中配置：

```bash
# Pi Sandbox 连接
PI_SANDBOX_URL=http://localhost:20062
PI_SANDBOX_TIMEOUT=300

# 默认模型
PI_DEFAULT_PROVIDER=anthropic
PI_DEFAULT_MODEL=claude-sonnet-4-20250514

# 安全策略
PI_ALLOWED_PATHS=/workspace,/tmp
PI_BLOCKED_PATHS=/etc,/root,/home
PI_ALLOW_NETWORK=true

# 资源限制
PI_MAX_MEMORY=512m
PI_MAX_CPU=1.0
```

### 配置结构体

```go
// internal/pi/config.go
type PiConfig struct {
    SandboxURL    string        `json:"sandbox_url" env:"PI_SANDBOX_URL" default:"http://localhost:20062"`
    Timeout       time.Duration `json:"timeout" env:"PI_SANDBOX_TIMEOUT" default:"300s"`
    DefaultModel  string        `json:"default_model" env:"PI_DEFAULT_MODEL" default:"claude-sonnet-4-20250514"`
    AllowedPaths  []string      `json:"allowed_paths" env:"PI_ALLOWED_PATHS" default:"/workspace,/tmp"`
    BlockedPaths  []string      `json:"blocked_paths" env:"PI_BLOCKED_PATHS" default:"/etc,/root,/home"`
    AllowNetwork  bool          `json:"allow_network" env:"PI_ALLOW_NETWORK" default:"true"`
    MaxMemory     string        `json:"max_memory" env:"PI_MAX_MEMORY" default:"512m"`
    MaxCPU        float64       `json:"max_cpu" env:"PI_MAX_CPU" default:"1.0"`
}
```

## Pipeline 集成

Pi Agent 可作为 Pipeline 中的处理节点使用。

### 基础模板

```json
{
  "id": "pi-agent-basic",
  "name": "Pi Agent 基础处理",
  "nodes": [
    {
      "id": "input",
      "type": "input",
      "config": {}
    },
    {
      "id": "pi_process",
      "type": "business_plugin",
      "config": {
        "plugin_name": "pi_agent",
        "params": {
          "workspace": "{{input.workspace}}",
          "model": "{{input.model}}"
        }
      }
    },
    {
      "id": "output",
      "type": "output",
      "config": {}
    }
  ],
  "edges": [
    {"from": "input", "to": "pi_process"},
    {"from": "pi_process", "to": "output"}
  ]
}
```

### 多步工作流模板

```json
{
  "id": "pi-agent-workflow",
  "name": "Pi Agent 多步工作流",
  "nodes": [
    {"id": "input", "type": "input"},
    {"id": "analyze", "type": "business_plugin", "config": {"plugin_name": "pi_agent", "params": {"task": "analyze"}}},
    {"id": "plan", "type": "business_plugin", "config": {"plugin_name": "pi_agent", "params": {"task": "plan"}}},
    {"id": "execute", "type": "business_plugin", "config": {"plugin_name": "pi_agent", "params": {"task": "execute"}}},
    {"id": "review", "type": "business_plugin", "config": {"plugin_name": "pi_agent", "params": {"task": "review"}}},
    {"id": "output", "type": "output"}
  ],
  "edges": [
    {"from": "input", "to": "analyze"},
    {"from": "analyze", "to": "plan"},
    {"from": "plan", "to": "execute"},
    {"from": "execute", "to": "review"},
    {"from": "review", "to": "output"}
  ]
}
```

## 安全控制

Pi Agent 集成 Centag 后，安全策略通过两层控制：

### 1. Centag 层（新增）

- **API 认证**：所有请求需通过 Centag 的 API Key 认证
- **代理模式白名单**：可在配置中限制哪些 API Key 可使用 Pi Agent 模式
- **速率限制**：Pi Agent 模式可配置独立的速率限制

### 2. Pi Sandbox 层（已有）

- **文件路径白名单/黑名单**
- **命令黑名单**
- **网络访问控制**
- **工具调用验证**
- **审计日志**

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/v1/chat/completions` | OpenAI 兼容入口（`X-Proxy-Mode: pi-agent`） |
| `GET` | `/api/v1/pi/sessions` | 会话列表 |
| `POST` | `/api/v1/pi/sessions` | 创建会话 |
| `DELETE` | `/api/v1/pi/sessions/:id` | 删除会话 |
| `POST` | `/api/v1/pi/prompt` | 发送 prompt |
| `GET` | `/api/v1/pi/events` | SSE 事件流 |

## 监控与可观测

- **审计日志**：Pi Agent 的每次工具调用都会记录到 Centag 审计日志
- **执行历史**：Pipeline 中的 Pi 节点执行记录可通过 `GET /api/v1/pipelines/:id/executions` 查看
- **指标**：Pi Agent 请求数、延迟、错误率通过 Centag 指标端点暴露

## 故障排查

### Pi Sandbox 连接失败

```bash
# 检查服务状态
./deploy/stack/start.sh status pi-sandbox

# 查看日志
./deploy/stack/start.sh logs pi-sandbox

# 测试连接
curl http://localhost:20062/api/health
```

### 会话超时

- 检查 `PI_SANDBOX_TIMEOUT` 配置
- 检查 Docker 容器资源限制（`PI_MAX_MEMORY`、`PI_MAX_CPU`）

### 安全策略拒绝

- 查看审计日志确认被拒绝的工具调用
- 调整 `PI_ALLOWED_PATHS`、`PI_BLOCKED_PATHS` 配置

## 相关文档

- [代理模式详细说明](proxy-modes.md)
- [Pipeline 插件架构](../../archive/deprecated/docs/guide/PIPELINE-PLUGIN-ARCHITECTURE-GUIDE.md)（已归档）
- [模式行为矩阵](mode-behavior-matrix.md)
- [Processor 插件](processor-plugins.md)
- [Pi Sandbox 操作手册](../../deploy/stack/docs/pi-sandbox-guide.md)
