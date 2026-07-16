# Centag 权限模型文档

> 面向 AI 编码智能体：定义插件权限模型、CapabilityBroker 工作机制、权限校验流程。

---

## 1. 权限模型概述

Centag 采用 **声明式权限 + 运行时强制** 模型，通过 `CapabilityBroker` 统一管控插件能力访问。

### 1.1 核心原则

- **最小权限**：插件仅获得声明且审批的权限
- **运行时强制**：每次能力调用前校验权限，非 silent fail
- **审计可观测**：所有权限拒绝记录审计日志
- **默认拒绝**：未显式声明的权限一律拒绝

### 1.2 权限类型

| 能力 | 权限标识 | 说明 | 示例场景 |
|------|----------|------|----------|
| LLM 调用 | `llm.call` | 调用 LLM 后端生成响应 | 模型推理、对话 |
| 存储读 | `storage.read` | 读取缓存/持久化数据 | 读取对话历史 |
| 存储写 | `storage.write` | 写入缓存/持久化数据 | 保存对话记录 |
| 记忆读 | `memory.read` | 读取向量记忆库 | RAG 检索 |
| 记忆写 | `memory.write` | 写入向量记忆库 | 记忆沉淀 |
| 密钥读 | `secrets.read` | 读取密钥/API Key | 获取第三方服务凭证 |
| HTTP 出站 | `network.outbound` | 发起出站 HTTP 请求 | Webhook、API 调用 |

---

## 2. 权限声明

### 2.1 插件描述符（Descriptor）

每个插件通过 descriptor 声明所需权限：

```yaml
# 示例：memory-local-bridge 插件描述符
implementation: "builtin.memory-local-bridge"
kind: "memory"
version: "1.0.0"
name: "Memory Local Bridge"
description: "本地记忆桥接插件"

permissions:
  - "memory.read"
  - "memory.write"
  - "llm.call"
  - "storage.read"

inputs:
  query:
    type: "string"
    required: true
  user_id:
    type: "string"
    default: "anonymous"
```

### 2.2 权限校验规则

```go
// internal/pipeline/capability_broker.go
func (b *CapabilityBroker) CheckPermission(pluginID string, capability string) error {
    plugin, err := b.registry.Get(pluginID)
    if err != nil {
        return ErrPluginNotFound
    }
    
    // 检查插件是否声明该权限
    for _, perm := range plugin.Descriptor().Permissions {
        if perm == capability {
            // 额外检查：插件状态、配额、限流等
            if err := b.checkPluginHealth(plugin); err != nil {
                return ErrPluginUnhealthy
            }
            return nil
        }
    }
    
    // 未声明权限，记录审计日志并拒绝
    b.auditLog.LogPermissionDenied(pluginID, capability)
    return ErrPermissionDenied
}
```

---

## 3. CapabilityBroker 架构

### 3.1 核心接口

```go
// internal/pipeline/capability_broker.go
type CapabilityBroker interface {
    // 权限检查
    CheckPermission(pluginID string, capability string) error
    
    // LLM 能力
    CallLLM(ctx context.Context, pluginID string, req *LLMRequest) (*LLMResponse, error)
    
    // 存储能力
    ReadStorage(ctx context.Context, pluginID, key string) ([]byte, error)
    WriteStorage(ctx context.Context, pluginID, key string, value []byte) error
    
    // 记忆能力
    ReadMemory(ctx context.Context, pluginID, query string) ([]MemoryItem, error)
    WriteMemory(ctx context.Context, pluginID string, item *MemoryItem) error
    
    // 密钥能力
    ReadSecret(ctx context.Context, pluginID, key string) (string, error)
    
    // HTTP 能力
    HTTPClient(pluginID string) (*http.Client, error)
}
```

### 3.2 调用流程

```
NodePlugin.Execute()
    ↓
CapabilityBroker.CallLLM(pluginID, req)
    ↓
CheckPermission(pluginID, "llm.call")
    ↓ (权限校验通过)
实际 LLM 调用
    ↓
返回结果 / 错误
```

### 3.3 错误处理

