# Centag API 参考文档

> 完整的 REST API 接口文档，涵盖认证、配置、后端、存储、插件等所有模块。

---

## 目录

1. [认证 API](#1-认证-api)
2. [用户 API](#2-用户-api)
3. [配置 API](#3-配置-api)
4. [后端 API](#4-后端-api)
5. [存储 API](#5-存储-api)
6. [插件 API](#6-插件-api)
   - 6.1 [业务插件 BusinessPlugin API](#61-业务插件-businessplugin-api)
7. [缓存 API](#7-缓存-api)
8. [代理模式 API](#8-代理模式-api)
9. [健康检查 API](#9-健康检查-api)
10. [Clash 规则 API](#10-clash-规则-api)
11. [对话记录 API](#12-对话记录-api-v022)

---


## 1. 认证 API

### POST /api/auth/login
用户登录

**请求体**:
```json
{
  "username": "admin",
  "password": "password"
}
```

**响应**:
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600,
    "user": {
      "id": 1,
      "username": "admin",
      "role": "admin"
    }
  }
}
```

### POST /api/auth/refresh
刷新访问令牌

**请求体**:
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

### POST /api/auth/logout
用户登出

### GET /api/auth/me
获取当前用户信息

**响应**:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "username": "admin",
    "role": "admin",
    "email": "admin@example.com"
  }
}
```

---

## 2. 用户 API

### GET /api/v1/user/apikeys
获取用户的 API 密钥列表

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "Production Key",
      "prefix": "llmproxy_ab12cd",
      "created_at": "2026-05-01T10:00:00Z",
      "expires_at": "2027-05-01T10:00:00Z"
    }
  ]
}
```

### POST /api/v1/user/apikeys
创建新的 API 密钥

**请求体**:
```json
{
  "name": "Development Key",
  "expires_days": 365
}
```

**响应**:
```json
{
  "success": true,
  "data": {
    "id": 2,
    "name": "Development Key",
    "full_key": "llmproxy_xxxxxxxxxxxxxxxx",
    "prefix": "llmproxy_xxxxxx",
    "created_at": "2026-05-05T10:00:00Z",
    "expires_at": "2027-05-05T10:00:00Z"
  }
}
```

### GET /api/v1/user/apikeys/:id
获取指定 API 密钥详情

### PUT /api/v1/user/apikeys/:id
更新 API 密钥

**请求体**:
```json
{
  "name": "Updated Key Name"
}
```

### DELETE /api/v1/user/apikeys/:id
删除 API 密钥

---

## 3. 配置 API

### GET /api/v1/config
获取完整配置

**响应**:
```json
{
  "success": true,
  "data": {
    "server": {
      "port": 20060,
      "host": "0.0.0.0",
      "mode": "debug"
    },
    "log": {
      "level": "info",
      "format": "json"
    },
    "cache": {
      "enabled": true,
      "default_ttl": 3600
    },
    "backends": [...],
    "storages": [...]
  }
}
```

### PUT /api/v1/config
更新完整配置

**请求体**:
```json
{
  "server": {
    "port": 20060,
    "host": "0.0.0.0"
  },
  "log": {
    "level": "debug"
  }
}
```

### GET /api/v1/config/:module
获取指定模块配置

**示例**: `GET /api/v1/config/server`

### PUT /api/v1/config/:module
更新指定模块配置

---

## 4. 后端 API

### GET /api/v1/backends
获取所有后端配置

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "openai-1",
      "name": "OpenAI GPT-4",
      "type": "openai",
      "base_url": "https://api.openai.com",
      "api_key": "sk-...",
      "models": ["gpt-4", "gpt-4-turbo"],
      "enabled": true,
      "health_status": "healthy"
    }
  ]
}
```

### POST /api/v1/backends
创建后端配置

**请求体**:
```json
{
  "id": "ollama-local",
  "name": "Ollama Local",
  "type": "ollama",
  "base_url": "http://localhost:21434",
  "api_key": "",
  "models": ["llama2", "mistral"],
  "enabled": true
}
```

### GET /api/v1/backends/:id
获取指定后端配置

### PUT /api/v1/backends/:id
更新后端配置

### DELETE /api/v1/backends/:id
删除后端配置

### POST /api/v1/backends/:id/health
执行健康检查

**响应**:
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "response_time_ms": 45,
    "last_check": "2026-05-05T10:00:00Z"
  }
}
```

---

## 5. 存储 API

### GET /api/v1/storages
获取所有存储配置

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "redis-1",
      "name": "Redis Cache",
      "type": "redis",
      "addr": "localhost:6379",
      "db": 0,
      "enabled": true
    }
  ]
}
```

### POST /api/v1/storages
创建存储配置

### GET /api/v1/storages/:id
获取指定存储配置

### PUT /api/v1/storages/:id
更新存储配置

### DELETE /api/v1/storages/:id
删除存储配置

### POST /api/v1/storages/:id/test
测试存储连接

---

## 6. 插件 API

### GET /api/v1/plugins
获取所有插件

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "protocol/openai",
      "name": "OpenAI Protocol",
      "type": "protocol",
      "version": "1.0.0",
      "enabled": true,
      "status": "loaded"
    }
  ]
}
```

### GET /api/v1/plugins/:id
获取插件详情

### POST /api/v1/plugins/:id/enable
启用插件

### POST /api/v1/plugins/:id/disable
禁用插件

### DELETE /api/v1/plugins/:id
卸载插件

### GET /api/v1/plugins/:id/config
获取插件配置

### PUT /api/v1/plugins/:id/config
更新插件配置

### GET /api/v1/plugins/registry
获取插件注册表列表

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "custom-backend",
      "name": "Custom Backend",
      "version": "1.0.0",
      "description": "A custom backend plugin",
      "author": "user@example.com",
      "downloads": 100,
      "rating": 4.5
    }
  ]
}
```

### POST /api/v1/plugins/registry/:id/install
从注册表安装插件

### GET /api/v1/plugins/registry/:id/download
下载插件包（验证 checksum）

### 6.1 业务插件 BusinessPlugin API

业务插件（`BusinessPlugin`）是扩展自 `NodePlugin` 的流水线节点，面向特定业务场景（优化、审核、摘要、翻译等）。通过 `BusinessPluginRegistry` 提供注册和发现。

#### BusinessPlugin 接口

| 方法 | 返回 | 说明 |
|------|------|------|
| `Descriptor()` | `NodePluginDescriptor` | 返回插件描述（含 name/implementation/kind/version） |
| `ValidateConfig(config)` | `error` | 校验配置 |
| `Execute(ctx, req)` | `NodeExecutionResponse` | 执行插件逻辑 |
| `GetBusinessType()` | `string` | 返回业务类型标识（如 optimize / review / summarize / translate） |
| `GetDependencies()` | `[]string` | 返回依赖列表（如 ["llm.call"]） |
| `GetBusinessMetadata()` | `BusinessPluginMetadata` | 返回业务元数据 |

#### BusinessPluginRegistry 注册/发现 API（Go 程序化接口）

| 方法 | 说明 |
|------|------|
| `Register(plugin, registryFn)` | 注册业务插件到索引，同时通过回调注册到全局 NodeRegistry |
| `GetByImplementation(impl)` | 按 implementation 标识查询（如 "business.optimizer"） |
| `GetByBusinessType(businessType)` | 按业务类型查询插件列表（如 "optimize" → 返回 optimizer 插件） |
| `GetAllBusinessTypes()` | 返回所有已注册的业务类型标识列表 |
| `GetAll()` | 返回所有业务插件列表 |
| `Count()` | 返回已注册插件数量 |

#### 注册示例

```go
import (
    "centag/internal/pipeline"
    "centag/plugins/business/optimizer"
)

registry := pipeline.NewNodeRegistry()
pipeline.RegisterBuiltinNodes(registry)

bizRegistry := pipeline.NewBusinessPluginRegistry()
registry.SetBusinessRegistry(bizRegistry)

optimizer.Register(registry, bizRegistry)  // 同时注册到两个注册表
```

#### 发现示例（在 HTTP handler 中）

```go
func listBusinessPlugins(c *gin.Context) {
    bizRegistry := nodeRegistry.GetBusinessRegistry()
    plugins := bizRegistry.GetAll()
    types := bizRegistry.GetAllBusinessTypes()
    // 返回 JSON 响应
}
```

---

## 7. 缓存 API

### GET /api/v2/cache/config
获取缓存 V2 配置

### PUT /api/v2/cache/config
更新缓存 V2 配置

### GET /api/v2/cache/status
获取缓存状态

**响应**:
```json
{
  "success": true,
  "data": {
    "enabled": true,
    "hits": 1234,
    "misses": 567,
    "hit_rate": 68.5,
    "size_mb": 45.2
  }
}
```

### POST /api/v2/cache/stats/reset
重置缓存统计

---

## 8. 代理模式 API

### GET /api/v1/proxy/modes
获取所有代理模式

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "smart-scheduling",
      "name": "智能调度",
      "description": "根据负载自动选择后端",
      "enabled": true
    },
    {
      "id": "fallback",
      "name": "故障转移",
      "description": "主后端故障时自动切换",
      "enabled": true
    }
  ]
}
```

### GET /api/v1/proxy/modes/:id
获取代理模式详情

### PUT /api/v1/proxy/modes/:id
更新代理模式配置

---

## 9. 健康检查 API

### GET /api/v1/health
系统健康检查

**响应**:
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "version": "1.0.0",
    "uptime_seconds": 3600,
    "components": {
      "database": "healthy",
      "cache": "healthy",
      "backends": {
        "openai-1": "healthy",
        "ollama-local": "healthy"
      }
    }
  }
}
```

