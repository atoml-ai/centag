# Centag 多租户用户指南

> 版本: v1.0  
> 适用版本: Centag ≥ v2.5  
> 最后更新: 2026-06-02

---

## 1. 什么是多租户？

多租户（Multi-Tenancy）是 Centag 的基础设施层能力，确保每个用户拥有**完全隔离**的资源空间：

- **独立后端配置**：每个用户可配置自己的 LLM API Key，互不泄露
- **独立流水线**：每个用户可创建自定义处理流水线
- **独立配额**：管理员可为每个用户设置资源使用上限
- **独立用量统计**：按用户聚合 Token 消耗和请求次数

**核心原则**：一用户 = 一租户，严格隔离，无共享租户。

**Web 导航（team）**：
- **超管**：用户/租户/限额、共用后端与共用策略、**存储配置**、系统配置、计费规则；首页流水线可「测试」打开对话抽屉。不含独立对话菜单、本机代理、记忆查询、业务用量页（直链踢回概览）。用户 API Key 在「用户管理」内查看/创建。
- **普通用户**：与 **personal 同源**（首页 / 用量 / 本机代理 / 记忆 / 更多）。后端与策略的主入口在**首页**（无侧栏「配置→后端/策略」）；**无独立对话菜单**（对话仅流水线「测试」抽屉）。「更多 → 系统」为「我的租户」；**无存储配置**、**不可见计费规则**。记忆为查询模式（无同步/重建索引）。

**用户资源配置（超管 → 用户管理 → 资源配置）**：
- Token 日/月限额（`users` 表，代理侧强制）
- 默认流水线（`default_pipeline_id`）
- 租户请求限额 / 最大后端数 / 最大 API Key 数
- **可用共用资源白名单**（空列表 = 不可用任何共用资源，只能靠自建）：
  - `allowed_backend_ids` / `allowed_model_ids`（双重筛选：模型须属于已选共用后端）
  - `allowed_pipeline_ids`
  - 仅可勾选当前 **enabled** 的共用后端/模型；运行时禁用则自动不可用
- **自建与默认流水线开关**（默认均为开启）：
  - `can_add_own_backends`：可否添加/改/删自有后端（自带密钥）
  - `can_add_own_pipelines`：可否添加/改/删自有流水线
  - `can_change_default_pipeline`：可否自行修改默认流水线（超管仍可代设）

---

## 2. 快速开始

### 2.1 启用多租户

多租户由 **产品版本（Edition）** 控制，真源环境变量为 `CENTAG_EDITION`：

| Edition | 多租户 | 典型场景 |
|---------|--------|----------|
| `team` | 启用 | 团队/企业部署（bootstrap 默认值；Profile `team` 固定） |
| `personal` | 关闭 | 个人全功能 / Desktop；**发行包 `personal` 固定为此值** |
| `minimal` | 关闭 | 精简文件配置版（无 DB） |

**个人版（personal）**：

```bash
# config/profiles/personal 已内置
CENTAG_EDITION=personal
LLM_PROXY_DB_DRIVER=sqlite
```

**团队版**：

```bash
export CENTAG_EDITION=team
```

或使用 [`config/profiles/team/`](../../config/profiles/team/) Profile 启动（已内置 `CENTAG_EDITION=team` + PostgreSQL stack）。

配额、租户隔离、管理员 API 在 `team` edition 下自动生效；无需再设置已废弃的 `LLM_PROXY_MULTI_TENANT_ENABLED`。

### 2.2 新用户注册流程

启用多租户后，新用户注册时**自动**完成：

1. 创建用户账号
2. 创建独立租户（tenant_id = user_id）
3. 复制系统预设后端到用户空间
4. 复制系统预设流水线到用户空间
5. 创建默认 API Key
6. 设置默认资源配额

用户首次登录即可看到自己的后端和流水线配置。

### 2.3 单用户模式升级

现有单用户部署**零配置升级**：

