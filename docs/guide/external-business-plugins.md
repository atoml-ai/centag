# 外部业务插件接入

Centag 默认二进制**不包含** `plugins/business/*`。流水线保留 `BusinessPlugin` 扩展点；业务实现通过外部 Go module 引入。

## 方式

1. 在消费方仓库（或本仓临时联调）的 `go.mod` 中：

```go
require example.com/org/centag-business v0.1.0

replace example.com/org/centag-business => ../centag-business
```

2. 在发行版入口（如 `dist/gateway/main.go` 或 `cmd/centag/main.go`）增加 blank import：

```go
import (
	_ "example.com/org/centag-business/router"
	_ "example.com/org/centag-business/optimizer"
	// ...
)
```

3. 构建时打开对应 `business_*` tags（与插件 `register.go` 的 `//go:build` 一致），例如：

```bash
go build -tags 'protocol_openai,...,business_router,business_optimizer' ./cmd/centag
```

也可把 tags 写回 `Makefile` 的 `BUILD_TAGS` / `start.sh` 的 `_FULL_FEATURE_TAGS`。

## 发行版策略

| 发行版 | 核心插件（本仓） | 业务插件 |
|--------|------------------|----------|
| minimal | 精简协议/后端 | 可选外部 |
| gateway | 全量协议/后端 + sqlite/pg + storage | 可选外部 |
| team | 与 gateway 对齐 | 可选外部 |

未引入外部业务模块时，网关仍可独立运行；流水线使用内置节点。
