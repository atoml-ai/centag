# Centag Admin E2E Test — 正本

> **业务正本**（`docs/harness/skills/`）。交互入口在 `.cursor/rules/centag-admin-e2e.mdc`、`.opencode/skills/centag-admin-e2e/` 等。分层见根目录 **`AGENT.md`**。
> 覆盖管理功能端到端测试。按 `TEST_DEPLOY_TYPE`（personal / team / minimal）裁剪；Agent 供应商默认跳过。
>
> ## 原则
> - **只验证 HTTP 状态码 + 响应内容**，不分析代码
> - 失败则记录并继续，最终生成报告
> - 测试数据在测试结束后清理

---

## 前置条件

| 参数 | 环境变量 | 必需 | 说明 |
|------|----------|------|------|
| 服务地址 | `TEST_BASE_URL` | ✅ | 如 `http://localhost:20060` |
| JWT Token | `TEST_JWT_TOKEN` | ✅ | 通过 POST /api/auth/login 获取 |
| Admin 用户名 | `ADMIN_USERNAME` | ✅ | 用于登录 |
| Admin 密码 | `ADMIN_PASSWORD` | ✅ | 用于登录 |
| 部署类型 | `TEST_DEPLOY_TYPE` | ✅ | `personal` / `team` / `minimal` |

---

## Edition 裁剪

| 模块 | personal | team | minimal |
|------|--------------------|------|---------|
| 用户管理 | ✅ | ✅ | ❌ 跳过 |
| `/api/v1/user/apikeys` | ✅ | ✅ | ❌ 改测 `/api/v1/settings/api-keys` |
| 多租户 / 成本 | ❌ | ✅ | ❌ |
| Token 用量 / Profile / 系统配置 | ✅ | ✅ | ❌（minimal 无等价或未启用） |
| Agent 供应商 | 可选（默认跳过 lean） | 可选 | ❌ |
| 后端 / 流水线 / 健康检查 | ✅ | ✅ | ✅ |
| 改密 | — | — | ✅ `POST /api/v1/settings/password`（仅探测可达性，不强制改密） |

设置 `TEST_SKIP_AGENT_PROVIDERS=true`（默认）跳过 Agent 供应商用例。

## 推荐执行方式（预置脚本）


为减少人为步骤与登录阻塞，推荐直接执行：

```bash
bash docs/harness/skills/admin-e2e-test.sh
python3 docs/harness/skills/wizard-report.py
```

脚本行为：

- 自动读取 `config/secrets/.env`（若存在）
- 自动凭据优先级：
  - 用户名：`ADMIN_USERNAME` → `LLM_PROXY_ADMIN_USERNAME` → `admin`
  - 密码：`ADMIN_PASSWORD` → `LLM_PROXY_ADMIN_PASSWORD`
- 优先复用 `TEST_JWT_TOKEN`，无效时自动登录刷新
- 产出 `/tmp/admin_e2e_results.json`，供 HTML 报告统一渲染
- 每条用例保存可审计证据：测试数据、请求 `curl`、请求体、响应头、响应体、断言表达式、断言结果与判定结论

---

## 测试流程

### Step 1: 获取认证 Token

```bash
# 登录获取 JWT
LOGIN_RESP=$(curl -s -X POST "$TEST_BASE_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$ADMIN_USERNAME\", \"password\": \"$ADMIN_PASSWORD\"}")

JWT_TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.access_token // empty')
if [ -z "$JWT_TOKEN" ]; then
  echo "❌ 登录失败: $LOGIN_RESP"
  exit 1
fi
echo "✅ 登录成功"

AUTH_HEADER="Authorization: Bearer $JWT_TOKEN"
```

---

### Step 2: 用户管理测试

