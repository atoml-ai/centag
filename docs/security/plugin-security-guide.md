# 插件安全指南

本文档描述了 Pipeline 插件节点的安全设计原则与最佳实践，包括安全治理与准入标准。

## 0. 安全治理概述

Centag 采用多层安全防护机制确保插件安全上线：

```
┌─────────────────────────────────────────────────────────────┐
│                    安全治理架构                              │
├─────────────────────────────────────────────────────────────┤
│  第1层: 来源验证 (Allowlist)                                 │
│  - 插件来源白名单                                            │
│  - 主机/IP 白名单                                            │
├─────────────────────────────────────────────────────────────┤
│  第2层: 完整性验证 (签名/哈希)                               │
│  - Manifest 哈希锁定                                          │
│  - 签名验证 (未来支持)                                        │
├─────────────────────────────────────────────────────────────┤
│  第3层: 网络策略                                             │
│  - 默认拒绝出站                                              │
│  - 允许/禁止端点列表                                          │
│  - 端口控制                                                  │
├─────────────────────────────────────────────────────────────┤
│  第4层: 准入检查                                             │
│  - 权限最小化检查                                            │
│  - 超时配置检查                                              │
│  - 错误处理检查                                              │
│  - 可观测性检查                                              │
└─────────────────────────────────────────────────────────────┘
```

## 1. 权限最小化原则

### 1.1 节点权限声明

每个节点在 `NodePluginDescriptor` 中必须显式声明所需的权限：

```go
func MyNodeDescriptor() pipeline.NodePluginDescriptor {
    return pipeline.NodePluginDescriptor{
        Permissions: []string{"llm.call"}, // 仅声明必需权限
    }
}
```

### 1.2 权限类型

| 权限 | 说明 | 风险等级 |
|------|------|----------|
| `llm.call` | 调用 LLM 生成内容 | 中 |
| `memory.read` | 读取记忆数据 | 中 |
| `memory.write` | 写入记忆数据 | 高 |
| `secrets.read` | 读取密钥 | 高 |
| `network.outbound` | 发起外部网络请求 | 高 |

### 1.3 权限检查

CapabilityBroker 在节点执行前进行权限验证：

```go
func (n *MyNode) Execute(ctx context.Context, input *pipeline.NodeInput) (*pipeline.NodeOutput, error) {
    broker := n.GetCapabilityBroker()
    perms := n.GetPermissions()
    if len(perms) == 0 {
        perms = []string{"llm.call"} // 默认权限
    }
    llmClient, err := broker.GetLLMClient(ctx, perms)
    // ...
}
```

## 2. PII 数据处理

### 2.1 PII 识别

节点在处理用户输入时，应识别并脱敏以下 PII：

- 姓名、电子邮件、电话号码
- 身份证号、银行卡号
- 地址、位置信息

### 2.2 脱敏配置

在节点配置中启用 PII 脱敏：

```yaml
nodes:
  - id: "processor"
    type: "processor"
    config:
      custom_config:
        pii_masking:
          enabled: true
          rules:
            - type: "email"
            - type: "phone"
            - type: "id_card"
```

### 2.3 内置脱敏器

Pipeline 提供内置的 PII 脱敏器：

```go
import "centag/internal/pipeline"

func ProcessWithMasking(input string) string {
    masker := pipeline.NewSensitiveMasker()
    return masker.Mask(input)
}
```

## 3. 密钥管理

### 3.1 密钥引用

节点通过 `secrets_ref` 引用密钥：

```yaml
nodes:
  - id: "memory-node"
    type: "memory"
    config:
      secrets_ref:
        - "memory_api_key"
      custom_config:
        query_type: "user"
```

### 3.2 密钥解析

CapabilityBroker 解析密钥引用：

```go
func (n *MyNode) Execute(ctx context.Context, input *pipeline.NodeInput) (*pipeline.NodeOutput, error) {
    if len(n.config.SecretsRef) > 0 {
        resolver, err := n.capabilityBroker.GetSecretsResolver(ctx, n.GetPermissions())
        if err != nil {
            return nil, fmt.Errorf("cannot acquire secrets resolver: %w", err)
        }
        for _, ref := range n.config.SecretsRef {
            value, err := resolver.Resolve(ref)
            if err != nil {
                return nil, fmt.Errorf("failed to resolve secret %q: %w", ref, err)
            }
            // 使用密钥
        }
    }
}
```

### 3.3 安全要求

- 密钥不得硬编码在代码中
- 密钥不得写入日志
- 密钥在传输过程中必须加密

## 4. 越权能力限制

### 4.1 能力边界

每个节点有其能力边界，不得超越：

| 节点类型 | 允许操作 | 禁止操作 |
|----------|----------|----------|
| `memory` | 读取用户/会话记忆 | 写入、删除、跨用户访问 |
| `audit` | 内容审核 | 修改内容、执行代码 |
| `optimize` | 文本优化 | 调用外部 API、访问文件系统 |

