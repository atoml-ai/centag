# 多租户 API 文档

> 版本: v1.0
> 最后更新: 2026-06-02

---

## 认证

所有 API 需要认证：

- **管理员 API**: JWT Bearer Token + Role = admin
- **用户 API**: JWT Bearer Token 或 API Key (`llmproxy_*`)

```http
Authorization: Bearer <token>
# 或
Authorization: Bearer llmproxy_xxxxxxxx
```

---

## 用户端点（任意认证用户）

### GET /api/v1/user/tenant

获取当前用户所属租户信息。

**响应**:

```json
{
  "id": "t_42_1717305600",
  "name": "alice's workspace",
  "description": "Auto-created tenant for user alice",
  "status": "active",
  "created_at": "2026-06-01T12:00:00Z",
  "updated_at": "2026-06-01T12:00:00Z",
  "metadata": {
    "auto_created": "true"
  }
}
```

**错误**:

- `401 Unauthorized`: 未认证
- `404 Not Found`: 用户无租户（需联系管理员）

---

### GET /api/v1/user/tenant/quota

获取当前租户配额和使用量。

**响应**:

```json
{
  "tenant_id": "t_42_1717305600",
  "daily_token_limit": 1000000,
  "monthly_token_limit": 10000000,
  "daily_request_limit": 10000,
  "monthly_request_limit": 100000,
  "max_backends": 10,
  "max_pipelines": 20,
  "used_today_tokens": 15000,
  "used_month_tokens": 250000,
  "used_today_requests": 120,
  "used_month_requests": 1800
}
```

---

## 管理员端点（Admin Only）

### GET /api/v1/admin/tenants

列出所有租户（支持分页）。

**查询参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页数量，默认 20 |
| status | string | 否 | 过滤状态: active, suspended |

**响应**:

```json
{
  "total": 42,
  "page": 1,
  "page_size": 20,
  "tenants": [
    {
      "id": "t_1_1717305600",
      "name": "alice's workspace",
      "status": "active",
      "user_id": 1,
      "username": "alice",
      "created_at": "2026-06-01T12:00:00Z"
    }
  ]
}
```

---

### GET /api/v1/admin/tenants/:id

获取租户详情。

**路径参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 租户 ID |

**响应**:

```json
{
  "id": "t_42_1717305600",
  "name": "alice's workspace",
  "description": "Auto-created tenant for user alice",
  "status": "active",
  "user_id": 42,
  "username": "alice",
  "quota": {
    "daily_token_limit": 1000000,
    "monthly_token_limit": 10000000,
    "daily_request_limit": 10000,
    "monthly_request_limit": 100000,
    "max_backends": 10,
    "max_pipelines": 20
  },
  "usage": {
    "used_today_tokens": 15000,
    "used_month_tokens": 250000,
    "used_today_requests": 120,
    "used_month_requests": 1800
  },
  "created_at": "2026-06-01T12:00:00Z",
  "updated_at": "2026-06-01T12:00:00Z"
}
```

---

### PUT /api/v1/admin/tenants/:id

更新租户信息（名称、描述、状态）。

**请求体**:

```json
{
  "name": "alice's new workspace",
  "description": "Updated description",
  "status": "active"
}
```

**响应**: 更新后的租户对象

---

### PUT /api/v1/admin/tenants/:id/quota

更新租户配额。

**请求体**:

```json
{
  "daily_token_limit": 2000000,
  "monthly_token_limit": 20000000,
  "daily_request_limit": 20000,
  "monthly_request_limit": 200000,
  "max_backends": 20,
  "max_pipelines": 40
}
```

**响应**: 更新后的配额对象

**注意**: 设置为 `0` 表示无限制。

---

### PUT /api/v1/admin/tenants/:id/quota/reset

重置租户用量统计（用于月初或手动清零）。

**响应**:

```json
{
  "message": "Quota reset successfully",
  "tenant_id": "t_42_1717305600",
  "reset_at": "2026-06-02T10:00:00Z"
}
```

---

### DELETE /api/v1/admin/tenants/:id

删除租户（谨慎操作）。

**警告**: 删除租户会级联删除：
- 租户专属后端配置
- 租户专属流水线配置
- 租户 API Keys
- 租户用量统计

系统预设资源不受影响。

**响应**:

```json
{
  "message": "Tenant deleted successfully",
  "tenant_id": "t_42_1717305600"
}
```

---

## 错误响应格式

所有错误返回统一格式：