```bash
echo "=== 用户管理测试 ==="

# 2.1 列出用户
echo "--- 列出用户 ---"
LIST_USERS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/admin/users")
HTTP_CODE=$(echo "$LIST_USERS" | tail -1)
BODY=$(echo "$LIST_USERS" | sed '$d')
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 列出用户: HTTP 200"
else
  echo "❌ 列出用户: HTTP $HTTP_CODE"
fi

# 2.2 创建用户
echo "--- 创建用户 ---"
TEST_USERNAME="e2e_test_user_$(date +%s)"
CREATE_USER=$(curl -s -w "\n%{http_code}" \
  -X POST "$TEST_BASE_URL/api/v1/admin/users" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$TEST_USERNAME\", \"password\": \"Test123456\", \"role\": \"normal\"}")
HTTP_CODE=$(echo "$CREATE_USER" | tail -1)
BODY=$(echo "$CREATE_USER" | sed '$d')
if [ "$HTTP_CODE" = "201" ]; then
  NEW_USER_ID=$(echo "$BODY" | jq -r '.data.id')
  echo "✅ 创建用户: HTTP 201, user_id=$NEW_USER_ID"
else
  echo "❌ 创建用户: HTTP $HTTP_CODE"
fi

# 2.3 创建重复用户（应返回 409）
echo "--- 创建重复用户 ---"
DUP_USER=$(curl -s -w "\n%{http_code}" \
  -X POST "$TEST_BASE_URL/api/v1/admin/users" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$TEST_USERNAME\", \"password\": \"Test123456\", \"role\": \"normal\"}")
HTTP_CODE=$(echo "$DUP_USER" | tail -1)
if [ "$HTTP_CODE" = "409" ]; then
  echo "✅ 重复用户检测: HTTP 409 (预期)"
else
  echo "⚠️ 重复用户检测: HTTP $HTTP_CODE (预期 409)"
fi

# 2.4 更新用户
echo "--- 更新用户 ---"
if [ -n "$NEW_USER_ID" ]; then
  UPDATE_USER=$(curl -s -w "\n%{http_code}" \
    -X PUT "$TEST_BASE_URL/api/v1/admin/users/$NEW_USER_ID" \
    -H "$AUTH_HEADER" \
    -H "Content-Type: application/json" \
    -d '{"display_name": "E2E Test User"}')
  HTTP_CODE=$(echo "$UPDATE_USER" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 更新用户: HTTP 200"
  else
    echo "❌ 更新用户: HTTP $HTTP_CODE"
  fi
fi

# 2.5 删除用户
echo "--- 删除用户 ---"
if [ -n "$NEW_USER_ID" ]; then
  DELETE_USER=$(curl -s -w "\n%{http_code}" \
    -X DELETE "$TEST_BASE_URL/api/v1/admin/users/$NEW_USER_ID" \
    -H "$AUTH_HEADER")
  HTTP_CODE=$(echo "$DELETE_USER" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 删除用户: HTTP 200"
  else
    echo "❌ 删除用户: HTTP $HTTP_CODE"
  fi
fi
```

---

### Step 3: API Key 管理测试

```bash
echo "=== API Key 管理测试 ==="

# 3.1 列出当前用户 API Keys
echo "--- 列出 API Keys ---"
LIST_KEYS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/user/apikeys")
HTTP_CODE=$(echo "$LIST_KEYS" | tail -1)
BODY=$(echo "$LIST_KEYS" | sed '$d')
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 列出 API Keys: HTTP 200"
else
  echo "❌ 列出 API Keys: HTTP $HTTP_CODE"
fi

# 3.2 创建 API Key
echo "--- 创建 API Key ---"
CREATE_KEY=$(curl -s -w "\n%{http_code}" \
  -X POST "$TEST_BASE_URL/api/v1/user/apikeys" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"name": "e2e-test-key"}')
HTTP_CODE=$(echo "$CREATE_KEY" | tail -1)
BODY=$(echo "$CREATE_KEY" | sed '$d')
if [ "$HTTP_CODE" = "201" ]; then
  NEW_KEY_ID=$(echo "$BODY" | jq -r '.data.id')
  echo "✅ 创建 API Key: HTTP 201, key_id=$NEW_KEY_ID"
else
  echo "❌ 创建 API Key: HTTP $HTTP_CODE"
fi

# 3.3 获取单个 API Key
echo "--- 获取 API Key ---"
if [ -n "$NEW_KEY_ID" ]; then
  GET_KEY=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/user/apikeys/$NEW_KEY_ID")
  HTTP_CODE=$(echo "$GET_KEY" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 获取 API Key: HTTP 200"
  else
    echo "❌ 获取 API Key: HTTP $HTTP_CODE"
  fi
fi

# 3.4 删除 API Key
echo "--- 删除 API Key ---"
if [ -n "$NEW_KEY_ID" ]; then
  DELETE_KEY=$(curl -s -w "\n%{http_code}" \
    -X DELETE "$TEST_BASE_URL/api/v1/user/apikeys/$NEW_KEY_ID" \
    -H "$AUTH_HEADER")
  HTTP_CODE=$(echo "$DELETE_KEY" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 删除 API Key: HTTP 200"
  else
    echo "❌ 删除 API Key: HTTP $HTTP_CODE"
  fi
fi
```

