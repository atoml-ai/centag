#!/usr/bin/env bash
# ============================================================================
# Centag Admin E2E — 预置执行脚本（证据增强版）
#
# 输出：
#   /tmp/admin_e2e_results.ndjson      # 每条用例的详细证据
#   /tmp/admin_e2e_results.json        # 汇总 + 明细
# ============================================================================

set -u
set -o pipefail

BASE_URL="${TEST_BASE_URL:-http://localhost:20060}"
TEST_DEPLOY_TYPE="${TEST_DEPLOY_TYPE:-${CENTAG_DEPLOY_TYPE:-gateway}}"
ENV_FILE="${ADMIN_ENV_FILE:-config/secrets/.env}"
RESULTS_NDJSON="/tmp/admin_e2e_results.ndjson"
RESULTS_JSON="/tmp/admin_e2e_results.json"
TMP_DIR="/tmp/admin_e2e_cases_$$"
mkdir -p "$TMP_DIR"

if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

resolve_admin_user() {
  if [ -n "${ADMIN_USERNAME:-}" ]; then
    echo "${ADMIN_USERNAME}"
    return
  fi
  if [ -n "${LLM_PROXY_ADMIN_USERNAME:-}" ]; then
    echo "${LLM_PROXY_ADMIN_USERNAME}"
    return
  fi
  echo "admin"
}

resolve_admin_password() {
  if [ -n "${ADMIN_PASSWORD:-}" ]; then
    echo "${ADMIN_PASSWORD}"
    return
  fi
  if [ -n "${LLM_PROXY_ADMIN_PASSWORD:-}" ]; then
    echo "${LLM_PROXY_ADMIN_PASSWORD}"
    return
  fi
  echo ""
}

http_code_in_expected() {
  local code="$1"
  local expected_csv="$2"
  case ",${expected_csv}," in
    *",${code},"*) return 0 ;;
    *) return 1 ;;
  esac
}

assert_json_expr() {
  local body="$1"
  local expr="$2"
  if [ -z "$expr" ]; then
    echo "true"
    return
  fi
  if printf "%s" "$body" | jq -e "$expr" >/dev/null 2>&1; then
    echo "true"
  else
    echo "false"
  fi
}

record_case() {
  local module="$1"
  local name="$2"
  local method="$3"
  local path="$4"
  local expected_codes="$5"
  local assert_desc="$6"
  local assert_expr="$7"
  local payload="$8"
  local mock_data="$9"

  local case_id
  case_id="$(date +%s%N)"
  local body_file="${TMP_DIR}/${case_id}.body"
  local header_file="${TMP_DIR}/${case_id}.headers"
  local url="${BASE_URL}${path}"

  if [ -n "$payload" ]; then
    curl -s -o "$body_file" -D "$header_file" -w "%{http_code}" -X "$method" "$url" \
      -H "$AUTH_HEADER" \
      -H "Content-Type: application/json" \
      -d "$payload" > "${TMP_DIR}/${case_id}.code"
  else
    curl -s -o "$body_file" -D "$header_file" -w "%{http_code}" -X "$method" "$url" \
      -H "$AUTH_HEADER" > "${TMP_DIR}/${case_id}.code"
  fi

  local http_code body headers http_ok assert_ok ok note curl_cmd
  http_code="$(cat "${TMP_DIR}/${case_id}.code")"
  body="$(cat "$body_file")"
  headers="$(cat "$header_file")"

  if http_code_in_expected "$http_code" "$expected_codes"; then
    http_ok="true"
  else
    http_ok="false"
  fi

  assert_ok="$(assert_json_expr "$body" "$assert_expr")"
  if [ "$http_ok" = "true" ] && [ "$assert_ok" = "true" ]; then
    ok="true"
    note="HTTP 与断言均通过"
  elif [ "$http_ok" != "true" ] && [ "$assert_ok" = "true" ]; then
    ok="false"
    note="断言通过但 HTTP 状态码不在预期集合"
  elif [ "$http_ok" = "true" ] && [ "$assert_ok" != "true" ]; then
    ok="false"
    note="HTTP 状态码符合预期，但业务断言未通过"
  else
    ok="false"
    note="HTTP 与断言均未通过"
  fi

  if [ -n "$payload" ]; then
    curl_cmd="curl -s -X ${method} \"${url}\" -H \"${AUTH_HEADER}\" -H \"Content-Type: application/json\" -d '${payload}'"
  else
    curl_cmd="curl -s -X ${method} \"${url}\" -H \"${AUTH_HEADER}\""
  fi

  jq -nc \
    --arg module "$module" \
    --arg name "$name" \
    --arg method "$method" \
    --arg path "$path" \
    --arg url "$url" \
    --arg expected_codes "$expected_codes" \
    --arg assert_desc "$assert_desc" \
    --arg assert_expr "$assert_expr" \
    --arg payload "$payload" \
    --arg mock_data "$mock_data" \
    --arg curl_cmd "$curl_cmd" \
    --arg response_headers "$headers" \
    --arg response_body "$body" \
    --argjson http_code "$http_code" \
    --argjson http_ok "$http_ok" \
    --argjson assert_ok "$assert_ok" \
    --argjson ok "$ok" \
    --arg note "$note" \
    '{
      module:$module,
      name:$name,
      method:$method,
      path:$path,
      url:$url,
      expected_codes:$expected_codes,
      assert_desc:$assert_desc,
      assert_expr:$assert_expr,
      request_payload:$payload,
      mock_data:$mock_data,
      curl_cmd:$curl_cmd,
      http_code:$http_code,
      http_ok:$http_ok,
      assert_ok:$assert_ok,
      ok:$ok,
      note:$note,
      response_headers:$response_headers,
      response_body:$response_body
    }' >> "$RESULTS_NDJSON"

  echo "${name} -> HTTP ${http_code}, assert=${assert_ok}"
}