```json
{
  "error": {
    "code": "quota_exceeded",
    "message": "daily token quota exceeded",
    "details": {
      "tenant_id": "t_42_1717305600",
      "limit": 1000000,
      "used": 1000000
    }
  }
}
```

**错误码列表**:

| 错误码 | HTTP 状态码 | 说明 |
|--------|-------------|------|
| `unauthorized` | 401 | 未认证或认证失败 |
| `forbidden` | 403 | 无权访问（非管理员） |
| `not_found` | 404 | 租户不存在 |
| `quota_exceeded` | 429 | 配额超限 |
| `invalid_request` | 400 | 请求参数错误 |
| `internal_error` | 500 | 服务器内部错误 |

---

## 数据模型

### Tenant

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 租户唯一标识（格式: `t_{user_id}_{timestamp}`） |
| name | string | 租户名称 |
| description | string | 租户描述 |
| status | string | 状态: `active`, `suspended` |
| user_id | int64 | 关联用户 ID |
| username | string | 关联用户名 |
| quota | ResourceQuota | 资源配额 |
| usage | TenantUsage | 使用量统计 |
| created_at | string (ISO 8601) | 创建时间 |
| updated_at | string (ISO 8601) | 更新时间 |

### ResourceQuota

| 字段 | 类型 | 说明 |
|------|------|------|
| daily_token_limit | int64 | 日 Token 限额（0 = 无限制） |
| monthly_token_limit | int64 | 月 Token 限额 |
| daily_request_limit | int64 | 日请求限额 |
| monthly_request_limit | int64 | 月请求限额 |
| max_backends | int | 最大后端数 |
| max_pipelines | int | 最大流水线数 |

### TenantUsage

| 字段 | 类型 | 说明 |
|------|------|------|
| used_today_tokens | int64 | 今日已用 Token |
| used_month_tokens | int64 | 本月已用 Token |
| used_today_requests | int64 | 今日已用请求数 |
| used_month_requests | int64 | 本月已用请求数 |

---

## 调用示例

### cURL

```bash
# 获取我的租户信息
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/user/tenant

# 获取租户配额
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/user/tenant/quota

# 管理员：列出所有租户
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://localhost:8080/api/v1/admin/tenants?page=1&page_size=20

# 管理员：更新配额
curl -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"daily_token_limit": 5000000}' \
  http://localhost:8080/api/v1/admin/tenants/t_42_1717305600/quota

# 管理员：重置用量
curl -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://localhost:8080/api/v1/admin/tenants/t_42_1717305600/quota/reset
```

### Python

```python
import requests

headers = {"Authorization": f"Bearer {token}"}

# 获取租户信息
resp = requests.get("http://localhost:8080/api/v1/user/tenant", headers=headers)
tenant = resp.json()
print(f"Tenant: {tenant['name']}, Status: {tenant['status']}")

# 获取配额
resp = requests.get("http://localhost:8080/api/v1/user/tenant/quota", headers=headers)
quota = resp.json()
print(f"Token usage: {quota['used_today_tokens']}/{quota['daily_token_limit']}")
```

---

## 实际状态 vs 文档差异（2026-06-13）

> v1.0 API 文档偏理想化。以下为实际测试确认的状态。

| 文档承诺 | 实际状态 |
|----------|:--------:|
| `GET /api/v1/user/tenant` 返回 `created_at` | ⚠️ 返回 `0001-01-01T00:00:00Z`（零值） |
| 配额超额返回 429 | ❌ 配额中间件未接入 |
| `username` 字段包含在响应中 | ❌ `tenantResponse` 无 `username` 字段 |
| 分页参数 `page`/`page_size` | ❌ 当前 `ListTenants` 未分页 |
| `max_pipelines` 字段 | ❌ 模型无此字段 |
| 错误格式 `{"error":{"code"..}}` | ❌ 使用 `{"success":false,"error":"..."}` |

> 修复方案见[多租户治理与增强技术方案](../exec-plans/completed/2026-06-13-multi-tenant-enhancement.md)。

### 📘 JavaScript

```javascript
// 获取租户信息
const resp = await fetch('/api/v1/user/tenant', {
  headers: { 'Authorization': `Bearer ${token}` }
});
const tenant = await resp.json();
console.log(`Tenant: ${tenant.name}`);

// 获取配额
const quotaResp = await fetch('/api/v1/user/tenant/quota', {
  headers: { 'Authorization': `Bearer ${token}` }
});
const quota = await quotaResp.json();
const usagePercent = (quota.used_today_tokens / quota.daily_token_limit * 100).toFixed(1);
console.log(`Token usage: ${usagePercent}%`);
```