---

### Step 4: 多租户管理测试（Team 版）

```bash
if [ "$TEST_DEPLOY_TYPE" = "team" ]; then
  echo "=== 多租户管理测试 ==="

  # 4.1 列出租户
  echo "--- 列出租户 ---"
  LIST_TENANTS=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/admin/tenants")
  HTTP_CODE=$(echo "$LIST_TENANTS" | tail -1)
  BODY=$(echo "$LIST_TENANTS" | sed '$d')
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 列出租户: HTTP 200"
    TENANT_COUNT=$(echo "$BODY" | jq '.data | length')
    echo "   租户数量: $TENANT_COUNT"
  else
    echo "❌ 列出租户: HTTP $HTTP_CODE"
  fi

  # 4.2 获取第一个租户详情
  echo "--- 获取租户详情 ---"
  FIRST_TENANT_ID=$(echo "$BODY" | jq -r '.data[0].id // empty')
  if [ -n "$FIRST_TENANT_ID" ]; then
    GET_TENANT=$(curl -s -w "\n%{http_code}" \
      -H "$AUTH_HEADER" \
      "$TEST_BASE_URL/api/v1/admin/tenants/$FIRST_TENANT_ID")
    HTTP_CODE=$(echo "$GET_TENANT" | tail -1)
    if [ "$HTTP_CODE" = "200" ]; then
      echo "✅ 获取租户详情: HTTP 200"
    else
      echo "❌ 获取租户详情: HTTP $HTTP_CODE"
    fi
  fi

  # 4.3 更新租户
  echo "--- 更新租户 ---"
  if [ -n "$FIRST_TENANT_ID" ]; then
    UPDATE_TENANT=$(curl -s -w "\n%{http_code}" \
      -X PUT "$TEST_BASE_URL/api/v1/admin/tenants/$FIRST_TENANT_ID" \
      -H "$AUTH_HEADER" \
      -H "Content-Type: application/json" \
      -d '{"name": "Updated Tenant Name"}')
    HTTP_CODE=$(echo "$UPDATE_TENANT" | tail -1)
    if [ "$HTTP_CODE" = "200" ]; then
      echo "✅ 更新租户: HTTP 200"
    else
      echo "❌ 更新租户: HTTP $HTTP_CODE"
    fi
  fi

  # 4.4 获取租户配额
  echo "--- 获取租户配额 ---"
  if [ -n "$FIRST_TENANT_ID" ]; then
    GET_QUOTA=$(curl -s -w "\n%{http_code}" \
      -H "$AUTH_HEADER" \
      "$TEST_BASE_URL/api/v1/admin/tenants/$FIRST_TENANT_ID/quota")
    HTTP_CODE=$(echo "$GET_QUOTA" | tail -1)
    if [ "$HTTP_CODE" = "200" ]; then
      echo "✅ 获取租户配额: HTTP 200"
    else
      echo "❌ 获取租户配额: HTTP $HTTP_CODE"
    fi
  fi

  # 4.5 更新租户配额
  echo "--- 更新租户配额 ---"
  if [ -n "$FIRST_TENANT_ID" ]; then
    UPDATE_QUOTA=$(curl -s -w "\n%{http_code}" \
      -X PUT "$TEST_BASE_URL/api/v1/admin/tenants/$FIRST_TENANT_ID/quota" \
      -H "$AUTH_HEADER" \
      -H "Content-Type: application/json" \
      -d '{"daily_token_limit": 100000, "monthly_token_limit": 3000000}')
    HTTP_CODE=$(echo "$UPDATE_QUOTA" | tail -1)
    if [ "$HTTP_CODE" = "200" ]; then
      echo "✅ 更新租户配额: HTTP 200"
    else
      echo "❌ 更新租户配额: HTTP $HTTP_CODE"
    fi
  fi

  # 4.6 重置租户配额
  echo "--- 重置租户配额 ---"
  if [ -n "$FIRST_TENANT_ID" ]; then
    RESET_QUOTA=$(curl -s -w "\n%{http_code}" \
      -X PUT "$TEST_BASE_URL/api/v1/admin/tenants/$FIRST_TENANT_ID/quota/reset" \
      -H "$AUTH_HEADER")
    HTTP_CODE=$(echo "$RESET_QUOTA" | tail -1)
    if [ "$HTTP_CODE" = "200" ]; then
      echo "✅ 重置租户配额: HTTP 200"
    else
      echo "❌ 重置租户配额: HTTP $HTTP_CODE"
    fi
  fi

  # 4.7 当前用户租户信息
  echo "--- 当前用户租户信息 ---"
  MY_TENANT=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/user/tenant")
  HTTP_CODE=$(echo "$MY_TENANT" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 获取当前租户: HTTP 200"
  else
    echo "❌ 获取当前租户: HTTP $HTTP_CODE"
  fi

  # 4.8 当前用户配额
  echo "--- 当前用户配额 ---"
  MY_QUOTA=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/user/tenant/quota")
  HTTP_CODE=$(echo "$MY_QUOTA" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 获取当前配额: HTTP 200"
  else
    echo "❌ 获取当前配额: HTTP $HTTP_CODE"
  fi
else
  echo "=== 跳过多租户测试（仅 Team 版）==="
fi
```