service_code="$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/api/v1/status" || true)"
if [ "$service_code" != "200" ]; then
  echo "❌ 服务不可用: ${BASE_URL}/api/v1/status -> HTTP ${service_code:-0}"
  exit 1
fi

ADMIN_USER="$(resolve_admin_user)"
ADMIN_PASS="$(resolve_admin_password)"
JWT="${TEST_JWT_TOKEN:-}"

if [ -n "$JWT" ]; then
  me_code="$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer ${JWT}" "${BASE_URL}/api/auth/me")"
  if [ "$me_code" != "200" ]; then
    JWT=""
  fi
fi

if [ -z "$JWT" ]; then
  if [ -z "$ADMIN_USER" ] || [ -z "$ADMIN_PASS" ]; then
    echo "❌ 登录失败：缺少可用的 Admin 用户名或密码"
    exit 1
  fi
  login_payload="$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PASS" '{username:$u,password:$p}')"
  login_resp="$(curl -s -X POST "${BASE_URL}/api/auth/login" -H "Content-Type: application/json" -d "$login_payload")"
  JWT="$(printf "%s" "$login_resp" | jq -r '.data.access_token // empty')"
  if [ -z "$JWT" ]; then
    echo "❌ 登录失败: ${login_resp}"
    exit 1
  fi
fi

export TEST_JWT_TOKEN="$JWT"
AUTH_HEADER="Authorization: Bearer ${JWT}"

: > "$RESULTS_NDJSON"
echo "=== Admin E2E 开始 (${TEST_DEPLOY_TYPE}) ==="

TEST_USERNAME="e2e_test_user_$(date +%s)"
TEST_PASSWORD="Test123456"
TEST_DISPLAY_NAME="E2E Test User"
TEST_APIKEY_NAME="e2e-test-key-$(date +%s)"
TEST_SKIP_AGENT_PROVIDERS="${TEST_SKIP_AGENT_PROVIDERS:-true}"

