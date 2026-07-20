# 外部业务插件接入

Centag 默认二进制**不包含** `plugins/business/*`。流水线保留 `BusinessPlugin` 扩展点；业务实现通过外部 Go module 引入。

## 方式

1. 在消费方仓库（或本仓临时联调）的 `go.mod` 中：

```go
require example.com/org/centag-business v0.1.0

replace example.com/org/centag-business => ../centag-business
```

2. 在发行版入口（如 `dist/personal/main.go` 或 `cmd/centag/main.go`）增加 blank import：

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
| personal | 全量协议/后端 + sqlite/pg + storage | 可选外部 |
| team | 与 personal 对齐 | 可选外部（商业增值见下节） |

未引入外部业务模块时，网关仍可独立运行；流水线使用内置节点。

## Open Core / Team 商业扩展（centag-pro）

商业化分层（v0.2.7）约定：

1. 开源仓 **只有** `minimal` / `personal` 发行版入口；**已删除 `dist/team`**。
2. Team（及未来 Enterprise）在私有仓 **`centag-pro`** 构建：`cmd/centag-team` + `bundle/*` + `plugins/*`；依赖方向 **pro → 开源**。
3. 开源提供 `core/pkg/extension`（Plugin Host）与 `editionmodule`（admin 增量）；**不要**用流水线 `BusinessPlugin` 冒充商业管理面。
4. 开源 `start.sh build team` 仅**转调** pro；无 pro checkout 时构建失败。

详见 [`docs/versions/v0.2.7/commercialization-layered/技术方案.md`](../versions/v0.2.7/commercialization-layered/技术方案.md) 与 [`dist/README.md`](../../dist/README.md)。