### GET /api/v1/health/ready
就绪检查（用于 Kubernetes）

### GET /api/v1/health/live
存活检查（用于 Kubernetes）

---

## 10. Pipeline API

### POST /api/v1/pipelines/router-mode/auto-build
路由模式自动配置

根据策略自动探测后端健康状态，为路由模式的各个节点推荐最优的后端和模型配置。

**请求体**:
```json
{
  "strategy": "fast",
  "dry_run": true,
  "pipeline_id": "router-mode",
  "probe_backends": false,
  "categories": ["code", "python", "java"]
}
```

**参数说明**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `strategy` | string | 否 | 配置策略，可选值：`fast`（快速匹配）、`balance`（均衡）、`cost`（成本优先）、`quality`（质量优先）、`latency`（延迟优先）。默认 `fast` |
| `dry_run` | bool | 否 | 是否仅预览不应用。默认 `true` |
| `pipeline_id` | string | 否 | 流水线 ID。默认 `router-mode` |
| `probe_backends` | bool | 否 | 是否探测后端健康状态（`fast` 策略无需探测）。默认 `false` |
| `categories` | array | 否 | 指定要配置的 category 列表。默认处理所有已配置的 category |

**策略说明**:

| 策略 | 响应时间 | 适用场景 | 说明 |
|------|----------|----------|------|
| `fast` | <100ms | 快速配置、简单场景 | 使用内置对照表，无需探测后端，毫秒级响应 |
| `balance` | 2-5s | 精确调优、复杂场景 | 综合考虑延迟、成功率、成本等因素 |
| `cost` | 2-5s | 成本敏感场景 | 优先选择免费或低成本后端 |
| `quality` | 2-5s | 质量优先场景 | 优先选择高质量后端（如 GPT-4、Claude） |
| `latency` | 2-5s | 延迟敏感场景 | 优先选择低延迟后端 |