| 错误 | 说明 | HTTP 状态码 | 审计事件 |
|------|------|------------|----------|
| `ErrPermissionDenied` | 权限未声明 | 403 | ✅ |
| `ErrPluginNotFound` | 插件不存在 | 404 | ✅ |
| `ErrPluginUnhealthy` | 插件不健康 | 503 | ✅ |
| `ErrQuotaExceeded` | 配额超限 | 429 | ✅ |

错误响应格式：
```json
{
    "code": 403,
    "message": "permission denied",
    "error": "plugin builtin.memory-local-bridge lacks permission: llm.call",
    "details": {
        "plugin_id": "builtin.memory-local-bridge",
        "required_permission": "llm.call",
        "request_id": "req-12345"
    }
}
```

---

## 4. 审计日志

### 4.1 审计事件类型

```go
type AuditEvent struct {
    Timestamp   time.Time `json:"timestamp"`
    EventType   string    `json:"event_type"`
    PluginID    string    `json:"plugin_id"`
    Capability  string    `json:"capability"`
    RequestID   string    `json:"request_id"`
    UserID      string    `json:"user_id,omitempty"`
    Allowed     bool      `json:"allowed"`
    Reason      string    `json:"reason,omitempty"`
}
```

### 4.2 审计事件示例

```json
{
    "timestamp": "2026-05-05T14:30:00Z",
    "event_type": "permission_check",
    "plugin_id": "builtin.memory-local-bridge",
    "capability": "llm.call",
    "request_id": "req-abc123",
    "user_id": "user-001",
    "allowed": false,
    "reason": "permission not declared in descriptor"
}
```

### 4.3 审计日志存储

- **存储位置**：`logs/audit.log`（文件）+ 数据库 `audit_events` 表
- **保留策略**：文件日志 30 天轮转，数据库 90 天归档
- **查询接口**：`GET /api/v1/admin/audit-events`

---

## 5. 实施状态（2026-05-05）

### 5.1 完成项（✅）

- [x] 权限模型设计文档
- [x] CapabilityBroker 接口定义
- [x] 插件描述符 permissions 字段
- [x] 错误响应格式定义

### 5.2 进行中（🔄）

- [ ] CapabilityBroker 主链路注入（`internal/proxy/pipeline_mode.go`）
- [ ] 权限校验运行时强制
- [ ] 审计日志完整实现

### 5.3 待完成（❌）

- [ ] 权限拒绝测试用例
- [ ] 集成测试（权限拦截验证）
- [ ] WebUI 权限配置界面
- [ ] 权限模板库（常用权限组合）

---

## 6. 开发指南

### 6.1 添加新权限类型

1. 在 `internal/pipeline/capability_broker.go` 定义权限常量：
   ```go
   const (
       PermLLMCall    = "llm.call"
       PermStorageRead = "storage.read"
       // 添加新权限
       PermNewCapability = "new.capability"
   )
   ```

2. 在 `CheckPermission` 中添加校验逻辑（如需特殊检查）

3. 更新本文档权限类型表

4. 添加单元测试

### 6.2 插件开发者声明权限

```yaml
# plugins/my-plugin/descriptor.yaml
implementation: "custom.my-plugin"
kind: "processor"
version: "1.0.0"

permissions:
  - "llm.call"          # 需要调用 LLM
  - "storage.read"       # 需要读缓存
  # 不声明不需要的权限
```

### 6.3 权限问题排查

**问题**：插件调用失败，返回 403

**排查步骤**：
1. 检查插件 descriptor 是否声明所需权限
2. 查看审计日志 `logs/audit.log`
3. 检查插件状态是否健康
4. 验证 CapabilityBroker 是否正确注入

---

## 7. 安全最佳实践

1. **最小权限原则**：只声明必需的权限
2. **定期审计**：每月审查插件权限使用情况
3. **权限变更审批**：敏感权限变更需管理员审批
4. **监控告警**：权限拒绝率超阈值时告警
5. **密钥隔离**：secrets.read 权限仅授予受信插件

---

*最后更新：2026-05-05*
*维护者：见 ../../docs/harness/AGENTS.md*