---

### Step 5: Token 用量统计测试

```bash
echo "=== Token 用量统计测试 ==="

# 5.1 用户用量
echo "--- 用户用量 ---"
USER_USAGE=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/user/token-usage")
HTTP_CODE=$(echo "$USER_USAGE" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 用户用量: HTTP 200"
else
  echo "❌ 用户用量: HTTP $HTTP_CODE"
fi

# 5.2 日用量
echo "--- 日用量 ---"
DAILY_USAGE=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/user/token-usage/daily")
HTTP_CODE=$(echo "$DAILY_USAGE" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 日用量: HTTP 200"
else
  echo "❌ 日用量: HTTP $HTTP_CODE"
fi

# 5.3 模型统计
echo "--- 模型统计 ---"
MODEL_STATS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/user/token-usage/models")
HTTP_CODE=$(echo "$MODEL_STATS" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 模型统计: HTTP 200"
else
  echo "❌ 模型统计: HTTP $HTTP_CODE"
fi

# 5.4 后端统计
echo "--- 后端统计 ---"
BACKEND_STATS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/user/token-usage/backends")
HTTP_CODE=$(echo "$BACKEND_STATS" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 后端统计: HTTP 200"
else
  echo "❌ 后端统计: HTTP $HTTP_CODE"
fi

# 5.5 全用户用量（Team 版 Admin）
if [ "$TEST_DEPLOY_TYPE" = "team" ]; then
  echo "--- 全用户用量 ---"
  ALL_USAGE=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/admin/token-usage/all")
  HTTP_CODE=$(echo "$ALL_USAGE" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 全用户用量: HTTP 200"
  else
    echo "❌ 全用户用量: HTTP $HTTP_CODE"
  fi

  echo "--- 用户排名 ---"
  RANKING=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/admin/token-usage/ranking")
  HTTP_CODE=$(echo "$RANKING" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 用户排名: HTTP 200"
  else
    echo "❌ 用户排名: HTTP $HTTP_CODE"
  fi
fi
```

---

### Step 6: 成本看板测试（Team 版）

