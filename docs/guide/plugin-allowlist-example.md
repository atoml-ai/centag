# 插件官方 Allowlist 示例

本文档提供生产部署时的插件 allowlist 配置示例。

## 受信任的插件来源

### 官方插件（内置）

```
# Centag 官方内置插件
builtin.generator
builtin.processor
builtin.reviewer
builtin.router
builtin.aggregator
builtin.audit
builtin.optimize
```

### 官方示例插件

```
# 官方示例插件（需审核后启用）
example.pii-redact
example.rag-retrieval
example.memory-query
example.tool-call
example.json-validator
```

### 受信任的第三方插件来源

```
# 受信任的域名
https://plugins.centag.ai
https://plugins.example.com
```

## 允许的 URL 模式

### 内部插件服务

```
# 内部插件（假设）
http://internal-plugin-*.internal:3000
http://10.0.0.0/8
```

### 外部受信任插件

```
# 受信任的外部插件（需审核）
https://plugins.trusted-partner.com
```

### 禁止的 URL 模式

```
# 禁止的 URL（安全考虑）
http://localhost:*          # 生产环境禁用 localhost
http://127.0.0.1:*        # 同上
http://192.168.0.0/16     # 内网地址（按需放行）
http://10.0.0.0/8          # 内网地址（按需放行）
```

## 推荐的权限配置

### 最小权限原则

| 插件类型 | 推荐权限 | 说明 |
|----------|------------|------|
| 生成插件 | `llm.call` | 只需调用 LLM |
| 处理插件 | `llm.call` | 只需调用 LLM |
| 审核插件 | `llm.call` | 只需调用 LLM |
| 检索插件 | `storage.read` | 只需读存储 |
| 记忆插件 | `memory.read`, `memory.write` | 读写记忆 |
| 工具调用插件 | `network.outbound`, `secrets.read` | 外部调用 + 密钥读取 |
| PII 脱敏插件 | (无) | 纯本地处理 |

### 权限配置示例

```json
{
  "plugins": {
    "example.pii-redact": {
      "enabled": true,
      "allowlist": {
        "permissions": []
      }
    },
    "example.rag-retrieval": {
      "enabled": true,
      "allowlist": {
        "permissions": ["storage.read"],
        "storage_namespace": "knowledge-base"
      }
    },
    "example.tool-call": {
      "enabled": true,
      "allowlist": {
        "permissions": ["network.outbound", "secrets.read", "llm.call"],
        "network_allowlist": ["https://api.example.com"],
        "secrets": ["api_key"]
      }
    }
  },
  "global": {
    "default_permissions": ["llm.call"],
    "network_policy": {
      "default": "deny",
      "allowlist": [
        "https://plugins.centag.ai",
        "https://*.trusted-partner.com"
      ]
    }
  }
}
```

## 启用/禁用插件

### 通过配置文件

```yaml
# config.yaml
pipeline:
  plugins:
    example.pii-redact:
      enabled: true
    example.rag-retrieval:
      enabled: false  # 临时禁用
```

### 通过 API

```bash
# 禁用插件
curl -X PATCH <http://localhost:20060/api/v1/pipelines/node-plugins/example.pii-redact> \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'

# 启用插件
curl -X PATCH <http://localhost:20060/api/v1/pipelines/node-plugins/example.pii-redact> \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

## 审计建议

1. **定期审查** allowlist，移除不再使用的插件
2. **最小化权限**，只授予插件所需的最小权限
3. **监控插件调用**，异常调用及时告警
4. **签名验证**，生产环境建议启用插件响应签名
5. **网络隔离**，敏感插件部署在独立网络环境

## 紧急情况处理

### 插件异常行为

```bash
# 1. 立即禁用问题插件
curl -X PATCH <http://localhost:20060/api/v1/pipelines/node-plugins/problem-plugin> \
  -d '{"enabled": false}'

# 2. 查看插件执行历史
curl <http://localhost:20060/api/v1/pipelines/executions?plugin=problem-plugin>

# 3. 检查插件指标
curl <http://localhost:20060/metrics> | grep plugin
```

### 熔断状态检查

```bash
# 查看所有插件状态
curl <http://localhost:20060/api/v1/pipelines/node-plugins> | jq '.[].{impl: .implementation, circuit: .circuit_state}'
```
