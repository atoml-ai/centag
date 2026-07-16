# 远程插件签名验证

本文档描述 Centag 远程流水线节点插件的签名验证机制。

## 概述

为防止恶意插件被加载到流水线中，Centag 支持对远程插件 manifest 进行 Ed25519 数字签名验证。管理员可配置一组信任的公钥，系统会在插件发现（discover）和注册时自动验证其签名。

## 签名算法

- **算法**: Ed25519
- **签名内容**: `NodePluginDescriptor` 的规范 JSON 序列化（**不含** `signature` 字段本身）
- **编码**: Base64（URL-safe 未采用，使用标准 Base64）
- **公钥格式**: 32 字节，Base64 编码
- **签名长度**: 64 字节，Base64 编码后约 88 字符

## Manifest 格式

远程插件在 `/.well-known/centag-node-plugin.json` 中提供的 manifest 可包含可选的 `signature` 字段：

```json
{
  "name": "My Secure Plugin",
  "implementation": "example.secure-plugin",
  "kind": "llm.generate",
  "version": "1.0.0",
  "signature": "BASE64_ED25519_SIGNATURE_HERE",
  "permissions": ["llm.call"]
}
```

签名覆盖上述 JSON **去掉 `signature` 字段后**的规范序列化内容。

## 配置

在数据库配置 JSON 的 `plugin_security` 段中添加信任的公钥：

```json
{
  "plugin_security": {
    "require_signature": false,
    "trusted_public_keys": [
      "BASE64_ENCODED_32BYTE_PUBLIC_KEY_1",
      "BASE64_ENCODED_32BYTE_PUBLIC_KEY_2"
    ]
  }
}
```

| 字段 | 说明 |
|------|------|
| `require_signature` | 是否强制要求所有远程插件必须提供有效签名。`false` 时，有签名的插件会验证，无签名的插件跳过 |
| `trusted_public_keys` | 信任的 Ed25519 公钥列表。任一公钥验证通过即算成功 |

## 签名状态

数据库 `plugin_registry.signature_status` 记录验证结果：

| 状态值 | 含义 |
|--------|------|
| `none` | 插件未提供签名，或签名验证被跳过 |
| `present` | 插件提供了签名，但系统未启用安全验证器 |
| `verified` | 签名验证通过 |
| `invalid` | 签名验证失败（签名不匹配、公钥未配置等） |

## 验证流程

1. **发现插件** (`POST /api/v1/pipelines/node-plugins/discover`)
   - 获取远程 manifest
   - 若 manifest 包含 `signature`，使用配置的公钥验证
   - 将验证结果写入 `signature_status`

2. **注册插件** (`NodeRegistry.RegisterPlugin`)
   - 若启用准入检查 (`AdmissionChecker`)，签名验证作为其中一步
   - 验证失败会导致注册被拒绝（当 `RequireSignature=true` 时）

3. **执行插件**
   - 当前版本不验证执行响应的签名（未来可选扩展）

## 生成签名示例（Go）

```go
package main

import (
    "crypto/ed25519"
    "encoding/base64"
    "encoding/json"
    "fmt"
)

type Manifest struct {
    Implementation string `json:"implementation"`
    Kind           string `json:"kind"`
    Version        string `json:"version"`
    Signature      string `json:"signature,omitempty"`
}

func main() {
    _, priv, _ := ed25519.GenerateKey(nil)
    pub := priv.Public().(ed25519.PublicKey)
    fmt.Println("Public Key (base64):", base64.StdEncoding.EncodeToString(pub))

    manifest := Manifest{
        Implementation: "example.my-plugin",
        Kind:           "llm.generate",
        Version:        "1.0.0",
    }
    data, _ := json.Marshal(manifest)
    sig := ed25519.Sign(priv, data)
    manifest.Signature = base64.StdEncoding.EncodeToString(sig)

    out, _ := json.MarshalIndent(manifest, "", "  ")
    fmt.Println(string(out))
}
```

## CLI 生成密钥对

```bash
# 使用 openssl（Ed25519 支持需 OpenSSL 3.0+）
openssl genpkey -algorithm Ed25519 -out plugin_key.pem
openssl pkey -in plugin_key.pem -pubout -out plugin_pub.pem

# 提取 base64 公钥
cat plugin_pub.pem | grep -v '^-' | base64 -d | base64
```

## 安全建议

1. **生产环境**建议启用 `require_signature: true`
2. 公钥应通过安全渠道分发，不应随代码仓库公开传播（除非是示例/测试用途）
3. 定期轮换密钥对，并在 `trusted_public_keys` 中保留新旧公钥的过渡期
4. 签名失败会被记录到日志，建议配合告警系统监控 `invalid` 状态

## 相关代码

- `internal/pipeline/plugin_security.go` — `ValidateSignature` / `ValidateManifestSignature`
- `internal/pipeline/plugin_admission.go` — `CheckSignature` / `CheckAll`
- `internal/server/pipeline_handler.go` — `DiscoverRemoteNodePlugin` 中的签名状态更新
- `internal/pipeline/plugin_contract.go` — `NodePluginDescriptor.Signature`