```bash
if [ "$TEST_DEPLOY_TYPE" = "team" ]; then
  echo "=== 成本看板测试 ==="

  # 6.1 成本汇总
  echo "--- 成本汇总 ---"
  COST_SUMMARY=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/admin/cost/summary")
  HTTP_CODE=$(echo "$COST_SUMMARY" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 成本汇总: HTTP 200"
  else
    echo "❌ 成本汇总: HTTP $HTTP_CODE"
  fi

  # 6.2 按租户分组
  echo "--- 按租户分组 ---"
  COST_BY_TENANT=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/admin/cost/summary?group_by=tenant")
  HTTP_CODE=$(echo "$COST_BY_TENANT" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 按租户分组: HTTP 200"
  else
    echo "❌ 按租户分组: HTTP $HTTP_CODE"
  fi

  # 6.3 按模型分组
  echo "--- 按模型分组 ---"
  COST_BY_MODEL=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/admin/cost/summary?group_by=model")
  HTTP_CODE=$(echo "$COST_BY_MODEL" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 按模型分组: HTTP 200"
  else
    echo "❌ 按模型分组: HTTP $HTTP_CODE"
  fi

  # 6.4 按后端分组
  echo "--- 按后端分组 ---"
  COST_BY_BACKEND=$(curl -s -w "\n%{http_code}" \
    -H "$AUTH_HEADER" \
    "$TEST_BASE_URL/api/v1/admin/cost/summary?group_by=backend")
  HTTP_CODE=$(echo "$COST_BY_BACKEND" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 按后端分组: HTTP 200"
  else
    echo "❌ 按后端分组: HTTP $HTTP_CODE"
  fi
else
  echo "=== 跳过成本看板测试（仅 Team 版）==="
fi
```

---

### Step 7: Agent 供应商管理测试（可选，默认跳过）

> 当 `TEST_SKIP_AGENT_PROVIDERS=true`（默认）或 `TEST_DEPLOY_TYPE=minimal` 时整步跳过。

```bash
echo "=== Agent 供应商管理测试 ==="

# 7.1 列出 Agent 供应商
echo "--- 列出 Agent 供应商 ---"
LIST_PROVIDERS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/agent-providers")
HTTP_CODE=$(echo "$LIST_PROVIDERS" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 列出 Agent 供应商: HTTP 200"
else
  echo "❌ 列出 Agent 供应商: HTTP $HTTP_CODE"
fi

# 7.2 创建 Agent 供应商
echo "--- 创建 Agent 供应商 ---"
CREATE_PROVIDER=$(curl -s -w "\n%{http_code}" \
  -X POST "$TEST_BASE_URL/api/v1/agent-providers" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"name": "E2E Test Provider", "agent_type": "claude-code", "backend_id": "bigmodel"}')
HTTP_CODE=$(echo "$CREATE_PROVIDER" | tail -1)
BODY=$(echo "$CREATE_PROVIDER" | sed '$d')
if [ "$HTTP_CODE" = "201" ]; then
  PROVIDER_ID=$(echo "$BODY" | jq -r '.data.id')
  echo "✅ 创建 Agent 供应商: HTTP 201, id=$PROVIDER_ID"
else
  echo "❌ 创建 Agent 供应商: HTTP $HTTP_CODE"
fi

# 7.3 删除 Agent 供应商
echo "--- 删除 Agent 供应商 ---"
if [ -n "$PROVIDER_ID" ]; then
  DELETE_PROVIDER=$(curl -s -w "\n%{http_code}" \
    -X DELETE "$TEST_BASE_URL/api/v1/agent-providers/$PROVIDER_ID" \
    -H "$AUTH_HEADER")
  HTTP_CODE=$(echo "$DELETE_PROVIDER" | tail -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 删除 Agent 供应商: HTTP 200"
  else
    echo "❌ 删除 Agent 供应商: HTTP $HTTP_CODE"
  fi
fi
```

---

### Step 8: 后端管理测试

