# 插件 Allowlist 策略

> 面向 AI 编码智能体：定义插件来源白名单策略，控制哪些插件可以被发现和加载。

---

## 1. 概述

Allowlist 策略用于控制插件来源，防止不可信插件进入系统。

### 1.1 策略层级

```
┌─────────────────────────────────────────────────┐
│                  Allowlist 策略                          │
├─────────────────────────────────────────────────┤
│  第1层: 来源验证 (Source Verification)              │
│  - 官方插件 (builtin.*) 自动允许                │
│  - 域名白名单 (trusted-domains)                  │
│  - 签名验证 (signature verification)           │
├─────────────────────────────────────────────────┤
│  第2层: 版本检查 (Version Check)                  │
│  - 最低版本要求 (min_centag_version)          │
│  - 版本兼容性检查                             │
├─────────────────────────────────────────────────┤
│  第3层: 哈希锁定 (Hash Lock)                    │
│  - ExpectedHash 字段验证                      │
│  - manifest SHA-256 哈希比对                  │
└─────────────────────────────────────────────────┘
```

---

## 2. 配置示例

### 2.1 信任域名配置

```yaml
# 旧版配置已归档至 ../../archive/deprecated/configs/plugin-security.yaml
plugin_security:
  admission_check:
    enabled: true
    allowlist:
      # 官方插件自动允许
      builtin_prefix: "builtin."
      
      # 信任的域名列表
      trusted_domains:
        - "https://plugins.centag.ai"
        - "https://github.com/centag-plugins"
        - "https://internal.company.com/plugins"
      
      # 显式允许的插件实现
      allowed_implementations:
        - "https://custom-plugin.example.com"
        - "builtin.memory-local-bridge"
      
      # 禁止的插件（优先级高于允许）
      blocked_implementations:
        - "builtin.deprecated-plugin"
  
  # 哈希锁定配置
  hash_verification:
    enabled: true
    auto_update: false  # 是否自动更新期望哈希
```

### 2.2 插件描述符中的哈希声明

```yaml
# 插件 manifest (.well-known/centag-node-plugin.json)
{
  "name": "My Custom Plugin",
  "implementation": "https://my-plugin.example.com",
  "kind": "processor",
  "version": "1.0.0",
  "expected_hash": "abc123...def789",  # 期望的 manifest SHA-256 哈希
  "permissions": ["llm.call"]
}
```

---

## 3. 验证流程

### 3.1 插件注册时的检查

```go
// internal/pipeline/plugin_registry.go
func (r *NodeRegistry) Register(plugin NodePlugin) error {
    // 1. 检查 allowlist
    if err := r.checkAllowlist(plugin.Descriptor()); err != nil {
        return fmt.Errorf("allowlist check failed: %w", err)
    }
    
    // 2. 检查哈希锁定
    if err := r.checkHashLock(plugin.Descriptor()); err != nil {
        return fmt.Errorf("hash verification failed: %w", err)
    }
    
    // 3. 准入检查
    if r.admissionChecker != nil && r.admissionChecker.cfg.Enabled {
        result := r.admissionChecker.CheckAll(plugin.Descriptor(), 30)
        if !result.Passed {
            return fmt.Errorf("admission check failed: %s", result.Summary())
        }
    }
    
    // 4. 注册插件
    // ...
}
```

### 3.2 Allowlist 检查实现

```go
// internal/pipeline/plugin_security.go
func (r *NodeRegistry) checkAllowlist(desc NodePluginDescriptor) error {
    impl := desc.Implementation
    
    // 官方插件自动允许
    if strings.HasPrefix(impl, BuiltinImplementationPrefix) {
        return nil
    }
    
    cfg := DefaultPluginSecurityConfig().Allowlist
    
    // 检查显式允许列表
    for _, allowed := range cfg.AllowedImplementations {
        if impl == allowed {
            return nil
        }
    }
    
    // 检查信任域名
    for _, domain := range cfg.TrustedDomains {
        if strings.HasPrefix(impl, domain) {
            return nil
        }
    }
    
    return fmt.Errorf("implementation %q not in allowlist", impl)
}
```

---

## 4. 网络策略

### 4.1 默认拒绝策略

```go
// internal/pipeline/capability_broker.go
func (b *CapabilityBroker) GetHTTPClient(pluginID string) (*http.Client, error) {
    plugin, err := b.registry.Get(pluginID)
    if err != nil {
        return nil, ErrPluginNotFound
    }
    
    // 检查网络权限
    hasNetworkAccess := false
    for _, perm := range plugin.Descriptor().Permissions {
        if perm == PermNetworkOutbound {
            hasNetworkAccess = true
            break
        }
    }
    
    if !hasNetworkAccess {
        return nil, ErrPermissionDenied
    }
    
    // 返回受控的 HTTP 客户端（超时、最大响应体、TLS 策略）
    return &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxResponseHeaderBytes: 10 << 20, // 10 MiB
            TLSClientConfig: &tls.Config{
                MinVersion: tls.VersionTLS12,
            },
        },
    }, nil
}
```

### 4.2 出站端点控制

```yaml
# 旧版配置已归档至 ../../archive/deprecated/configs/plugin-security.yaml
plugin_security:
  network_policy:
    default_deny: true  # 默认拒绝所有出站请求
    
    # 允许的端点列表
    allowed_endpoints:
      - host: "api.openai.com"
        port: [443]
        path_prefix: "/v1/"
      
      - host: "api.anthropic.com"
        port: [443]
        path_prefix: "/v1/"
      
      # 允许访问内网
      - host_pattern: "^internal\\.company\\.com$"
        port: [80, 443]
```

---

## 5. 监控与告警

### 5.1 指标

| 指标 | 说明 |
|------|------|
| `plugin_allowlist_denials_total` | Allowlist 拒绝次数 |
| `plugin_hash_verification_failures_total` | 哈希验证失败次数 |
| `plugin_network_denials_total` | 网络策略拒绝次数 |

### 5.2 告警规则

```yaml
# 告警：插件拒绝率过高
- alert: PluginRejectionHigh
  expr: rate(plugin_allowlist_denials_total[5m]) > 0.1
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "插件拒绝率过高: {{ $value }}"
```

---

## 6. 迁移指南

### 6.1 从开放模式迁移到 Allowlist

1. **第一阶段**：审计模式（只记录，不拒绝）
   ```yaml
   plugin_security:
     admission_check:
       enabled: true
     allowlist:
       mode: audit  # 只记录，不拒绝
   ```

2. **第二阶段**：灰度放开（部分插件放行）
   ```yaml
   plugin_security:
     allowlist:
       mode: graylist
       allowed_implementations:
         - "builtin.memory-local-bridge"
         - "https://trusted-partner.com/plugin"
   ```

3. **第三阶段**：完全启用（默认拒绝）
   ```yaml
   plugin_security:
     allowlist:
       mode: enforce  # 默认拒绝，只允许白名单
   ```

---

*最后更新：2026-05-06*
*维护者：见 ../../docs/harness/AGENTS.md*