```bash
# 1. 更新到最新版本
git pull origin main

# 2. 运行数据库迁移（自动为现有用户创建租户）
go run cmd/migrate/main.go

# 3. 启动服务
./centag
```

现有用户的后端配置自动标记为系统预设，所有用户可见但不可修改。

---

## 3. 用户操作指南

### 3.1 查看我的租户信息

登录 WebUI → 功能中心 → **我的租户**：

- 租户 ID（一键复制）
- 今日 Token 使用量 / 限额
- 今日请求次数 / 限额
- 资源限制（后端数、流水线数、存储空间）

### 3.2 管理后端配置

**系统预设后端**（管理员配置）：
- 所有用户可见
- 用户**不可修改**（保护管理员配置的 API Key）
- 用户**可选择使用**

**用户专属后端**：
- 仅当前用户可见
- 用户**可自由增删改**
- 支持 OpenAI、Anthropic、Ollama 等所有类型

**添加后端步骤**：
1. WebUI → 后端管理 → 新增后端
2. 填写后端名称、类型、Base URL
3. 输入**自己的 API Key**（安全存储，他人不可见）
4. 选择支持的模型
5. 保存

### 3.3 管理流水线

**系统预设流水线**（管理员配置）：
- 所有用户可用
- 用户**不可修改**

**用户专属流水线**：
- 仅当前用户可见
- 用户**可自由创建**

**创建流水线步骤**：
1. WebUI → 流水线管理 → 新建流水线
2. 选择节点类型（生成器、处理器、路由器等）
3. 配置节点参数
4. 保存并测试

### 3.4 查看用量统计

WebUI → 我的租户 → 用量统计：

| 指标 | 说明 |
|------|------|
| 今日 Token | 当天消耗的 Token 数 |
| 本月 Token | 当月累计 Token 数 |
| 今日请求 | 当天请求次数 |
| 本月请求 | 当月累计请求次数 |
| 限额使用率 | 进度条可视化 |

---

## 4. 管理员操作指南

### 4.1 租户管理

WebUI → 系统管理 → **租户管理**（需管理员权限）：

- 查看所有租户列表
- 查看租户详情（配额、用量、状态）
- 编辑租户配额
- 重置租户用量
- 禁用/启用租户

### 4.2 设置租户配额

**配额维度**：

| 配额项 | 说明 | 默认值 |
|--------|------|--------|
| 日 Token 限额 | 每天最多消耗 Token 数 | 1,000,000 |
| 月 Token 限额 | 每月最多消耗 Token 数 | 10,000,000 |
| 日请求限额 | 每天最多请求次数 | 10,000 |
| 月请求限额 | 每月最多请求次数 | 100,000 |
| 最大后端数 | 用户可配置的后端数量上限 | 10 |
| 最大流水线数 | 用户可创建的流水线数量上限 | 20 |

**配额超限行为**：
- 返回 HTTP 429（Too Many Requests）
- 响应体包含 `{"error": {"type": "quota_exceeded", "message": "..."}}`
- 用户可联系管理员调整配额

### 4.3 系统预设管理

管理员配置的系统预设对所有用户生效：

**后端预设**：
```bash
# 添加系统预设后端（所有用户可见）
curl -X POST http://localhost:8080/api/v1/admin/backends \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "id": "default-openai",
    "name": "Default OpenAI",
    "type": "openai",
    "base_url": "https://api.openai.com",
    "api_key": "sk-...",
    "enabled": true
  }'
```

**流水线预设**：
```bash
# 添加系统预设流水线
curl -X POST http://localhost:8080/api/v1/admin/pipelines \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{...pipeline config...}'
```

**安全注意**：系统预设后端的 API Key 在复制到用户空间时会**自动清空**，用户需自行配置自己的 API Key。

---

## 5. 资源隔离策略

### 5.1 隔离级别矩阵