```bash
echo "=== 后端管理测试 ==="

# 8.1 列出后端
echo "--- 列出后端 ---"
LIST_BACKENDS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/backends")
HTTP_CODE=$(echo "$LIST_BACKENDS" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 列出后端: HTTP 200"
else
  echo "❌ 列出后端: HTTP $HTTP_CODE"
fi

# 8.2 获取后端详情
echo "--- 获取后端详情 ---"
GET_BACKEND=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/backends/bigmodel")
HTTP_CODE=$(echo "$GET_BACKEND" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 获取后端详情: HTTP 200"
else
  echo "❌ 获取后端详情: HTTP $HTTP_CODE"
fi

# 8.3 后端模型列表
echo "--- 后端模型列表 ---"
GET_MODELS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/backends/bigmodel/models")
HTTP_CODE=$(echo "$GET_MODELS" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 后端模型列表: HTTP 200"
else
  echo "❌ 后端模型列表: HTTP $HTTP_CODE"
fi

# 8.4 后端探测
echo "--- 后端探测 ---"
PROBE_BACKEND=$(curl -s -w "\n%{http_code}" \
  -X POST "$TEST_BASE_URL/api/v1/backends/bigmodel/probe" \
  -H "$AUTH_HEADER")
HTTP_CODE=$(echo "$PROBE_BACKEND" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 后端探测: HTTP 200"
else
  echo "❌ 后端探测: HTTP $HTTP_CODE"
fi

# 8.5 熔断器状态
echo "--- 熔断器状态 ---"
CB_STATUS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/backends/circuit-breaker")
HTTP_CODE=$(echo "$CB_STATUS" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 熔断器状态: HTTP 200"
else
  echo "❌ 熔断器状态: HTTP $HTTP_CODE"
fi
```

---

### Step 9: 流水线管理测试

```bash
echo "=== 流水线管理测试 ==="

# 9.1 列出流水线
echo "--- 列出流水线 ---"
LIST_PIPELINES=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/pipelines")
HTTP_CODE=$(echo "$LIST_PIPELINES" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 列出流水线: HTTP 200"
else
  echo "❌ 列出流水线: HTTP $HTTP_CODE"
fi

# 9.2 获取流水线模板
echo "--- 获取流水线模板 ---"
GET_TEMPLATES=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/pipelines/templates")
HTTP_CODE=$(echo "$GET_TEMPLATES" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 获取流水线模板: HTTP 200"
else
  echo "❌ 获取流水线模板: HTTP $HTTP_CODE"
fi

# 9.3 获取单个流水线
echo "--- 获取单个流水线 ---"
GET_PIPELINE=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/pipelines/direct-backend")
HTTP_CODE=$(echo "$GET_PIPELINE" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 获取流水线: HTTP 200"
else
  echo "❌ 获取流水线: HTTP $HTTP_CODE"
fi

# 9.4 验证流水线
echo "--- 验证流水线 ---"
VALIDATE_PIPELINE=$(curl -s -w "\n%{http_code}" \
  -X POST "$TEST_BASE_URL/api/v1/pipelines/direct-backend/validate" \
  -H "$AUTH_HEADER")
HTTP_CODE=$(echo "$VALIDATE_PIPELINE" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 验证流水线: HTTP 200"
else
  echo "❌ 验证流水线: HTTP $HTTP_CODE"
fi
```

---

### Step 10: 系统配置测试

```bash
echo "=== 系统配置测试 ==="

# 10.1 获取配置
echo "--- 获取配置 ---"
GET_CONFIG=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/config")
HTTP_CODE=$(echo "$GET_CONFIG" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 获取配置: HTTP 200"
else
  echo "❌ 获取配置: HTTP $HTTP_CODE"
fi

# 10.2 监控统计
echo "--- 监控统计 ---"
MONITOR_STATS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/monitor/stats")
HTTP_CODE=$(echo "$MONITOR_STATS" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 监控统计: HTTP 200"
else
  echo "❌ 监控统计: HTTP $HTTP_CODE"
fi

# 10.3 Dashboard 统计
echo "--- Dashboard 统计 ---"
DASHBOARD=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/monitor/dashboard")
HTTP_CODE=$(echo "$DASHBOARD" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ Dashboard 统计: HTTP 200"
else
  echo "❌ Dashboard 统计: HTTP $HTTP_CODE"
fi

# 10.4 日志查看
echo "--- 日志查看 ---"
LOGS=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/logs?limit=10")
HTTP_CODE=$(echo "$LOGS" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 日志查看: HTTP 200"
else
  echo "❌ 日志查看: HTTP $HTTP_CODE"
fi
```

---

### Step 11: 用户 Profile 测试

