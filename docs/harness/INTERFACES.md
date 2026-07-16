# Centag 接口清单

本文件是 Centag 所有 HTTP 接口的完整清单，按分类组织。接口变更时必须同步更新本文件。

> 约束总则见 [AGENTS.md §2](AGENTS.md#2-接口边界与约束必须严格遵守)。

---

## 1. 代理接口（协议层，核心对外能力）

代理接口严格遵循标准大模型协议，仅做协议转换与转发，不包含业务逻辑。

| 方法 | 路径 | 协议 | 说明 |
|------|------|------|------|
| POST | `/v1/chat/completions` | OpenAI | Chat Completions |
| POST | `/v1/responses` | OpenAI | Responses（Codex / wire_api=responses） |
| POST | `/v1/messages` | Anthropic | Messages |
| POST | `/v1/embeddings` | OpenAI | Embeddings |
| POST | `/v1/completions` | OpenAI | Completions |
| GET | `/v1/models` | OpenAI | 模型列表 |
| POST | `/v1beta/models/*action` | Gemini | Gemini 原生入口（generateContent 等） |
| POST | `/v1/mcp` | MCP | JSON-RPC 2.0 代理 |
| POST | `/v1/mcp/capabilities` | MCP | 能力查询 |
| GET | `/v1/mcp/health` | MCP | 健康检查 |
| POST | `/api/v1/openai/chat/completions` | OpenAI | 兼容前缀（保留） |
| POST | `/api/v1/openai/embeddings` | OpenAI | 兼容前缀（保留） |
| GET | `/api/v1/openai/models` | OpenAI | 兼容前缀（保留） |

> 约束：代理接口不得增加业务参数、不得引入有状态逻辑。所有垂直场景能力通过请求头（`X-Pipeline-ID`、`X-Proxy-Mode`）或配置绑定选择 Pipeline 实现。

---

## 2. 管理接口（Centag 运维管理）

管理接口用于系统配置、资源管理与运维监控，仅限内部管理使用，不面向普通 Agent。

### 2.1 认证与用户

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 登录 |
| POST | `/api/auth/refresh` | 刷新 Token |
| POST | `/api/auth/logout` | 登出 |
| GET | `/api/auth/me` | 当前用户 |
| GET | `/api/v1/user/profile` | 获取用户资料 |
| PUT | `/api/v1/user/profile` | 更新用户资料 |
| PUT | `/api/v1/user/password` | 修改密码 |
| GET/POST/PUT/DELETE | `/api/v1/user/apikeys/*` | 用户 API Key 管理 |
| GET | `/api/v1/user/token-usage` | 用户 Token 用量 |
| GET | `/api/v1/user/token-usage/daily` | 用户每日用量 |
| GET | `/api/v1/user/token-usage/models` | 用户按模型统计 |
| GET | `/api/v1/user/token-usage/backends` | 用户按后端统计 |

### 2.2 管理员（团队版）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST/PUT/DELETE | `/api/v1/admin/users/*` | 用户管理 |
| GET/POST/PUT/DELETE | `/api/v1/admin/api-keys/*` | 虚拟 Key 管理 |
| GET | `/api/v1/admin/token-usage/all` | 全局用量统计 |
| GET | `/api/v1/admin/token-usage/ranking` | 用户用量排行 |
| GET/POST/PUT | `/api/v1/admin/quotas/*` | 配额管理 |
| GET | `/api/v1/admin/cost/summary` | 成本汇总 |
| GET/PUT/DELETE | `/api/v1/admin/tenants/*` | 租户管理 |
| GET | `/api/v1/admin/tenants/:id/quota` | 租户配额 |
| PUT | `/api/v1/admin/tenants/:id/quota/reset` | 重置租户配额 |
| GET | `/api/v1/admin/ab-eval/*` | A/B 评估 |

### 2.3 后端与存储

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/backends/types` | 已有后端类型元数据 |
| GET/POST/PUT/DELETE | `/api/v1/backends/*` | 后端配置 CRUD |
| GET | `/api/v1/backends/export` | 导出后端配置 |
| GET | `/api/v1/backends/:id/models` | 获取后端模型列表 |
| POST | `/api/v1/backends/fetch-models` | 拉取远程模型 |
| POST | `/api/v1/backends/test` | 连接测试 |
| POST | `/api/v1/backends/:id/probe` | 后端探测 |
| POST | `/api/v1/backends/probe-all` | 全量探测 |
| POST | `/api/v1/backends/probe-all-sse` | 全量探测（SSE） |
| GET | `/api/v1/backends/circuit-breaker` | 熔断器状态 |
| POST | `/api/v1/backends/circuit-breaker/:id/reset` | 重置熔断器 |
| GET/POST/PUT/DELETE | `/api/v1/storage/*` | 存储配置管理 |
| POST | `/api/v1/storage/test` | 存储连接测试 |
| POST | `/api/v1/storage/connect` | 连接存储 |
| POST | `/api/v1/storage/disconnect` | 断开存储 |
| POST | `/api/v1/storage/set-default` | 设为默认存储 |

### 2.4 流水线与插件

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST/PUT/DELETE | `/api/v1/pipelines/*` | 流水线 CRUD |
| GET | `/api/v1/pipelines/templates` | 流水线模板 |
| GET | `/api/v1/pipelines/node-plugins` | 节点插件列表 |
| POST | `/api/v1/pipelines/node-plugins/:implementation/test` | 测试节点插件 |
| POST | `/api/v1/pipelines/node-plugins/discover` | 发现远程插件 |
| GET | `/api/v1/pipelines/plugin-metrics` | 插件指标 |
| POST | `/api/v1/pipelines/execute-direct` | 直接执行流水线 |
| POST | `/api/v1/pipelines/:id/execute` | 执行流水线 |
| POST | `/api/v1/pipelines/:id/validate` | 校验流水线 |
| POST | `/api/v1/pipelines/:id/auto-build` | 自动构建 |
| POST | `/api/v1/pipelines/:id/auto-build/rollback` | 回滚自动构建 |
| GET | `/api/v1/pipelines/:id/export` | 导出流水线 |
| GET | `/api/v1/pipelines/:id/executions` | 执行历史 |
| PUT | `/api/v1/pipelines/:id/nodes/:nodeId/config` | 更新节点配置 |
| GET/POST/PUT/DELETE | `/api/v1/pipelines/plugin-registry/*` | 插件注册表 |
| GET/PUT | `/api/v1/pipeline/defaults` | 默认流水线配置 |
| GET/PUT | `/api/v1/plugins/*` | 插件管理 |
| POST/GET/DELETE | `/api/v1/registry/*` | 插件市场 |
| POST | `/api/v1/webhooks/pipeline/:id` | Webhook 触发流水线 |

### 2.5 缓存与策略

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/cache/stats` | 缓存统计 |
| POST/DELETE | `/api/v1/cache/clear` | 清空缓存 |
| DELETE | `/api/v1/cache/invalidate/:key` | 失效指定缓存 |
| POST | `/api/v1/cache/enable` | 启用/禁用缓存 |
| GET | `/api/v1/cache/enabled` | 查询缓存状态 |
| POST | `/api/v1/cache/ttl` | 设置 TTL |
| POST | `/api/v1/cache/check` | 查询缓存 |
| POST | `/api/v1/cache/info` | 缓存详情 |
| DELETE | `/api/v1/cache/entry` | 删除缓存条目 |
| POST | `/api/v1/cache/warmup` | 缓存预热 |
| GET | `/api/v1/cache/list` | 列出缓存条目 |
| GET/POST | `/api/v1/cache/semantic/threshold` | 语义缓存阈值 |
| POST | `/api/v1/cache/semantic/search` | 语义搜索 |
| POST | `/api/v1/cache/generate-key` | 生成缓存 Key |
| GET/POST | `/api/v1/cache/qa-split/*` | QA 拆分配置 |
| GET/POST/PUT/DELETE | `/api/v1/strategies/*` | 匹配策略 |
| GET/POST/PUT | `/api/v1/evaluation/*` | 缓存评估插件 |

### 2.6 代理模式

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST/PUT/DELETE | `/api/v1/proxy-modes/*` | 代理模式 CRUD |
| POST | `/api/v1/proxy-modes/:key/enable` | 启用/禁用模式 |
| GET/POST/DELETE | `/api/v1/session/proxy-mode` | 会话级代理模式 |

### 2.7 监控、日志与系统

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/monitor/stats` | 系统统计 |
| GET | `/api/v1/monitor/cache` | 缓存统计 |
| GET | `/api/v1/monitor/dashboard` | 仪表盘数据 |
| GET | `/api/v1/monitor/request` | 请求统计 |
| GET | `/api/v1/monitor/route-backend` | 路由后端统计 |
| GET | `/api/v1/monitor/plugins` | 插件状态 |
| GET | `/api/v1/monitor/config` | 配置信息 |
| POST | `/api/v1/monitor/reset` | 重置统计 |
| GET | `/api/v1/logs` | 日志查询 |
| GET | `/api/v1/logs/stats` | 日志统计 |
| POST | `/api/v1/logs/export` | 导出日志 |
| GET | `/api/v1/logs/stream` | 日志流（SSE） |
| GET | `/api/v1/logs/tail` | 最新日志 |
| POST | `/api/v1/logs/clear` | 清空日志 |
| GET | `/api/v1/traces/:request_id` | 请求追踪 |
| GET/PUT | `/api/v1/config` | 系统配置 |
| POST | `/api/v1/system/update` | 系统更新 |
| GET | `/api/v1/system/update/history` | 更新历史 |
| POST | `/api/v1/system/rollback` | 回滚更新 |
| GET | `/api/v1/status` | 系统状态 |
| GET | `/health` | 健康检查 |
| GET | `/health/ready` | 就绪检查 |
| GET | `/ping` | Ping |

---

## 3. 其它接口（待整理）

以下接口不属于标准代理协议，也不属于核心管理功能。标注 `⚠️ 违规` 的接口不符合 §1 约束条件，需后续处理。

### 3.1 违规接口（需迁移或移除）

| 分组 | 路径 | 端点数 | 违反条款 | 建议处理方式 |
|------|------|--------|---------|-------------|
| `⚠️` Clash 订阅 | `/api/v1/user/clash/*`, `/clash/subscribe/:token` | 9 | 垂直业务场景接口 | 迁移为插件或移至独立应用 |
| `⚠️` Agent 记忆 | `/api/v1/memory/*` | 12 | 面向 Agent 的业务专用接口 | 迁移为插件实现 |
| `⚠️` 系统代理 MITM | `/api/v1/proxy/pac`, `/api/v1/proxy/ca.crt`, `/api/v1/proxy/status`, `/api/v1/proxy/domains/*`, `/api/v1/proxy/patterns/*` | 8 | 桌面应用专属功能，非 LLM 代理核心 | 移至 `web/` 或独立模块 |
| `⚠️` 主机代理 | `/api/v1/host-proxy/*` | 4 | 桌面应用专属功能，非 LLM 代理核心 | 移至 `web/` 或独立模块 |

### 3.2 待确认接口（定位需讨论）

| 分组 | 路径 | 端点数 | 说明 | 待确认事项 |
|------|------|--------|------|-----------|
| Agent 快速配置 | `/api/v1/agent/*` | 5 | Agent 工具配置生成 | 便利功能，可否移至 WebUI 前端实现 |
| Agent 供应商配置 | `/api/v1/agent-providers/*` | 7 | Agent 供应商热切换 | 管理范畴？业务范畴？ |
| 静态资源 | `/`, `/static/*` | - | WebUI SPA | 内置前端，保留 |