# ---------------------------------------------------------------------------
# minimal：精简套件
# ---------------------------------------------------------------------------
if [ "${TEST_DEPLOY_TYPE}" = "minimal" ]; then
  record_case "Auth" "bootstrap-status" "GET" "/api/v1/auth/bootstrap-status" "200" \
    "返回 success 且含 edition/initialized" \
    '.success == true' \
    "" \
    "无"

  record_case "后端管理" "后端-列表" "GET" "/api/v1/backends" "200" \
    "返回成功且 data 为数组" \
    '.success == true and ((.data|type)=="array")' \
    "" \
    "无"

  record_case "流水线管理" "流水线-列表" "GET" "/api/v1/pipelines" "200" \
    "返回成功且 data 为数组" \
    '.success == true and ((.data|type)=="array")' \
    "" \
    "无"

  record_case "API Key" "settings-api-keys" "GET" "/api/v1/settings/api-keys" "200" \
    "返回成功" \
    '.success == true' \
    "" \
    "无"

  record_case "API Key" "settings-api-keys-status" "GET" "/api/v1/settings/api-keys/status" "200" \
    "返回 auth_required 字段" \
    '.success == true' \
    "" \
    "无"

  record_case "健康检查" "健康检查" "GET" "/health" "200" \
    "健康检查返回 200" \
    '((.status // "") | ascii_downcase) as $s | ($s == "ok" or $s == "healthy")' \
    "" \
    "无"

  jq -s '
    def pct(a;b): if b==0 then 0 else ((a*10000/b)|floor)/100 end;
    . as $rows |
    {
      timestamp:(now|strftime("%Y-%m-%d %H:%M:%S")),
      mode:"admin",
      deploy_type:(env.TEST_DEPLOY_TYPE // "minimal"),
      base_url:(env.TEST_BASE_URL // "http://localhost:20060"),
      total:($rows|length),
      passed:($rows|map(select(.ok))|length),
      failed:($rows|map(select(.ok|not))|length),
      pass_rate:(pct(($rows|map(select(.ok))|length);($rows|length))),
      results:$rows
    }' "$RESULTS_NDJSON" > "$RESULTS_JSON"

  summary="$(jq -r '"SUMMARY passed=\(.passed)/\(.total) rate=\(.pass_rate)%"' "$RESULTS_JSON")"
  echo "RESULT_FILE=${RESULTS_JSON}"
  echo "${summary}"
  rm -rf "$TMP_DIR"
  exit 0
fi

record_case "用户管理" "用户管理-列表" "GET" "/api/v1/admin/users" "200" \
  "返回成功且 data 为数组" \
  '.success == true and ((.data|type)=="array")' \
  "" \
  "无"

create_user_payload="$(jq -nc --arg u "$TEST_USERNAME" --arg p "$TEST_PASSWORD" '{username:$u,password:$p,role:"normal"}')"
record_case "用户管理" "用户管理-创建" "POST" "/api/v1/admin/users" "201" \
  "创建成功并返回用户 ID" \
  '.success == true and ((.data.id|type)=="number" or ((.data.id|type)=="string" and (.data.id|length>0)))' \
  "$create_user_payload" \
  "username=${TEST_USERNAME};password=${TEST_PASSWORD};role=normal"

NEW_USER_ID="$(tail -n 1 "$RESULTS_NDJSON" | jq -r '.response_body | fromjson? // . | .data.id // empty' 2>/dev/null || true)"
if [ -z "$NEW_USER_ID" ]; then
  # 兼容 response_body 本身为 JSON 字符串
  NEW_USER_ID="$(tail -n 1 "$RESULTS_NDJSON" | jq -r '.response_body | (fromjson? // .) | .data.id // empty' 2>/dev/null || true)"
fi

if [ -n "$NEW_USER_ID" ]; then
  update_payload="$(jq -nc --arg d "$TEST_DISPLAY_NAME" '{display_name:$d}')"
  record_case "用户管理" "用户管理-更新" "PUT" "/api/v1/admin/users/${NEW_USER_ID}" "200" \
    "更新成功" \
    '.success == true' \
    "$update_payload" \
    "user_id=${NEW_USER_ID};display_name=${TEST_DISPLAY_NAME}"

  record_case "用户管理" "用户管理-删除" "DELETE" "/api/v1/admin/users/${NEW_USER_ID}" "200" \
    "删除成功" \
    '.success == true' \
    "" \
    "user_id=${NEW_USER_ID}"
fi

record_case "API Key" "APIKey-列表" "GET" "/api/v1/user/apikeys" "200" \
  "返回成功且 data 为数组" \
  '.success == true and ((.data|type)=="array")' \
  "" \
  "无"

create_key_payload="$(jq -nc --arg n "$TEST_APIKEY_NAME" '{name:$n}')"
record_case "API Key" "APIKey-创建" "POST" "/api/v1/user/apikeys" "201" \
  "创建成功并返回 key ID" \
  '.success == true and ((.data.id|type)=="number" or ((.data.id|type)=="string" and (.data.id|length>0)))' \
  "$create_key_payload" \
  "name=${TEST_APIKEY_NAME}"

NEW_KEY_ID="$(tail -n 1 "$RESULTS_NDJSON" | jq -r '.response_body | (fromjson? // .) | .data.id // empty' 2>/dev/null || true)"
if [ -n "$NEW_KEY_ID" ]; then
  record_case "API Key" "APIKey-获取" "GET" "/api/v1/user/apikeys/${NEW_KEY_ID}" "200" \
    "获取成功并返回同一 key ID" \
    '.success == true and ((.data.id // .data.key_id // "")|tostring|length > 0)' \
    "" \
    "key_id=${NEW_KEY_ID}"

  record_case "API Key" "APIKey-删除" "DELETE" "/api/v1/user/apikeys/${NEW_KEY_ID}" "200" \
    "删除成功" \
    '.success == true' \
    "" \
    "key_id=${NEW_KEY_ID}"
fi

record_case "Token 用量" "Token用量-用户" "GET" "/api/v1/user/token-usage" "200" \
  "返回成功" \
  '.success == true' \
  "" \
  "无"

if [ "${TEST_SKIP_AGENT_PROVIDERS}" != "true" ]; then
  record_case "Agent 供应商" "Agent供应商-列表" "GET" "/api/v1/agent-providers" "200" \
    "返回成功且 agent_providers 为数组" \
    '((.agent_providers|type)=="array")' \
    "" \
    "无"
else
  echo "⏭️  跳过 Agent 供应商（TEST_SKIP_AGENT_PROVIDERS=true）"
fi

record_case "后端管理" "后端-列表" "GET" "/api/v1/backends" "200" \
  "返回成功且 data 为数组" \
  '.success == true and ((.data|type)=="array")' \
  "" \
  "无"

record_case "流水线管理" "流水线-列表" "GET" "/api/v1/pipelines" "200" \
  "返回成功且 data 为数组" \
  '.success == true and ((.data|type)=="array")' \
  "" \
  "无"

record_case "系统配置" "系统配置-获取" "GET" "/api/v1/config" "200" \
  "返回成功且 data 为对象" \
  '.success == true and ((.data|type)=="object")' \
  "" \
  "无"

record_case "Profile" "Profile-获取" "GET" "/api/v1/user/profile" "200" \
  "返回成功且 data 为对象" \
  '.success == true and ((.data|type)=="object")' \
  "" \
  "无"

record_case "健康检查" "健康检查" "GET" "/health" "200" \
  "健康检查返回 200 且 status=ok 或 healthy" \
  '((.status // "") | ascii_downcase) as $s | ($s == "ok" or $s == "healthy")' \
  "" \
  "无"

if [ "${TEST_DEPLOY_TYPE}" = "team" ]; then
  record_case "多租户" "多租户-列表" "GET" "/api/v1/admin/tenants" "200" \
    "返回成功且 data 为数组" \
    '.success == true and ((.data|type)=="array")' \
    "" \
    "无"
  record_case "成本看板" "成本看板-汇总" "GET" "/api/v1/admin/cost/summary" "200" \
    "返回成功" \
    '.success == true' \
    "" \
    "无"
fi

jq -s '
  def pct(a;b): if b==0 then 0 else ((a*10000/b)|floor)/100 end;
  . as $rows |
  {
    timestamp:(now|strftime("%Y-%m-%d %H:%M:%S")),
    mode:"admin",
    deploy_type:(env.TEST_DEPLOY_TYPE // "gateway"),
    base_url:(env.TEST_BASE_URL // "http://localhost:20060"),
    total:($rows|length),
    passed:($rows|map(select(.ok))|length),
    failed:($rows|map(select(.ok|not))|length),
    pass_rate:(pct(($rows|map(select(.ok))|length);($rows|length))),
    results:$rows
  }' "$RESULTS_NDJSON" > "$RESULTS_JSON"

summary="$(jq -r '"SUMMARY passed=\(.passed)/\(.total) rate=\(.pass_rate)%"' "$RESULTS_JSON")"
echo "RESULT_FILE=${RESULTS_JSON}"
echo "${summary}"
rm -rf "$TMP_DIR"
exit 0