**响应** (dry_run=true):
```json
{
  "success": true,
  "data": {
    "pipeline_id": "router-mode",
    "updates": [
      {
        "target_node": "chat-generator",
        "old_backend": "ollama-local",
        "old_model": "llama3",
        "new_backend": "bigmodel",
        "new_model": "GLM-4-flash",
        "reason": "简单对话任务，推荐使用免费云端后端降低成本",
        "categories": "chat,conversation,qeneral"
      },
      {
        "target_node": "code-generator",
        "old_backend": "ollama-local",
        "old_model": "llama3",
        "new_backend": "bigmodel",
        "new_model": "GLM-4-flash",
        "reason": "代码生成任务，推荐使用代码能力强的后端",
        "categories": "code,python,java,go"
      }
    ],
    "probe_results": null,
    "warnings": []
  }
}
```

**响应** (dry_run=false):
```json
{
  "success": true,
  "data": {
    "pipeline_id": "router-mode",
    "applied": true,
    "updates_count": 4
  }
}
```

---

## 12. 对话记录 API (v0.2.2)

> 统一 Conversation/Session 抽象。存储：minimal=文件，personal/gateway=SQLite，team=PostgreSQL。  
> 代理请求可通过 `X-Session-ID` 续写同一会话；响应回显该头。可选 `X-Conversation-Category`。

### GET /api/v1/conversations/sessions
列出会话（需 JWT）。Query：`category`、`limit`、`offset`、`since`、`until`（RFC3339）；team 下 admin 可用 `user_id`。

### GET /api/v1/conversations/sessions/:id
会话详情。team 非 admin 仅可访问自己的会话。

### GET /api/v1/conversations/sessions/:id/messages
会话消息分页。Query：`limit`、`offset`。

### GET /api/v1/conversations/categories
当前用户可见的 category 聚合列表。

等价路径亦挂在 `/api/v1/user/conversations/*`（同上）。

---

## 11. Clash 规则 API

### GET /api/v1/user/clash/rules
获取 Clash 规则列表

### POST /api/v1/user/clash/rules
创建 Clash 规则

### GET /api/v1/user/clash/rules/:id
获取指定规则

### PUT /api/v1/user/clash/rules/:id
更新规则

### DELETE /api/v1/user/clash/rules/:id
删除规则

### POST /api/v1/user/clash/rules/:id/reset
重置规则统计

### POST /api/v1/user/clash/rules/:id/token
生成订阅令牌

### GET /api/v1/user/clash/default-rule
获取默认规则

### GET /clash/subscribe/:token
Clash 订阅端点（外部访问）

---

## 通用响应格式

### 成功响应
```json
{
  "success": true,
  "data": { ... },
  "message": "操作成功"
}
```

### 错误响应
```json
{
  "success": false,
  "error": {
    "code": "INVALID_REQUEST",
    "message": "请求参数错误",
    "details": { ... }
  }
}
```

### 分页响应
```json
{
  "success": true,
  "data": [ ... ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

---

## 认证方式

### Bearer