### 4.2 上下文隔离

节点只能访问其上下文中可用的数据：

```go
func (n *MemoryNode) Execute(ctx context.Context, input *pipeline.NodeInput) (*pipeline.NodeOutput, error) {
    // 仅能访问 input.Content 和 input.Context
    userID := "0"
    if input.Context != nil {
        if uid, ok := input.Context["user_id"].(string); ok {
            userID = uid
        }
    }
    // 不能访问其他用户的数据
}
```

### 4.3 超时与资源限制

节点必须遵守全局超时配置：

```go
func NewMyNode(config pipeline.NodeConfig) (pipeline.PipelineNode, error) {
    return &MyNode{
        BaseNode: pipeline.BaseNode{
            config:      config,
            timeout:     30, // 默认超时
            retryConfig: pipeline.DefaultRetryConfig(),
        },
    }, nil
}
```

## 5. 安全清单

部署前请确认以下检查项：

- [ ] 节点已声明最小必要权限
- [ ] PII 数据在配置中已脱敏或标记
- [ ] 密钥通过 secrets_ref 引用，不在配置中明文
- [ ] 节点仅能访问授权的数据源
- [ ] 超时配置与全局配置一致
- [ ] 节点日志不包含敏感信息

## 6. 插件上架流程

### 6.1 上架流程概览

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   开发阶段    │ →  │   自检阶段    │ →  │   审查阶段    │ →  │   上线阶段    │
└──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘
      │                   │                   │                   │
      ▼                   ▼                   ▼                   ▼
  开发插件实现      运行准入检查      安全团队审查       部署并监控
```

### 6.2 开发阶段

1. 实现插件接口 (`NodePlugin`)
2. 编写 `NodePluginDescriptor`，声明权限
3. 配置 manifest (`/.well-known/centag-node-plugin.json`)

### 6.3 自检阶段

运行准入检查：
```bash
go test ./internal/pipeline/... -run TestAdmission
```

检查清单：
- [ ] 权限声明最小化
- [ ] 超时配置合理 (5-300秒)
- [ ] 版本号已设置
- [ ] 提供可观测性接口

### 6.4 审查阶段

安全团队审查要点：
- 权限是否最小化
- 是否有潜在安全风险
- 网络访问是否合规
- 错误处理是否完善

### 6.5 上线阶段

1. 在配置中注册插件
2. 启用安全验证（如需要）
3. 监控运行状态

## 7. 安全配置

### 7.1 配置结构

在 `Config.PluginSecurity` 中配置：

```go
type PluginSecurityConfig struct {
    AllowlistEnabled bool   // 是否启用白名单
    AllowedSources   []string // 允许的来源
    AllowedHosts     []string // 允许的主机
    RequireSignature bool   // 是否要求签名
    RequireHashLock  bool   // 是否要求哈希锁定
    NetworkPolicy    PluginNetworkPolicy // 网络策略
    AdmissionCheck   PluginAdmissionConfig // 准入检查
}
```

### 7.2 启用白名单模式

```yaml
plugin_security:
  allowlist_enabled: true
  allowed_sources:
    - "plugins.example.com"
    - "internal.company.com"
  allowed_hosts:
    - "192.168.1.0/24"
    - "10.0.0.10"
```

### 7.3 启用哈希锁定

```yaml
plugin_security:
  require_hash_lock: true
  # 远程插件 manifest 需包含 expected_hash 字段
```

### 7.4 网络策略

```yaml
plugin_security:
  network_policy:
    default_deny: true  # 默认拒绝所有出站
    allowed_endpoints:
      - "api.openai.com"
      - "api.anthropic.com"
    blocked_endpoints:
      - "malicious.com"
    allowed_ports:
      - 443
      - 80
    blocked_ports:
      - 22
      - 3389
```

## 8. 回滚策略

### 8.1 回滚触发条件

- 准入检查失败且无法修复
- 安全漏洞被披露
- 运行时异常频发

### 8.2 回滚步骤

1. 从配置中移除插件
2. 重启服务
3. 检查日志确认卸载成功
4. 通知相关方

### 8.3 快速回滚配置

在配置中预留备用插件：
```yaml
pipelines:
  - id: "main-pipeline"
    nodes:
      - id: "processor"
        implementation: "plugin-v1"  # 主插件
      - id: "processor-fallback"
        implementation: "builtin-processor"  # 备用
```

## 9. 报告安全漏洞

如发现安全漏洞，请通过以下方式报告：

1. 创建 GitHub Issue，标记为 `security`
2. 发送邮件至 security@example.com
3. 提供漏洞详情和复现步骤

我们重视所有安全报告，并会在 24 小时内响应。

---

*最后更新：2026-05-04*
*版本：1.1.0*