| 资源类型 | 隔离策略 | 说明 |
|----------|----------|------|
| 后端配置 | 继承 + 覆盖 | 系统预设 + 用户专属覆盖 |
| 流水线配置 | 继承 + 覆盖 | 同上 |
| API Key | 完全隔离 | 用户级 Key 仅属于该用户 |
| 记忆文档 | 完全隔离 | 按用户隔离 |
| Token 用量 | 完全隔离 | 按租户聚合统计 |
| 插件注册 | 共享 + 私有 | 市场插件共享，私有插件隔离 |

### 5.2 数据查询过滤

所有涉及租户数据的查询自动附加 `tenant_id` 过滤：

```sql
-- 后端配置查询示例
SELECT * FROM backends 
WHERE tenant_id = ? OR tenant_id = 'system'
ORDER BY priority DESC;
```

- `tenant_id = 'system'`：系统预设，所有租户可见
- `tenant_id = 'tenant-xxx'`：租户专属，仅该租户可见
- 租户专属配置**覆盖**同名系统预设

---

## 6. 故障排查

### 6.1 常见问题

**Q: 启用多租户后，现有用户看不到自己的后端？**

A: 运行数据库迁移脚本，为现有用户自动创建租户：
```bash
go run cmd/migrate/main.go
```

**Q: 用户报告 429 配额超限？**

A: 检查用户配额设置，或临时重置用量：
```bash
curl -X PUT http://localhost:8080/api/v1/admin/tenants/{tenant_id}/quota/reset \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Q: 系统预设后端的 API Key 泄露？**

A: 系统预设后端复制到用户空间时会**自动清空 API Key**，这是安全设计。用户需自行配置自己的 API Key。

**Q: 如何关闭多租户回到单用户模式？**

A: 切换到 `personal` edition 并重启：
```bash
export CENTAG_EDITION=personal
```

Desktop 侧车默认即为 `personal`。关闭后用户管理、租户隔离与团队版专属导航将不可用。

### 6.2 日志排查

启用调试日志查看租户上下文：

```bash
export LLM_PROXY_LOG_LEVEL=debug
```

关键日志字段：
- `tenant_id`：当前请求归属的租户
- `user_id`：当前请求的用户 ID
- `backend_id`：选中的后端
- `quota_check`：配额检查结果

---

## 7. 性能影响

多租户功能对性能的影响：

| 场景 | 开销 | 说明 |
|------|------|------|
| 单用户模式 | 零开销 | `tenant_id == ""` 时跳过所有租户逻辑 |
| 多租户 - 内存缓存命中 | < 1μs | 配额检查使用内存窗口缓存 |
| 多租户 - 内存缓存未命中 | ~5ms | 首次查询数据库获取配额 |
| 后端选择 | 无额外开销 | 租户过滤在内存中完成 |

**优化建议**：
- 生产环境建议使用 PostgreSQL（>50 用户时）
- 配额窗口缓存自动刷新，无需手动干预

---

## 参考文档

- [多租户治理与增强技术方案](../exec-plans/completed/2026-06-13-multi-tenant-enhancement.md)（缺陷追踪与修复路线图）
- [API 文档](../api/tenant.md)
- [架构设计](../../docs/harness/ARCHITECTURE.md)
- [部署指南](DEPLOYMENT_GUIDE.md)

---

## 已知问题（v1.0）

> 以下为 2026-06-13 测试发现的问题，详见[技术方案](../exec-plans/completed/2026-06-13-multi-tenant-enhancement.md)。

| 问题 | 影响 | 状态 |
|------|------|:----:|
| Admin 无法查看租户专属流水线 | 管理后台看不到用户创建的流水线 | 🔴 待修复 |
| API Key 未绑定 tenant_id | 使用 API Key 代理请求时丢失租户上下文 | 🔴 待修复 |
| 配额未集成代理中间件 | 使用量不更新，无熔断 | 🔴 待修复 |
| CreatedAt 字段为零值 | 租户创建时间不可见 | 🟡 待修复 |
| 内置流水线模板为空 | 新用户首次使用无模板参考 | 🟡 待修复 |
