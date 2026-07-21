# 外部业务插件接入

Centag 默认二进制**不包含** `plugins/business/*`。流水线保留 `BusinessPlugin` 扩展点；业务实现通过外部 Go module 引入。

## 方式

1. 在消费方仓库（或本仓临时联调）的 `go.mod` 中：

```go
require example.com/org/centag-pro v0.1.0

replace example.com/org/centag-pro => ../centag-pro
```

2. 在发行版入口（如 `dist/personal/main.go` 或 `cmd/centag/main.go`）增加 blank import：

```go
import (
	_ "example.com/org/centag-pro/router"
	_ "example.com/org/centag-pro/optimizer"
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
4. Team **只**在 `centag-pro` 构建（`./start.sh build team`，与开源用法对齐）；开源 `start.sh` **拒绝** `build team` / `debug team`。

### Extension Host 白名单（E2.0+）

`extension.Host` 当前能力（插件在 `Init` 中调用；Server 在组装完 deps 后 `InitAll`，再于 `setupRoutes` 落盘）：

| 方法 | 用途 |
|------|------|
| `Edition()` | 运行时 edition |
| `Deps()` | 开源句柄袋（HookManager 等；E2.1+ 继续充实） |
| `RegisterTeamAdmin` | 挂到 `/api/v1/admin`（JWT+admin+team 门控） |
| `RegisterUserAPI` | 挂到 `/api/v1/user` |
| `RegisterSystemAPI` | 挂到 `/api/v1/system`（team 门控） |
| `RegisterProtectedMiddleware` | 挂到 proxy-auth 后的 v1 组 |
| `RegisterBillingHook` | 排队后 flush 到 HookManager |
| `RegisterCloser` | 关机时清理（如 billing.Service） |

存量产品 API **不改路径**；`/api/v1/admin/pro` 仍只给 `editionmodule` 增量用。

**D6**：Team 产品实现在闭源 `centag-pro/internal/teamadmin`；开源仅提供 Host 与原语 facade（`authapi`、`tokenusageapi`、`billingasync`、`quotaapi`、`agentapi`、`abevalapi`、`systemupdateapi`）。`plugins/team` 在 license 有效时挂载：

- `/api/v1/admin/users*`、代管 API Keys、tenants、admin usage/quotas、`/admin/ab-eval*`、BillingHook、租户 QuotaMiddleware
- `/api/v1/system/update*`（及 rollback/delete-update）

用户自管 Key、token-usage、cost/summary、billing/rules、AB Persist 钩子仍在开源；personal/开源二进制不注册上述 team 产品面。

前端（E3）：开源 `web/` 为 Host；Team 页面在 `centag-pro/web/packs/team`，经 `./start.sh build fe` 注入。`/cost` 留开源（D1）。personal 构建使用空 stub，产物无 Users/Tenants 等 chunk。

详见 [`docs/versions/v0.2.7/commercialization-layered/技术方案.md`](../versions/v0.2.7/commercialization-layered/技术方案.md) 与 [`dist/README.md`](../../dist/README.md)。