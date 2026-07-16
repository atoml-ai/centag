# 网络策略配置

> 面向 AI 编码智能体：定义插件的网络访问控制策略。

---

## 1. 概述

网络策略控制插件能否发起出站HTTP请求，以及可以访问哪些端点。

### 1.1 策略层级

```
┌─────────────────────────────────────────────────┐
│                  网络策略                              │
├─────────────────────────────────────────────────┤
│  第1层: 全局默认策略                          │
│  - 默认拒绝所有出站请求 (default_deny: true)              │
│  - 仅允许显式声明的权限和端点                      │
├─────────────────────────────────────────────────┤
│  第2层: 权限检查                              │
│  - 检查插件是否声明 `network.outbound` 权限                │
│  - 未声明则拒绝所有网络请求                        │
├─────────────────────────────────────────────────┤
│  第3层: 端点白名单                            │
│  - 根据插件实现匹配允许的端点列表                     │
│  - 支持精确匹配、前缀匹配、正则匹配                     │
├─────────────────────────────────────────────────┤
│  第4层: 请求过滤                              │
│  - 检查目标主机、端口、路径                          │
│  - 检查请求头和请求体大小                           │
│  - 记录所有网络访问（审计日志）                        │
└─────────────────────────────────────────────────┘
```

---

## 2. 配置示例

### 2.1 基础配置

```yaml
# 旧版配置已归档至 ../../archive/deprecated/configs/plugin-security.yaml
plugin_security:
  network_policy:
    # 默认策略：拒绝所有未显式允许的请求
    default_deny: true
    
    # 全局最大连接数
    max_connections: 100
    
    # 全局请求超时（秒）
    request_timeout: 30
    
    # 允许的端点列表
    allowed_endpoints:
      # 允许访问 OpenAI API
      - host: "api.openai.com"
        ports: [443]
        path_prefix: "/v1/"
        max_requests_per_minute: 100
        
      # 允许访问 Anthropic API
      - host: "api.anthropic.com"
        ports: [443]
        path_prefix: "/v1/"
        
      # 允许访问内网服务（正则匹配）
      - host_pattern: "^internal\\.company\\.com$"
        ports: [80, 443]
        methods: ["GET", "POST"]
        
      # 允许访问本地服务
      - host: "localhost"
        ports: [8080, 8081]
        path_prefix: "/webhook/"
```

### 2.2 插件级覆盖

```yaml
# 插件可以声明自己的网络需求
# plugins/my-plugin/descriptor.yaml
implementation: "https://my-plugin.example.com"
kind: "processor"
version: "1.0.0"
permissions:
  - "network.outbound"

# 插件特定的网络策略（可选）
network_policy:
  allowed_endpoints:
    - host: "api.external-service.com"
      ports: [443]
      path_prefix: "/v1/"
  max_request_body: 1048576  # 1 MB
  max_response_body: 4194304  # 4 MB
```

---

## 3. 实现机制

### 3.1 CapabilityBroker 网络检查

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
        b.auditLog.LogNetworkDenied(pluginID, "permission not declared")
        return nil, ErrPermissionDenied
    }
    
    // 创建受控的 HTTP 客户端
    return &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxResponseHeaderBytes: 4 << 20, // 4 MB
            TLSClientConfig: &tls.Config{
                MinVersion: tls.VersionTLS12,
            },
            // 自定义 DialContext 进行端点检查
            DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
                // 解析主机和端口
                host, port, err := net.SplitHostPort(addr)
                if err != nil {
                    return nil, err
                }
                
                // 检查端点是否在允许列表中
                if err := b.checkEndpoint(pluginID, host, port); err != nil {
                    b.auditLog.LogNetworkDenied(pluginID, err.Error())
                    return nil, err
                }
                
                // 创建实际连接
                return (&net.Dialer{}).DialContext(ctx, network, addr)
            },
        },
    }, nil
}
```

### 3.2 端点检查实现

```go
// internal/pipeline/capability_broker.go
func (b *CapabilityBroker) checkEndpoint(pluginID, host string, port int) error {
    cfg := DefaultPluginSecurityConfig().NetworkPolicy
    
    // 检查全局允许列表
    for _, ep := range cfg.AllowedEndpoints {
        if b.matchEndpoint(ep, host, port) {
            return nil  // 允许
        }
    }
    
    // 检查插件特定允许列表（如果有的话）
    plugin, _ := b.registry.Get(pluginID)
    if plugin != nil {
        for _, ep := range plugin.Descriptor().NetworkPolicy.AllowedEndpoints {
            if b.matchEndpoint(ep, host, port) {
                return nil  // 允许
            }
        }
    }
    
    return fmt.Errorf("endpoint %s:%d not in allowlist", host, port)
}

func (b *CapabilityBroker) matchEndpoint(ep EndpointConfig, host string, port int) bool {
    // 检查端口
    portAllowed := false
    for _, p := range ep.Ports {
        if p == port {
            portAllowed = true
            break
        }
    }
    if !portAllowed {
        return false
    }
    
    // 检查主机（精确匹配）
    if ep.Host != "" && host == ep.Host {
        return true
    }
    
    // 检查主机（正则匹配）
    if ep.HostPattern != "" {
        matched, _ := regexp.MatchString(ep.HostPattern, host)
        if matched {
            return true
        }
    }
    
    return false
}
```

---

## 4. 监控与告警

### 4.1 指标

| 指标 | 说明 |
|------|------|
| `plugin_network_requests_total` | 插件网络请求总次数 |
| `plugin_network_denials_total` | 网络请求被拒绝次数 |
| `plugin_network_latency_seconds` | 网络请求延迟分布 |

### 4.2 告警规则

```yaml
# 告警：插件网络拒绝率过高
- alert: PluginNetworkDenialHigh
  expr: rate(plugin_network_denials_total[5m]) > 0.1
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "插件网络拒绝率过高: {{ $value }}"
```

---

## 5. 故障排查

### 5.1 问题：插件网络请求失败

**排查步骤**：
1. 检查插件是否声明 `network.outbound` 权限
2. 查看审计日志 `logs/audit.log` 中的网络拒绝记录
3. 检查目标端点是否在允许列表中
4. 验证插件特定网络策略（如果有）

### 5.2 测试网络策略

```bash
# 测试插件网络访问
curl -X POST http://localhost:20060/api/v1/pipelines/node-plugins/test-network/execute \
  -H "Content-Type: application/json" \
  -d '{
    "config": {"backend": "test"},
    "input": {"url": "https://api.openai.com/v1/models"}
  }'
```

---

## 6. 最佳实践

1. **最小权限**：只声明必需的 `network.outbound` 权限
2. **精确匹配**：尽量使用精确的主机和路径匹配，避免过于宽泛的正则
3. **超时设置**：为网络请求设置合理的超时时间
4. **监控告警**：监控网络拒绝率和延迟，及时发现异常
5. **审计日志**：记录所有网络访问，便于事后追溯

---

*最后更新：2026-05-06*
*维护者：见 ../../docs/harness/AGENTS.md*