```bash
echo "=== 用户 Profile 测试 ==="

# 11.1 获取 Profile
echo "--- 获取 Profile ---"
GET_PROFILE=$(curl -s -w "\n%{http_code}" \
  -H "$AUTH_HEADER" \
  "$TEST_BASE_URL/api/v1/user/profile")
HTTP_CODE=$(echo "$GET_PROFILE" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 获取 Profile: HTTP 200"
else
  echo "❌ 获取 Profile: HTTP $HTTP_CODE"
fi

# 11.2 更新 Profile
echo "--- 更新 Profile ---"
UPDATE_PROFILE=$(curl -s -w "\n%{http_code}" \
  -X PUT "$TEST_BASE_URL/api/v1/user/profile" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"display_name": "E2E Admin User"}')
HTTP_CODE=$(echo "$UPDATE_PROFILE" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 更新 Profile: HTTP 200"
else
  echo "❌ 更新 Profile: HTTP $HTTP_CODE"
fi

# 11.3 修改密码
echo "--- 修改密码（跳过，避免锁定）---"
echo "⏭️ 跳过修改密码测试"
```

---

### Step 12: 健康检查

```bash
echo "=== 健康检查 ==="

# 12.1 健康检查
echo "--- 健康检查 ---"
HEALTH=$(curl -s -w "\n%{http_code}" "$TEST_BASE_URL/health")
HTTP_CODE=$(echo "$HEALTH" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 健康检查: HTTP 200"
else
  echo "❌ 健康检查: HTTP $HTTP_CODE"
fi

# 12.2 就绪检查
echo "--- 就绪检查 ---"
READY=$(curl -s -w "\n%{http_code}" "$TEST_BASE_URL/health/ready")
HTTP_CODE=$(echo "$READY" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 就绪检查: HTTP 200"
else
  echo "❌ 就绪检查: HTTP $HTTP_CODE"
fi
```

---


### Step M: Minimal 精简套件（仅 `TEST_DEPLOY_TYPE=minimal`）

当部署类型为 `minimal` 时，**跳过 Step 2–7、10–11**，仅执行：

1. Step 1 登录（`POST /api/auth/login`，用户名 `admin`）
2. `GET /api/v1/auth/bootstrap-status` → 200
3. `GET /api/v1/backends` → 200
4. `GET /api/v1/pipelines` → 200
5. `GET /api/v1/settings/api-keys` → 200（或 `api-keys/status`）
6. `GET /health` → 200 且 `edition=minimal`
7. （可选）`POST /api/v1/settings/password` 仅在用户明确要求时执行，避免破坏环境

推荐仍走预置脚本：`TEST_DEPLOY_TYPE=minimal bash docs/harness/skills/admin-e2e-test.sh`

---
## 测试报告模板

```bash
echo ""
echo "=========================================="
echo "       Centag Admin E2E 测试报告"
echo "=========================================="
echo "测试时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "部署类型: $TEST_DEPLOY_TYPE"
echo "服务地址: $TEST_BASE_URL"
echo ""
echo "测试结果汇总:"
echo "  - 用户管理: $USER_MGMT_RESULT"
echo "  - API Key: $APIKEY_RESULT"
echo "  - 多租户: $TENANT_RESULT"
echo "  - Token 用量: $TOKEN_USAGE_RESULT"
echo "  - 成本看板: $COST_RESULT"
echo "  - Agent 供应商: $AGENT_PROVIDER_RESULT"
echo "  - 后端管理: $BACKEND_RESULT"
echo "  - 流水线管理: $PIPELINE_RESULT"
echo "  - 系统配置: $CONFIG_RESULT"
echo "  - 用户 Profile: $PROFILE_RESULT"
echo "  - 健康检查: $HEALTH_RESULT"
echo "=========================================="
```

---

## 常见问题

| 症状 | 原因 | 解决 |
|------|------|------|
| 401 Unauthorized | JWT 过期 | 重新登录获取 token |
| 403 Forbidden | 非 Admin 角色 | 确保使用 Admin 账号 |
| 409 Conflict | 资源已存在 | 检查唯一约束 |
| 500 Internal Server Error | 服务端错误 | 查看服务日志 |
| 超时 | 服务未启动 | 启动 Centag 服务 |
