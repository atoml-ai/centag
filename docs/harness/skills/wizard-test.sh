#!/usr/bin/env bash
# ============================================================================
# Centag Wizard Test — 测试执行脚本 (Steps A–F)
#
# 用法：通过环境变量注入所有参数，直接执行：
#   export TEST_BASE_URL="http://localhost:20060"
#   export TEST_JWT_TOKEN="<从 POST /api/auth/login 获取>"
#   export ADMIN_USERNAME="admin"
#   export ADMIN_PASSWORD="..."
#   export TEST_BACKEND_ID="bigmodel"
#   export TEST_BACKEND_TYPE="openai"
#   export TEST_BACKEND_BASE_URL="https://..."
#   export TEST_BACKEND_KEY="..."
#   export TEST_BACKEND_MODEL="glm-4-flash"
#   export TEST_BACKEND_SOURCE="real"   # real|mock
#   export TEST_MOCK_OPENAI_HOST="127.0.0.1"
#   export TEST_MOCK_OPENAI_PORT="28081"
#   export CENTAG_DEPLOY_TYPE="personal"  # personal|team|minimal
#   export TEST_PIPELINES="smart-scheduling,direct-backend,transparent-proxy,fixed-egress"
#   bash wizard-test.sh
#
# 输出：
#   /tmp/wizard_test_data.json          — 每条流水线的测试结果
#   /tmp/wizard_pipeline_update_cmds.json — 流水线 PUT 命令记录
#   /tmp/wizard_probe.json              — 后端探测结果
#   /tmp/wizard_test_results.sh         — PASSED/FAILED/TOTAL 供报告脚本读取
# ============================================================================
set -o pipefail

BASE="${TEST_BASE_URL:-http://localhost:20060}"
# Align deploy type for admin-e2e / report scripts
export CENTAG_DEPLOY_TYPE="${CENTAG_DEPLOY_TYPE:-personal}"
export TEST_DEPLOY_TYPE="${TEST_DEPLOY_TYPE:-$CENTAG_DEPLOY_TYPE}"
JWT="${TEST_JWT_TOKEN}"
ENTRY_VARIANTS="${TEST_ENTRY_VARIANTS:-header-full}"
REPEAT_PER_VARIANT="${TEST_REPEAT_PER_VARIANT:-1}"
LOG_EVIDENCE_LEVEL="${TEST_LOG_EVIDENCE_LEVEL:-basic}"
BACKEND_SOURCE="${TEST_BACKEND_SOURCE:-real}"           # real | mock
MOCK_OPENAI_HOST="${TEST_MOCK_OPENAI_HOST:-127.0.0.1}"
MOCK_OPENAI_PORT="${TEST_MOCK_OPENAI_PORT:-28081}"
MOCK_OPENAI_BASE_URL="http://${MOCK_OPENAI_HOST}:${MOCK_OPENAI_PORT}"
SCRIPT_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
MOCK_SERVER_SCRIPT="${SCRIPT_ROOT}/scripts/test/e2e/fake_openai_server.py"
MOCK_SERVER_PID=""
BACKEND_BACKUP_FILE=""
BACKEND_BACKUP_TARGET_ID=""
TEMP_BACKEND_FALLBACK="${TEST_TEMP_BACKEND_FALLBACK:-auto}"
TEMP_BACKEND_PREFIX="${TEST_TEMP_BACKEND_PREFIX:-}"
TEMP_BACKEND_USED="false"
TEMP_BACKEND_SOURCE_ID=""
TEMP_BACKEND_FINAL_ID=""
TEMP_BACKEND_REASON=""
TEMP_BACKEND_SIGNAL=""

cleanup_wizard() {
  if [ -n "${BACKEND_BACKUP_FILE}" ] && [ -f "${BACKEND_BACKUP_FILE}" ] && [ -n "${JWT}" ] && [ -n "${BACKEND_BACKUP_TARGET_ID}" ]; then
    local backup_payload
    backup_payload="$(cat "${BACKEND_BACKUP_FILE}")"
    if [ -n "${backup_payload}" ]; then
      curl -s --max-time 20 -X PUT "${BASE}/api/v1/backends/${BACKEND_BACKUP_TARGET_ID}" \
        -H "Authorization: Bearer ${JWT}" \
        -H "Content-Type: application/json" \
        -d "${backup_payload}" >/dev/null 2>&1 || true
      echo "🔄 已恢复后端配置: ${BACKEND_BACKUP_TARGET_ID}"
    fi
  fi

  if [ -n "${MOCK_SERVER_PID}" ]; then
    kill "${MOCK_SERVER_PID}" >/dev/null 2>&1 || true
    wait "${MOCK_SERVER_PID}" >/dev/null 2>&1 || true
  fi

  if [ -n "${BACKEND_BACKUP_FILE}" ]; then
    rm -f "${BACKEND_BACKUP_FILE}" >/dev/null 2>&1 || true
  fi
}
trap cleanup_wizard EXIT

backup_backend_config() {
  if [ -z "${JWT}" ] || [ -z "${TEST_BACKEND_ID}" ]; then
    return 1
  fi
  BACKEND_BACKUP_TARGET_ID="${TEST_BACKEND_ID}"
  local export_json
  export_json=$(curl -s --max-time 20 \
    -H "Authorization: Bearer ${JWT}" \
    "${BASE}/api/v1/backends/export")
  BACKEND_BACKUP_FILE="/tmp/wizard_backend_backup_${BACKEND_BACKUP_TARGET_ID}_$$.json"
  echo "$export_json" | jq -c --arg id "${BACKEND_BACKUP_TARGET_ID}" '.data // . | map(select(.id == $id)) | .[0]' > "${BACKEND_BACKUP_FILE}" 2>/dev/null || true
  if [ ! -s "${BACKEND_BACKUP_FILE}" ] || [ "$(cat "${BACKEND_BACKUP_FILE}")" = "null" ]; then
    rm -f "${BACKEND_BACKUP_FILE}"
    BACKEND_BACKUP_FILE=""
    BACKEND_BACKUP_TARGET_ID=""
    return 1
  fi
  return 0
}

start_mock_openai_if_needed() {
  if [ "${BACKEND_SOURCE}" != "mock" ]; then
    return 0
  fi

  if [ ! -f "${MOCK_SERVER_SCRIPT}" ]; then
    echo "❌ Mock 模式失败：未找到 fake server 脚本 ${MOCK_SERVER_SCRIPT}"
    return 1
  fi

  python3 "${MOCK_SERVER_SCRIPT}" --host "${MOCK_OPENAI_HOST}" --port "${MOCK_OPENAI_PORT}" >/tmp/wizard_fake_openai.log 2>&1 &
  MOCK_SERVER_PID="$!"
  local health=""
  local i
  for i in $(seq 1 25); do
    health=$(curl -s --max-time 2 "${MOCK_OPENAI_BASE_URL}/health" || true)
    if echo "$health" | jq -e '.status=="ok"' >/dev/null 2>&1; then
      break
    fi
    sleep 0.2
  done
  if ! echo "$health" | jq -e '.status=="ok"' >/dev/null 2>&1; then
    echo "❌ Mock 模式失败：fake openai 未就绪 (${MOCK_OPENAI_BASE_URL})"
    if [ -f "/tmp/wizard_fake_openai.log" ]; then
      echo "   fake server 日志: $(cat /tmp/wizard_fake_openai.log)"
    fi
    return 1
  fi

  TEST_BACKEND_TYPE="openai"
  TEST_BACKEND_BASE_URL="${MOCK_OPENAI_BASE_URL}"
  TEST_BACKEND_KEY="${TEST_BACKEND_KEY:-fake-e2e-key}"
  TEST_BACKEND_MODEL="${TEST_BACKEND_MODEL:-glm-5.2}"
  export TEST_BACKEND_TYPE TEST_BACKEND_BASE_URL TEST_BACKEND_KEY TEST_BACKEND_MODEL
  echo "✅ 已启用 Mock 模式：${TEST_BACKEND_ID} → ${TEST_BACKEND_BASE_URL}"
  return 0
}

# ---- 辅助函数：更新单条流水线节点 ----
update_pipeline_nodes() {
  local pid="$1"
  local backend="$2"
  local model="$3"
  local BASE="$4"
  local KEY="$5"

  local raw
  raw=$(curl -s --max-time 10 \
    -H "Authorization: Bearer ${KEY}" \
    "${BASE}/api/v1/pipelines/${pid}")

  if [ -z "$raw" ]; then
    echo "  ⚠️  ${pid}: GET 返回空，跳过"
    return 1
  fi

  local pipeline
  pipeline=$(echo "$raw" | jq -c '.data // .' 2>/dev/null)

  if [ -z "$pipeline" ] || [ "$pipeline" = "null" ]; then
    echo "  ⚠️  ${pid}: 无法解析流水线 JSON，跳过"
    return 1
  fi

  local node_count
  node_count=$(echo "$pipeline" | jq '.nodes | length' 2>/dev/null)
  if [ "$node_count" = "0" ] || [ -z "$node_count" ]; then
    echo "  ⚠️  ${pid}: 无节点数据 (count=${node_count:-0})，跳过"
    return 1
  fi

  local updated
  updated=$(echo "$pipeline" | jq -c \
    --arg b "$backend" --arg m "$model" \
    '.nodes = (.nodes | map(.backend = $b | .model = $m | .config.backend = $b | .config.model = $m))' 2>/dev/null)

  if [ -z "$updated" ] || [ "$updated" = "null" ]; then
    echo "  ⚠️  ${pid}: jq 处理失败，跳过"
    return 1
  fi

  local tmpfile
  tmpfile="/tmp/wizard_put_${pid}_$$.json"
  echo "$updated" > "$tmpfile"

  local put_cmd="curl -s -w '\\n%{http_code}' --max-time 10 -X PUT '${BASE}/api/v1/pipelines/${pid}' -H 'Authorization: Bearer \${JWT}' -H 'Content-Type: application/json' -d @${tmpfile}"
  local put_resp
  local http_code
  put_resp=$(curl -s -w "\n%{http_code}" --max-time 10 \
    -X PUT "${BASE}/api/v1/pipelines/${pid}" \
    -H "Authorization: Bearer ${KEY}" \
    -H "Content-Type: application/json" \
    -d "@${tmpfile}")
  http_code=$(echo "$put_resp" | tail -1)
  local resp_body
  resp_body=$(echo "$put_resp" | sed '$d')

  local success_str="false"
  local note=""
  if [ "$http_code" = "200" ]; then
    success_str="true"
    note="节点已更新 (${backend}/${model}, ${node_count} nodes)"
  else
    note=$(echo "$resp_body" | jq -r '.data.message // .message // .error // "无详情"' 2>/dev/null)
  fi

  echo "${PIPELINE_CMD_SEP}{ \"pipeline\": \"${pid}\", \"success\": ${success_str}, \"http_code\": ${http_code}, \"note\": \"${note}\", \"put_cmd\": \"${put_cmd}\" }" >> "$PIPELINE_UPDATE_CMDS_FILE"
  PIPELINE_CMD_SEP=","

  rm -f "$tmpfile"

  if [ "$http_code" = "200" ]; then
    echo "  ✅  ${pid}: 节点已更新 (${backend}/${model})"
  else
    echo "  ⚠️  ${pid}: PUT 返回 HTTP ${http_code}，${note}"
    return 1
  fi
}

trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  echo "$s"
}

normalize_http_code() {
  local raw="$1"
  if [[ "$raw" =~ ^[0-9]{3}$ ]]; then
    if [ "$raw" = "000" ]; then
      echo "0"
      return
    fi
    echo "$raw"
    return
  fi
  echo "0"
}

shortcut_for_pipeline() {
  local pid="$1"
  case "$pid" in
    smart-scheduling) echo "#s" ;;
    direct-backend) echo "#d" ;;
    transparent-proxy) echo "#t" ;;
    transparent-fast) echo "#tf" ;;
    fallback-mode) echo "#f" ;;
    optimize-mode) echo "#o" ;;
    audit-mode) echo "#a" ;;
    model-matching) echo "#m" ;;
    aggregator-mode) echo "#ag" ;;
    translate-mode) echo "#l" ;;
    router-mode) echo "#r" ;;
    pipeline-mode) echo "#p" ;;
    security-mode) echo "#sec" ;;
    *) echo "" ;;
  esac
}

json_escape() {
  printf "%s" "$1" | jq -Rs '.' 2>/dev/null
}

url_encode() {
  python3 - "$1" <<'PY'
import sys, urllib.parse
print(urllib.parse.quote(sys.argv[1], safe=''))
PY
}

should_trigger_temp_backend_fallback() {
  local http_code="$1"
  local content="$2"
  local tokens="$3"
  local error_msg="$4"
  local cache_read="$5"

  TEMP_BACKEND_SIGNAL=""
  if echo "$error_msg" | rg -qi "circuit breaker open|all fallback attempts failed"; then
    TEMP_BACKEND_SIGNAL="circuit_breaker_or_fallback_failed"
    return 0
  fi

  if [ "$http_code" = "500" ] || [ "$http_code" = "503" ]; then
    TEMP_BACKEND_SIGNAL="http_${http_code}"
    return 0
  fi

  # 200+空响应需要结合缓存命中判断，避免旧缓存导致误切换
  if [ "$http_code" = "200" ] && [ -z "$content" ] && [ "${tokens:-0}" = "0" ]; then
    if [ "$cache_read" = "true" ]; then
      return 1
    fi
    TEMP_BACKEND_SIGNAL="empty_response_tokens0_no_cache"
    return 0
  fi

  return 1
}

create_temp_backend() {
  local base="$1"
  local jwt="$2"
  local source_id="$3"
  local backend_type="$4"
  local backend_url="$5"
  local backend_key="$6"
  local probe_model="$7"
  local prefix="$8"
  local ts new_id create_body create_code create_resp probe_resp probe_ok

  ts="$(date +%Y%m%d%H%M%S)"
  if [ -z "$prefix" ]; then
    prefix="${source_id}-temp"
  fi
  new_id="${prefix}-${ts}"

  create_body=$(cat <<EOF
{"id":"${new_id}","name":"${new_id}","enabled":true,"api_key":"${backend_key}","type":"${backend_type}","base_url":"${backend_url}","auto_fetch_models":true}
EOF
)
  create_resp=$(curl -s --max-time 20 -w "\n%{http_code}" \
    -X POST "${base}/api/v1/backends" \
    -H "Authorization: Bearer ${jwt}" \
    -H "Content-Type: application/json" \
    -d "${create_body}")
  create_code=$(echo "$create_resp" | tail -1)

  if [ "$create_code" != "200" ] && [ "$create_code" != "201" ]; then
    echo "$create_resp" > "/tmp/wizard_temp_backend_create_${new_id}.json"
    echo ""
    return 1
  fi

  # 立即探测新后端，确保可用
  probe_resp=$(curl -s --max-time 20 \
    -X POST "${base}/api/v1/backends/${new_id}/probe" \
    -H "Authorization: Bearer ${jwt}" \
    -H "Content-Type: application/json")
  probe_ok=$(echo "$probe_resp" | jq -r '.data.success // .success // false' 2>/dev/null)
  if [ "$probe_ok" != "true" ]; then
    # 探测失败也允许继续，后续 Step F 再做真实校验
    :
  fi

  echo "$new_id"
  return 0
}

reset_backend_circuit_breaker() {
  local base="$1"
  local jwt="$2"
  local backend_id="$3"
  if [ -z "$backend_id" ]; then
    return 0
  fi
  local code
  code=$(curl -s -o "/tmp/wizard_breaker_reset_${backend_id}.json" -w "%{http_code}" --max-time 15 \
    -X POST "${base}/api/v1/backends/circuit-breaker/${backend_id}/reset" \
    -H "Authorization: Bearer ${jwt}" \
    -H "Content-Type: application/json")
  if [ "$code" = "200" ]; then
    echo "✅ 已重置熔断器: ${backend_id}"
  else
    echo "⚠️  熔断器重置失败: ${backend_id} (HTTP ${code})"
  fi
}

detect_header_override_support() {
  local base="$1"
  local auth_key="$2"
  local out_file="/tmp/wizard_header_probe_body_$$.json"
  local header_file="/tmp/wizard_header_probe_headers_$$.txt"
  local code pipeline_id

  code=$(curl -s -o "$out_file" -D "$header_file" -w "%{http_code}" --max-time 20 \
    -X POST "${base}/v1/chat/completions" \
    -H "Authorization: Bearer ${auth_key}" \
    -H "X-Proxy-Mode: smart-scheduling" \
    -H "Content-Type: application/json" \
    -d '{"model":"glm-4-flash","messages":[{"role":"user","content":"header override probe"}],"max_tokens":16}')

  pipeline_id="$(trim "$(awk 'BEGIN{IGNORECASE=1} /^X-Pipeline-Id:/ {sub(/\r$/,"",$2); print $2}' "$header_file" | tail -1)")"
  rm -f "$out_file" "$header_file"

  if [ "$code" = "200" ] && [ "$pipeline_id" = "smart-scheduling" ]; then
    echo "true"
  else
    echo "false"
  fi
}

try_enable_header_override() {
  local base="$1"
  local jwt="$2"
  local cfg_resp cfg_payload put_code

  cfg_resp=$(curl -s --max-time 20 \
    -H "Authorization: Bearer ${jwt}" \
    "${base}/api/v1/config")

  cfg_payload=$(echo "$cfg_resp" | jq -c '.data | .proxy.allow_header_override = true' 2>/dev/null)
  if [ -z "$cfg_payload" ] || [ "$cfg_payload" = "null" ]; then
    echo "false"
    return
  fi

  put_code=$(curl -s -o /tmp/wizard_enable_header_override_resp.json -w "%{http_code}" --max-time 30 \
    -X PUT "${base}/api/v1/config" \
    -H "Authorization: Bearer ${jwt}" \
    -H "Content-Type: application/json" \
    -d "${cfg_payload}")

  if [ "$put_code" = "200" ]; then
    echo "true"
  else
    echo "false"
  fi
}

# ========================================================================
# Step A: 健康探测
# ========================================================================
echo "=== Step A: 健康探测 ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "${BASE}/health" 2>/dev/null)
if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ 服务就绪 (${BASE}/health → 200)"
else
  echo "❌ 服务不可达或未就绪 (HTTP ${HTTP_CODE})"
  echo "   请先启动服务"
  exit 1
fi

# ========================================================================
# Step B: 确定测试凭据
# ========================================================================
echo ""
echo "=== Step B: 确认认证凭据 ==="

if [ -z "${JWT}" ]; then
  echo "❌ TEST_JWT_TOKEN 未设置！"
  exit 1
fi

ME_RESP=$(curl -s --max-time 10 "${BASE}/api/auth/me" \
  -H "Authorization: Bearer ${JWT}")
ME_USER=$(echo "$ME_RESP" | jq -r '.data.username // .username // empty' 2>/dev/null)

if [ -n "$ME_USER" ]; then
  echo "✅ JWT 有效，当前用户: ${ME_USER}"
else
  echo "⚠️  JWT 无效或已过期，尝试自动重新登录…"
  if [ -n "${ADMIN_USERNAME}" ] && [ -n "${ADMIN_PASSWORD}" ]; then
    LOGIN_RESP=$(curl -s --max-time 10 -X POST "${BASE}/api/auth/login" \
      -H "Content-Type: application/json" \
      -d "{\"username\":\"${ADMIN_USERNAME}\",\"password\":\"${ADMIN_PASSWORD}\"}")
    JWT=$(echo "$LOGIN_RESP" | jq -r '.data.access_token // .access_token // empty' 2>/dev/null)
    if [ -n "$JWT" ]; then
      export TEST_JWT_TOKEN="${JWT}"
      echo "✅ JWT 已刷新，重新验证通过"
    else
      echo "❌ 自动重新登录失败"
      exit 1
    fi
  else
    echo "❌ 缺少 ADMIN_USERNAME/ADMIN_PASSWORD，无法自动重新登录"
    exit 1
  fi
fi

case "${CENTAG_DEPLOY_TYPE:-personal}" in
  desktop|personal|minimal)
    echo "=== ${CENTAG_DEPLOY_TYPE} — 使用 Admin JWT 测试 ==="
    TEST_AUTH_KEY="${JWT}"
    ;;
  team)
    echo "=== Web 团队版 — 使用用户 Key 测试 ==="
    if [ -n "${TEST_USER_KEY}" ]; then
      TEST_AUTH_KEY="${TEST_USER_KEY}"
      echo "  使用测试用户 API Key"
    else
      echo "  ⚠️  TEST_USER_KEY 未设置，降级使用 Admin JWT"
      TEST_AUTH_KEY="${JWT}"
    fi
    ;;
  *)
    echo "⚠️  未知 CENTAG_DEPLOY_TYPE=${CENTAG_DEPLOY_TYPE}，按 personal 使用 Admin JWT"
    TEST_AUTH_KEY="${JWT}"
    ;;
esac
export TEST_AUTH_KEY

echo ""
echo "=== Step B3: Header 覆盖能力探测 ==="
HEADER_OVERRIDE_SUPPORTED="$(detect_header_override_support "$BASE" "$TEST_AUTH_KEY")"
if [ "$HEADER_OVERRIDE_SUPPORTED" = "true" ]; then
  echo "✅ X-Proxy-Mode Header 生效（可执行 header-full 严格判定）"
else
  echo "⚠️  X-Proxy-Mode Header 当前未生效，尝试自动开启 allow_header_override..."
  if [ "$(try_enable_header_override "$BASE" "$JWT")" = "true" ]; then
    sleep 1
    HEADER_OVERRIDE_SUPPORTED="$(detect_header_override_support "$BASE" "$TEST_AUTH_KEY")"
  fi
  if [ "$HEADER_OVERRIDE_SUPPORTED" = "true" ]; then
    echo "✅ 已自动开启并验证 X-Proxy-Mode Header 生效"
  else
    echo "⚠️  自动开启失败或仍未生效（可能被运行时配置覆盖）"
    echo "   Step F 将优先使用 prompt-shortcut 触发目标流水线，避免误判"
  fi
fi
export HEADER_OVERRIDE_SUPPORTED

# ========================================================================
# Step B2: 验证测试用户（仅团队版）
# ========================================================================
if [ "${CENTAG_DEPLOY_TYPE}" = "team" ]; then
  echo ""
  echo "=== Step B2: 验证测试用户 ==="
  USER_CHECK=$(curl -s --max-time 10 \
    -H "Authorization: Bearer ${JWT}" \
    "${BASE}/api/v1/admin/users?username=wizardtest")
  USER_ID=$(echo "$USER_CHECK" | jq -r '.data[0].id // .data.id // empty' 2>/dev/null)

  if [ -n "$USER_ID" ]; then
    echo "✅ 测试用户 wizardtest 存在 (ID: ${USER_ID})"
  else
    echo "⚠️  测试用户 wizardtest 未找到，降级使用 Admin JWT"
  fi

  if [ -n "${TEST_USER_KEY}" ]; then
    echo "✅ 用户 API Key 已获取"
  else
    echo "⚠️  用户 API Key 未设置"
  fi
fi

# ========================================================================
# Step B4: 后端来源模式（real/mock）
# ========================================================================
echo ""
echo "=== Step B4: 后端来源模式 ==="
if [ "${BACKEND_SOURCE}" = "mock" ]; then
  echo "ℹ️  选择模式: mock（本地 fake OpenAI）"
  if ! start_mock_openai_if_needed; then
    exit 1
  fi
  if backup_backend_config; then
    echo "✅ 已备份后端配置（脚本结束自动恢复）: ${TEST_BACKEND_ID}"
  else
    echo "⚠️  未能备份后端配置，mock 测试后可能保留临时配置"
  fi
else
  echo "ℹ️  选择模式: real（真实后端大模型）"
fi

# ========================================================================
# Step C: 配置后端
# ========================================================================
echo ""
echo "=== Step C: 配置后端: ${TEST_BACKEND_ID} ==="

BACKEND_UPDATE=$(curl -s --max-time 10 -X PUT "${BASE}/api/v1/backends/${TEST_BACKEND_ID}" \
  -H "Authorization: Bearer ${JWT}" \
  -H "Content-Type: application/json" \
  -d "{\"id\":\"${TEST_BACKEND_ID}\",\"enabled\":true,\"api_key\":\"${TEST_BACKEND_KEY}\",\"type\":\"${TEST_BACKEND_TYPE}\",\"base_url\":\"${TEST_BACKEND_BASE_URL}\",\"auto_fetch_models\":true}")

BACKEND_ERROR=$(echo "$BACKEND_UPDATE" | jq -r '.data.message // .message // .error // ""' 2>/dev/null)

if echo "$BACKEND_UPDATE" | jq -e '.data.enabled == true or .enabled == true' > /dev/null 2>&1; then
  echo "✅ 后端 ${TEST_BACKEND_ID} 已启用并配置 Key (type=${TEST_BACKEND_TYPE}, url=${TEST_BACKEND_BASE_URL})"
else
  echo "⚠️  后端配置结果异常: ${BACKEND_ERROR:-未知}（继续…）"
fi

# ========================================================================
# Step D: 验证后端连接
# ========================================================================
echo ""
echo "=== Step D: 验证后端连接: ${TEST_BACKEND_ID} ==="

PROBE_RESP=$(curl -s --max-time 15 -X POST "${BASE}/api/v1/backends/${TEST_BACKEND_ID}/probe" \
  -H "Authorization: Bearer ${JWT}" \
  -H "Content-Type: application/json")

PROBE_SUCCESS=$(echo "$PROBE_RESP" | jq -r '.data.success // .success // false' 2>/dev/null)
PROBE_STATUS=$(echo "$PROBE_RESP" | jq -r '.data.status // .status // "unknown"' 2>/dev/null)
PROBE_ERROR=$(echo "$PROBE_RESP" | jq -r '.data.last_error // .data.message // .error // ""' 2>/dev/null)
PROBE_RT=$(echo "$PROBE_RESP" | jq -r '.data.response_time // .response_time // 0' 2>/dev/null)
PROBE_MODELS=$(echo "$PROBE_RESP" | jq -r '.data.models_count // .models_count // 0' 2>/dev/null)

# 保存探测结果到文件（供报告脚本读取）
echo "{\"success\": \"${PROBE_SUCCESS}\", \"status\": \"${PROBE_STATUS}\", \"error\": \"${PROBE_ERROR}\", \"response_time\": ${PROBE_RT}, \"models_count\": ${PROBE_MODELS}}" > /tmp/wizard_probe.json

if [ "$PROBE_SUCCESS" = "true" ]; then
  echo "✅ 后端 ${TEST_BACKEND_ID} 连接验证通过"
  echo "   响应时间: ${PROBE_RT}ms, 模型数: ${PROBE_MODELS}"
elif [ "$PROBE_STATUS" = "healthy" ] || [ "$PROBE_STATUS" = "available" ]; then
  echo "✅ 后端 ${TEST_BACKEND_ID} 连接验证通过"
  echo "   响应时间: ${PROBE_RT}ms, 模型数: ${PROBE_MODELS}"
elif echo "$PROBE_ERROR" | grep -qi "backend type is required"; then
  echo "⚠️  后端 ${TEST_BACKEND_ID} 缺少 type 字段"
elif echo "$PROBE_ERROR" | grep -qi "base url"; then
  echo "⚠️  后端 ${TEST_BACKEND_ID} 缺少 base_url 字段"
elif [ "$PROBE_STATUS" = "unhealthy" ] || [ "$PROBE_STATUS" = "error" ]; then
  echo "⚠️  后端 ${TEST_BACKEND_ID} 连接验证失败"
  echo "   状态: ${PROBE_STATUS}, 错误: ${PROBE_ERROR}"
  echo "   将继续配置流水线，但测试可能失败"
else
  echo "⚠️  后端验证状态未知: success=${PROBE_SUCCESS}, status=${PROBE_STATUS}"
fi

# ========================================================================
# Step D2: 临时后端兜底（可选）
# ========================================================================
echo ""
echo "=== Step D2: 临时后端兜底评估 ==="
if [ "$TEMP_BACKEND_FALLBACK" = "off" ]; then
  echo "⏭️  已关闭临时后端兜底（TEST_TEMP_BACKEND_FALLBACK=off）"
else
  smoke_file="/tmp/wizard_primary_smoke_${TEST_BACKEND_ID}.json"
  smoke_header="/tmp/wizard_primary_smoke_${TEST_BACKEND_ID}.headers"
  smoke_code=$(curl -s -o "$smoke_file" -D "$smoke_header" -w "%{http_code}" --max-time 30 \
    -X POST "${BASE}/v1/chat/completions" \
    -H "Authorization: Bearer ${TEST_AUTH_KEY}" \
    -H "X-Proxy-Mode: direct-backend" \
    -H "X-Backend-ID: ${TEST_BACKEND_ID}" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"${TEST_BACKEND_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"请只回复 OK\"}],\"max_tokens\":16}")
  smoke_content=$(jq -r '.choices[0].message.content // empty' "$smoke_file" 2>/dev/null)
  smoke_tokens=$(jq -r '.usage.total_tokens // 0' "$smoke_file" 2>/dev/null)
  smoke_error=$(jq -r '.error.message // .message // empty' "$smoke_file" 2>/dev/null)
  smoke_cache_read="$(trim "$(awk 'BEGIN{IGNORECASE=1} /^X-Cache-Read:/ {sub(/\r$/,"",$2); print $2}' "$smoke_header" | tail -1)")"

  if should_trigger_temp_backend_fallback "$smoke_code" "$smoke_content" "$smoke_tokens" "$smoke_error" "$smoke_cache_read"; then
    echo "⚠️  检测到后端 ${TEST_BACKEND_ID} 存在高风险信号（可能熔断/空响应），尝试创建临时后端..."
    TEMP_BACKEND_SOURCE_ID="${TEST_BACKEND_ID}"
    new_backend_id="$(create_temp_backend "$BASE" "$JWT" "$TEST_BACKEND_ID" "$TEST_BACKEND_TYPE" "$TEST_BACKEND_BASE_URL" "$TEST_BACKEND_KEY" "$TEST_BACKEND_MODEL" "$TEMP_BACKEND_PREFIX")"
    if [ -n "$new_backend_id" ]; then
      TEMP_BACKEND_USED="true"
      TEMP_BACKEND_FINAL_ID="$new_backend_id"
      TEMP_BACKEND_REASON="${TEMP_BACKEND_SIGNAL:-primary_backend_unhealthy}"
      TEST_BACKEND_ID="$new_backend_id"
      export TEST_BACKEND_ID
      echo "✅ 已切换到临时后端: ${TEST_BACKEND_ID}（来源: ${TEMP_BACKEND_SOURCE_ID}）"
    else
      echo "⚠️  创建临时后端失败，继续使用原后端 ${TEST_BACKEND_ID}"
    fi
  else
    if [ "$smoke_code" = "200" ] && [ -z "$smoke_content" ] && [ "${smoke_tokens:-0}" = "0" ] && [ "$smoke_cache_read" = "true" ]; then
      echo "✅ 冒烟命中缓存空结果（X-Cache-Read=true），不判定为后端异常，无需临时后端"
    else
      echo "✅ 后端 ${TEST_BACKEND_ID} 冒烟验证通过，无需临时后端"
    fi
  fi
fi

# ========================================================================
# Step D3: 重置目标后端熔断器（避免 smart-scheduling 误失败）
# ========================================================================
echo ""
echo "=== Step D3: 重置后端熔断器 ==="
reset_backend_circuit_breaker "$BASE" "$JWT" "$TEST_BACKEND_ID"
if [ -n "$TEMP_BACKEND_SOURCE_ID" ] && [ "$TEMP_BACKEND_SOURCE_ID" != "$TEST_BACKEND_ID" ]; then
  reset_backend_circuit_breaker "$BASE" "$JWT" "$TEMP_BACKEND_SOURCE_ID"
fi

# ========================================================================
# Step E: 更新流水线节点
# ========================================================================
echo ""
echo "=== Step E: 更新流水线节点 (${TEST_BACKEND_ID}/${TEST_BACKEND_MODEL}) ==="

PIPELINE_UPDATE_CMDS_FILE="/tmp/wizard_pipeline_update_cmds.json"
echo "[" > "$PIPELINE_UPDATE_CMDS_FILE"
PIPELINE_CMD_SEP=""

IFS=',' read -ra PIPELINES <<< "${TEST_PIPELINES}"
for pid in "${PIPELINES[@]}"; do
  update_pipeline_nodes "$pid" "${TEST_BACKEND_ID}" "${TEST_BACKEND_MODEL}" "$BASE" "$JWT"
done
echo "]" >> "$PIPELINE_UPDATE_CMDS_FILE"
echo ""

# ========================================================================
# Step F: 执行流水线测试
# ========================================================================
echo "=========================================="
echo "  开始流水线测试"
  echo "  版本: ${CENTAG_DEPLOY_TYPE:-personal}"
echo "  后端: ${TEST_BACKEND_ID} / ${TEST_BACKEND_MODEL}"
echo "  流水线: ${TEST_PIPELINES}"
echo "=========================================="
echo ""

TOTAL=${#PIPELINES[@]}
PASSED=0
FAILED=0
RESULTS=""
DETAILS=""

TEST_DATA_FILE="/tmp/wizard_test_data.json"
echo "[" > "$TEST_DATA_FILE"
TEST_DATA_SEP=""

for pipeline_id in "${PIPELINES[@]}"; do
  MODE="${pipeline_id}"
  MODE_SHORTCUT="$(shortcut_for_pipeline "$pipeline_id")"
  REQUEST_CONTENT="用一句话介绍你自己"
  INVOKE_METHOD="header-mode"
  if [ "$HEADER_OVERRIDE_SUPPORTED" != "true" ] && [ -n "$MODE_SHORTCUT" ]; then
    REQUEST_CONTENT="${MODE_SHORTCUT} 用一句话介绍你自己"
    INVOKE_METHOD="prompt-shortcut-fallback"
  fi

  echo -n "  [${pipeline_id}] ... "

  RESULT_FILE="/tmp/wizard_test_${pipeline_id}.json"
  HEADER_FILE="/tmp/wizard_test_${pipeline_id}_headers.txt"

  if [ "$INVOKE_METHOD" = "header-mode" ]; then
    CURL_CMD="curl -s -w \"\\n%{http_code}\" --max-time 60 -X POST \"${BASE}/v1/chat/completions\" -H \"Authorization: Bearer \${TEST_AUTH_KEY}\" -H \"X-Proxy-Mode: ${MODE}\" -H \"Content-Type: application/json\" -d '{\"model\":\"${TEST_BACKEND_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"${REQUEST_CONTENT}\"}],\"max_tokens\":50}'"
  else
    CURL_CMD="curl -s -w \"\\n%{http_code}\" --max-time 60 -X POST \"${BASE}/v1/chat/completions\" -H \"Authorization: Bearer \${TEST_AUTH_KEY}\" -H \"Content-Type: application/json\" -d '{\"model\":\"${TEST_BACKEND_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"${REQUEST_CONTENT}\"}],\"max_tokens\":50}'"
  fi

  TEST_START=$(date +%s)
  CURL_EXIT=0

  if [ "$INVOKE_METHOD" = "header-mode" ]; then
    HTTP_CODE=$(curl -s -o "$RESULT_FILE" -D "$HEADER_FILE" -w "%{http_code}" --max-time 60 \
      -X POST "${BASE}/v1/chat/completions" \
      -H "Authorization: Bearer ${TEST_AUTH_KEY}" \
      -H "X-Proxy-Mode: ${MODE}" \
      -H "Content-Type: application/json" \
      -d "{\"model\":\"${TEST_BACKEND_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"${REQUEST_CONTENT}\"}],\"max_tokens\":50}")
    CURL_EXIT=$?
  else
    HTTP_CODE=$(curl -s -o "$RESULT_FILE" -D "$HEADER_FILE" -w "%{http_code}" --max-time 60 \
      -X POST "${BASE}/v1/chat/completions" \
      -H "Authorization: Bearer ${TEST_AUTH_KEY}" \
      -H "Content-Type: application/json" \
      -d "{\"model\":\"${TEST_BACKEND_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"${REQUEST_CONTENT}\"}],\"max_tokens\":50}")
    CURL_EXIT=$?
  fi

  TEST_END=$(date +%s)
  TEST_DURATION=$((TEST_END - TEST_START))

  CONTENT=$(jq -r '.choices[0].message.content // empty' "$RESULT_FILE" 2>/dev/null)
  TOKENS=$(jq -r '.usage.total_tokens // 0' "$RESULT_FILE" 2>/dev/null)
  PROMPT_TOKENS=$(jq -r '.usage.prompt_tokens // 0' "$RESULT_FILE" 2>/dev/null)
  COMPLETION_TOKENS=$(jq -r '.usage.completion_tokens // 0' "$RESULT_FILE" 2>/dev/null)
  ERROR=$(jq -r '.error.message // .error // .message // empty' "$RESULT_FILE" 2>/dev/null)
  ERROR_TYPE=$(jq -r '.error.type // empty' "$RESULT_FILE" 2>/dev/null)
  MODEL_RETURNED=$(jq -r '.model // empty' "$RESULT_FILE" 2>/dev/null)
  RESP_SNIPPET=$(jq -c '.choices[0].message // .' "$RESULT_FILE" 2>/dev/null | head -c 500)

  RESP_HEADERS_SNIPPET=$(grep -iE '^(x-proxy|content-type|x-request-id|x-pipeline|server|x-duration)' "$HEADER_FILE" 2>/dev/null | head -c 1000)
  RESP_PIPELINE_ID=$(awk 'BEGIN{IGNORECASE=1} /^X-Pipeline-Id:/ {sub(/\r$/,"",$2); print $2}' "$HEADER_FILE" | tail -1 | tr -d '\r')

  # smart-scheduling 在冷启动时可能出现首个请求连接超时，做一次受控重试以避免假失败
  if [ "$pipeline_id" = "smart-scheduling" ] && [ "$HTTP_CODE" = "000" ]; then
    echo "⚠️ 首次请求 HTTP 000，执行一次重试..." >&2
    sleep 1
    TEST_START=$(date +%s)
    if [ "$INVOKE_METHOD" = "header-mode" ]; then
      HTTP_CODE=$(curl -s -o "$RESULT_FILE" -D "$HEADER_FILE" -w "%{http_code}" --max-time 60 \
        -X POST "${BASE}/v1/chat/completions" \
        -H "Authorization: Bearer ${TEST_AUTH_KEY}" \
        -H "X-Proxy-Mode: ${MODE}" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"${TEST_BACKEND_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"${REQUEST_CONTENT}\"}],\"max_tokens\":50}")
      CURL_EXIT=$?
    else
      HTTP_CODE=$(curl -s -o "$RESULT_FILE" -D "$HEADER_FILE" -w "%{http_code}" --max-time 60 \
        -X POST "${BASE}/v1/chat/completions" \
        -H "Authorization: Bearer ${TEST_AUTH_KEY}" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"${TEST_BACKEND_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"${REQUEST_CONTENT}\"}],\"max_tokens\":50}")
      CURL_EXIT=$?
    fi
    TEST_END=$(date +%s)
    TEST_DURATION=$((TEST_END - TEST_START))
    CONTENT=$(jq -r '.choices[0].message.content // empty' "$RESULT_FILE" 2>/dev/null)
    TOKENS=$(jq -r '.usage.total_tokens // 0' "$RESULT_FILE" 2>/dev/null)
    PROMPT_TOKENS=$(jq -r '.usage.prompt_tokens // 0' "$RESULT_FILE" 2>/dev/null)
    COMPLETION_TOKENS=$(jq -r '.usage.completion_tokens // 0' "$RESULT_FILE" 2>/dev/null)
    ERROR=$(jq -r '.error.message // .error // .message // empty' "$RESULT_FILE" 2>/dev/null)
    ERROR_TYPE=$(jq -r '.error.type // empty' "$RESULT_FILE" 2>/dev/null)
    MODEL_RETURNED=$(jq -r '.model // empty' "$RESULT_FILE" 2>/dev/null)
    RESP_SNIPPET=$(jq -c '.choices[0].message // .' "$RESULT_FILE" 2>/dev/null | head -c 500)
    RESP_HEADERS_SNIPPET=$(grep -iE '^(x-proxy|content-type|x-request-id|x-pipeline|server|x-duration)' "$HEADER_FILE" 2>/dev/null | head -c 1000)
    RESP_PIPELINE_ID=$(awk 'BEGIN{IGNORECASE=1} /^X-Pipeline-Id:/ {sub(/\r$/,"",$2); print $2}' "$HEADER_FILE" | tail -1 | tr -d '\r')
  fi

  HTTP_CODE_NUM="$(normalize_http_code "$HTTP_CODE")"
  if [ -z "$ERROR" ] && [ "$HTTP_CODE_NUM" = "0" ] && [ "${CURL_EXIT:-0}" != "0" ]; then
    ERROR="curl_exit_${CURL_EXIT}"
  fi

  # 注意：此处不能使用 local（顶层 for 循环不在函数内）
  passed="false"
  status_text=""
  analysis=""

  if [ "$HTTP_CODE_NUM" = "200" ]; then
    if [ -n "$CONTENT" ] || [ "$TOKENS" -gt 0 ]; then
      if [ "$RESP_PIPELINE_ID" = "$pipeline_id" ]; then
        echo "✅ HTTP 200, tokens=${TOKENS}, pipeline=${RESP_PIPELINE_ID}"
        PASSED=$((PASSED + 1))
        passed="true"
        status_text="✅ 通过"
        analysis="流水线返回成功，且响应头 X-Pipeline-Id 与目标流水线一致，模式命中正确。"
        RESULTS="${RESULTS}✅ ${pipeline_id} — HTTP 200, tokens=${TOKENS}, pipeline=${RESP_PIPELINE_ID}\n"
        DETAILS="${DETAILS}| ${pipeline_id} | ✅ 通过 | ${TOKENS} | 命中 ${RESP_PIPELINE_ID} |\n"
      else
        echo "❌ HTTP 200 但命中流水线不一致 (expected=${pipeline_id}, actual=${RESP_PIPELINE_ID:-空})"
        FAILED=$((FAILED + 1))
        passed="false"
        status_text="❌ 失败（流水线命中不一致）"
        analysis="HTTP 返回成功但 X-Pipeline-Id 与目标流水线不一致，说明模式触发/解析链路存在偏差，不能判定该流水线已被正确测试。"
        RESULTS="${RESULTS}❌ ${pipeline_id} — HTTP 200, pipeline mismatch (${RESP_PIPELINE_ID})\n"
        DETAILS="${DETAILS}| ${pipeline_id} | ❌ 失败 | ${TOKENS} | 命中流水线不一致: ${RESP_PIPELINE_ID} |\n"
      fi
    else
      echo "❌ HTTP 200, 空响应+tokens=0（后端错误可能被吞没）"
      FAILED=$((FAILED + 1))
      passed="false"
      status_text="❌ 失败（HTTP 200 空响应）"
      analysis="HTTP 返回 200 但响应内容为空且 tokens=0。这通常表示 Centag 代理层吞没了后端返回的真实错误。可能原因：(1) 后端 API Key 无效或已过期 (2) 账户余额不足 (3) 模型名称不存在 (4) 后端限流/过载。建议用 curl 直连后端 API 确认 Key 可用性和模型是否存在。"
      RESULTS="${RESULTS}❌ ${pipeline_id} — HTTP 200, 空响应+tokens=0（后端错误吞没）\n"
      DETAILS="${DETAILS}| ${pipeline_id} | ❌ 失败 | 0 | HTTP 200 空响应（后端错误吞没）|\n"
    fi
  else
    echo "❌ HTTP ${HTTP_CODE}, ${ERROR:-无错误信息}"
    FAILED=$((FAILED + 1))
    passed="false"
    status_text="❌ 失败 (HTTP ${HTTP_CODE})"
    analysis="请求返回 HTTP ${HTTP_CODE}。错误信息: ${ERROR:-无}。可能原因: 流水线配置异常、节点不存在、后端服务完全不可达或认证配置错误。"
    RESULTS="${RESULTS}❌ ${pipeline_id} — HTTP ${HTTP_CODE}, ${ERROR}\n"
    DETAILS="${DETAILS}| ${pipeline_id} | ❌ 失败 | 0 | HTTP ${HTTP_CODE}: ${ERROR} |\n"
  fi

  safe_content=$(echo "$CONTENT" | jq -Rs '.' 2>/dev/null)
  safe_error=$(echo "$ERROR" | jq -Rs '.' 2>/dev/null)
  safe_error_type=$(echo "$ERROR_TYPE" | jq -Rs '.' 2>/dev/null)
  safe_analysis=$(echo "$analysis" | jq -Rs '.' 2>/dev/null)
  safe_curl_cmd=$(echo "$CURL_CMD" | jq -Rs '.' 2>/dev/null)
  safe_resp_headers=$(echo "$RESP_HEADERS_SNIPPET" | jq -Rs '.' 2>/dev/null)
  safe_resp_snippet=$(echo "$RESP_SNIPPET" | jq -Rs '.' 2>/dev/null)

  echo "${TEST_DATA_SEP}{ \"pipeline\": \"${pipeline_id}\", \"mode\": \"${MODE}\", \"invoke_method\": \"${INVOKE_METHOD}\", \"expected_pipeline_id\": \"${pipeline_id}\", \"response_pipeline_id\": \"${RESP_PIPELINE_ID}\", \"passed\": ${passed}, \"http_code\": ${HTTP_CODE_NUM}, \"status_text\": \"${status_text}\", \"content\": ${safe_content}, \"tokens\": ${TOKENS}, \"prompt_tokens\": ${PROMPT_TOKENS}, \"completion_tokens\": ${COMPLETION_TOKENS}, \"duration_s\": ${TEST_DURATION}, \"model_returned\": \"${MODEL_RETURNED}\", \"error\": ${safe_error}, \"error_type\": ${safe_error_type}, \"analysis\": ${safe_analysis}, \"curl_cmd\": ${safe_curl_cmd}, \"resp_headers_snippet\": ${safe_resp_headers}, \"resp_snippet\": ${safe_resp_snippet} }" >> "$TEST_DATA_FILE"
  TEST_DATA_SEP=","

  sleep 1
done

echo "]" >> "$TEST_DATA_FILE"

# ========================================================================
# Step G: 入口变体矩阵补充验证（可选）
# ========================================================================
MATRIX_TOTAL=0
MATRIX_PASSED=0
MATRIX_SKIPPED=0
MATRIX_FILE="/tmp/wizard_variant_matrix.json"
echo "[" > "$MATRIX_FILE"
MATRIX_SEP=""

echo ""
echo "=== Step G: 入口变体矩阵补充验证（可选） ==="
echo "  variants: ${ENTRY_VARIANTS}"
echo "  repeat: ${REPEAT_PER_VARIANT}"
echo "  log evidence: ${LOG_EVIDENCE_LEVEL}"

if [ -z "$REPEAT_PER_VARIANT" ] || [ "$REPEAT_PER_VARIANT" -lt 1 ] 2>/dev/null; then
  REPEAT_PER_VARIANT=1
fi

IFS=',' read -ra VARIANTS <<< "${ENTRY_VARIANTS}"
for pipeline_id in "${PIPELINES[@]}"; do
  shortcut_code="$(shortcut_for_pipeline "$pipeline_id")"

  for raw_variant in "${VARIANTS[@]}"; do
    variant="$(trim "$raw_variant")"
    if [ -z "$variant" ]; then
      continue
    fi

    for ((i=1; i<=REPEAT_PER_VARIANT; i++)); do
      MATRIX_TOTAL=$((MATRIX_TOTAL + 1))
      req_tag="WIZARD_${pipeline_id}_${variant}_${i}_$(date +%s)"

      req_mode_header=""
      req_prompt_prefix=""
      req_model_name="${TEST_BACKEND_MODEL}"
      matrix_log_resolved_cmd=""
      matrix_log_finished_cmd=""
      resolved_request_ids=""
      finished_request_ids=""
      resolved_full_text=""
      finished_full_text=""

      case "$variant" in
        header-full)
          req_mode_header="$pipeline_id"
          ;;
        model-pipeline)
          req_model_name="pipeline.${pipeline_id}"
          ;;
        prompt-shortcut)
          if [ -n "$shortcut_code" ]; then
            req_prompt_prefix="${shortcut_code} "
          fi
          ;;
        *)
          echo "  ⚠️  ${pipeline_id} / ${variant}：未知入口变体，跳过"
          MATRIX_TOTAL=$((MATRIX_TOTAL - 1))
          continue
          ;;
      esac

      if [ "$HEADER_OVERRIDE_SUPPORTED" != "true" ] && [ "$variant" = "header-full" ]; then
        MATRIX_TOTAL=$((MATRIX_TOTAL - 1))
        MATRIX_SKIPPED=$((MATRIX_SKIPPED + 1))
        safe_variant=$(json_escape "$variant")
        safe_req_tag=$(json_escape "$req_tag")
        safe_note=$(json_escape "已跳过：X-Proxy-Mode Header 当前未生效（allow_header_override 可能关闭）")
        echo "${MATRIX_SEP}{\"pipeline\": \"${pipeline_id}\", \"variant\": ${safe_variant}, \"try\": ${i}, \"skipped\": true, \"passed\": false, \"http_code\": 0, \"req_tag\": ${safe_req_tag}, \"note\": ${safe_note}}" >> "$MATRIX_FILE"
        MATRIX_SEP=","
        echo "  ⏭️  [${pipeline_id}] ${variant} #${i} 跳过（Header 未生效）"
        continue
      fi

      matrix_result_file="/tmp/wizard_matrix_${pipeline_id}_${variant}_${i}.json"
      matrix_header_file="/tmp/wizard_matrix_${pipeline_id}_${variant}_${i}_headers.txt"
      matrix_request_body="{\"model\":\"${req_model_name}\",\"messages\":[{\"role\":\"user\",\"content\":\"${req_prompt_prefix}请严格回复：${req_tag}\"}],\"temperature\":0,\"max_tokens\":64}"
      if [ -n "$req_mode_header" ]; then
        matrix_curl_cmd="curl -s --max-time 60 -X POST \"${BASE}/v1/chat/completions\" -H \"Authorization: Bearer \${TEST_AUTH_KEY}\" -H \"X-Proxy-Mode: ${req_mode_header}\" -H \"Content-Type: application/json\" -d '${matrix_request_body}'"
      else
        matrix_curl_cmd="curl -s --max-time 60 -X POST \"${BASE}/v1/chat/completions\" -H \"Authorization: Bearer \${TEST_AUTH_KEY}\" -H \"Content-Type: application/json\" -d '${matrix_request_body}'"
      fi

      matrix_start_iso="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
      matrix_start_epoch="$(date +%s)"

      if [ -n "$req_mode_header" ]; then
        matrix_http_code=$(curl -s -o "$matrix_result_file" -D "$matrix_header_file" -w "%{http_code}" --max-time 60 \
          -X POST "${BASE}/v1/chat/completions" \
          -H "Authorization: Bearer ${TEST_AUTH_KEY}" \
          -H "X-Proxy-Mode: ${req_mode_header}" \
          -H "Content-Type: application/json" \
          -d "${matrix_request_body}")
      else
        matrix_http_code=$(curl -s -o "$matrix_result_file" -D "$matrix_header_file" -w "%{http_code}" --max-time 60 \
          -X POST "${BASE}/v1/chat/completions" \
          -H "Authorization: Bearer ${TEST_AUTH_KEY}" \
          -H "Content-Type: application/json" \
          -d "${matrix_request_body}")
      fi

      matrix_end_epoch="$(date +%s)"
      matrix_window_from_epoch=$((matrix_start_epoch - 1))
      matrix_window_to_epoch=$((matrix_end_epoch + 2))
      matrix_start_iso="$(date -u -r "$matrix_window_from_epoch" +"%Y-%m-%dT%H:%M:%SZ")"
      matrix_end_iso="$(date -u -r "$matrix_window_to_epoch" +"%Y-%m-%dT%H:%M:%SZ")"
      matrix_elapsed="$((matrix_end_epoch - matrix_start_epoch))"

      matrix_tokens=$(jq -r '.usage.total_tokens // 0' "$matrix_result_file" 2>/dev/null)
      matrix_content=$(jq -r '.choices[0].message.content // empty' "$matrix_result_file" 2>/dev/null)
      matrix_error=$(jq -r '.error.message // .message // empty' "$matrix_result_file" 2>/dev/null)

      resp_x_proxy_mode="$(trim "$(awk 'BEGIN{IGNORECASE=1} /^X-Proxy-Mode:/ {sub(/\r$/,"",$2); print $2}' "$matrix_header_file" | tail -1)")"
      resp_x_pipeline_id="$(trim "$(awk 'BEGIN{IGNORECASE=1} /^X-Pipeline-Id:/ {sub(/\r$/,"",$2); print $2}' "$matrix_header_file" | tail -1)")"
      resp_x_backend_id="$(trim "$(awk 'BEGIN{IGNORECASE=1} /^X-Backend-Id:/ {sub(/\r$/,"",$2); print $2}' "$matrix_header_file" | tail -1)")"
      resp_x_pipeline_success="$(trim "$(awk 'BEGIN{IGNORECASE=1} /^X-Pipeline-Success:/ {sub(/\r$/,"",$2); print $2}' "$matrix_header_file" | tail -1)")"

      resolved_hits=0
      finished_hits=0
      finished_pipeline_match=0
      finished_primary_request_id=""
      log_evidence_strength="basic"

      if [ "$LOG_EVIDENCE_LEVEL" = "full" ]; then
        q_resolved=$(url_encode "Resolved pipeline: ${pipeline_id}")
        q_finished=$(url_encode "pipeline execution finished")
        from_q=$(url_encode "$matrix_start_iso")
        to_q=$(url_encode "$matrix_end_iso")
        matrix_log_resolved_cmd="curl -s -G \"${BASE}/api/v1/logs\" -H \"Authorization: Bearer \${JWT}\" --data-urlencode \"limit=100\" --data-urlencode \"category=llm\" --data-urlencode \"from=${matrix_start_iso}\" --data-urlencode \"to=${matrix_end_iso}\" --data-urlencode \"q=Resolved pipeline: ${pipeline_id}\""
        matrix_log_finished_cmd="curl -s -G \"${BASE}/api/v1/logs\" -H \"Authorization: Bearer \${JWT}\" --data-urlencode \"limit=100\" --data-urlencode \"category=llm\" --data-urlencode \"from=${matrix_start_iso}\" --data-urlencode \"to=${matrix_end_iso}\" --data-urlencode \"q=pipeline execution finished\""

        resolved_resp=$(curl -s --max-time 15 \
          -H "Authorization: Bearer ${JWT}" \
          "${BASE}/api/v1/logs?limit=100&category=llm&q=${q_resolved}&from=${from_q}&to=${to_q}")
        resolved_hits=$(echo "$resolved_resp" | jq -r '.data.logs | length // 0' 2>/dev/null)
        resolved_request_ids=$(echo "$resolved_resp" | jq -r '[(.data.logs // [])[] | .request_id // empty | select(. != "")] | unique | join(",")' 2>/dev/null)
        resolved_full_text=$(echo "$resolved_resp" | jq -r '((.data.logs // []) | map("[\(.timestamp // .time // .created_at // "-")] [\(.level // "-")] [req=\(.request_id // "-")] \(.message // .msg // .content // "-")\nextra=\((.extra // {}) | tostring)") | join("\n\n")) // ""' 2>/dev/null)

        finished_resp=$(curl -s --max-time 15 \
          -H "Authorization: Bearer ${JWT}" \
          "${BASE}/api/v1/logs?limit=100&category=llm&q=${q_finished}&from=${from_q}&to=${to_q}")
        finished_hits=$(echo "$finished_resp" | jq -r '.data.logs | length // 0' 2>/dev/null)
        finished_pipeline_match=$(echo "$finished_resp" | jq -r --arg p "$pipeline_id" '[(.data.logs // [])[] | select((.extra.pipeline_id // "") == $p)] | length' 2>/dev/null)
        finished_request_ids=$(echo "$finished_resp" | jq -r --arg p "$pipeline_id" '[(.data.logs // [])[] | select((.extra.pipeline_id // "") == $p) | .request_id // empty | select(. != "")] | unique | join(",")' 2>/dev/null)
        finished_primary_request_id=$(echo "$finished_resp" | jq -r --arg p "$pipeline_id" '((.data.logs // []) | map(select((.extra.pipeline_id // "") == $p)) | sort_by(.timestamp // .time // .created_at // "") | reverse | .[0].request_id) // ""' 2>/dev/null)
        finished_full_text=$(echo "$finished_resp" | jq -r --arg p "$pipeline_id" '((.data.logs // []) | map(select((.extra.pipeline_id // "") == $p)) | map("[\(.timestamp // .time // .created_at // "-")] [\(.level // "-")] [req=\(.request_id // "-")] \(.message // .msg // .content // "-")\nextra=\((.extra // {}) | tostring)") | join("\n\n")) // ""' 2>/dev/null)
      fi

      if [ -z "$resolved_full_text" ]; then
        resolved_full_text="(未采集或无匹配日志)"
      fi
      if [ -z "$finished_full_text" ]; then
        finished_full_text="(未采集或无匹配日志)"
      fi
      if [ "$LOG_EVIDENCE_LEVEL" = "full" ]; then
        if [ "$finished_pipeline_match" = "1" ]; then
          log_evidence_strength="high"
        elif [ "$finished_pipeline_match" -gt 1 ] || [ "$resolved_hits" -gt 0 ]; then
          log_evidence_strength="medium"
        else
          log_evidence_strength="low"
        fi
      fi

      matrix_passed="false"
      matrix_note=""
      if [ "$matrix_http_code" = "200" ] && [ "$resp_x_pipeline_id" = "$pipeline_id" ]; then
        matrix_passed="true"
        matrix_note="HTTP 与响应头通过"
      else
        matrix_note="HTTP/响应头不满足预期"
      fi

      if [ "$LOG_EVIDENCE_LEVEL" = "full" ] && [ "$finished_pipeline_match" -lt 1 ] && [ "$resolved_hits" -lt 1 ]; then
        matrix_passed="false"
        matrix_note="${matrix_note}; 日志证据未匹配"
      fi
      if [ "$LOG_EVIDENCE_LEVEL" = "full" ] && [ "$finished_pipeline_match" -gt 1 ]; then
        matrix_note="${matrix_note}; 日志窗口内存在多条同流水线 finished，已展示全部证据"
      fi

      if [ "$matrix_passed" = "true" ]; then
        MATRIX_PASSED=$((MATRIX_PASSED + 1))
        echo "  ✅ [${pipeline_id}] ${variant} #${i} 通过 (HTTP ${matrix_http_code}, X-Pipeline-Id=${resp_x_pipeline_id})"
      else
        echo "  ❌ [${pipeline_id}] ${variant} #${i} 失败 (HTTP ${matrix_http_code}, X-Pipeline-Id=${resp_x_pipeline_id}, err=${matrix_error:-无})"
      fi

      safe_variant=$(json_escape "$variant")
      safe_req_mode_header=$(json_escape "$req_mode_header")
      safe_req_prompt_prefix=$(json_escape "$req_prompt_prefix")
      safe_req_model_name=$(json_escape "$req_model_name")
      safe_req_tag=$(json_escape "$req_tag")
      safe_matrix_curl_cmd=$(json_escape "$matrix_curl_cmd")
      safe_matrix_log_resolved_cmd=$(json_escape "$matrix_log_resolved_cmd")
      safe_matrix_log_finished_cmd=$(json_escape "$matrix_log_finished_cmd")
      safe_resolved_request_ids=$(json_escape "$resolved_request_ids")
      safe_finished_request_ids=$(json_escape "$finished_request_ids")
      safe_finished_primary_request_id=$(json_escape "$finished_primary_request_id")
      safe_resolved_full_text=$(json_escape "$resolved_full_text")
      safe_finished_full_text=$(json_escape "$finished_full_text")
      safe_log_evidence_strength=$(json_escape "$log_evidence_strength")
      safe_matrix_note=$(json_escape "$matrix_note")
      safe_matrix_error=$(json_escape "$matrix_error")
      safe_matrix_content=$(json_escape "$matrix_content")
      safe_resp_x_proxy_mode=$(json_escape "$resp_x_proxy_mode")
      safe_resp_x_pipeline_id=$(json_escape "$resp_x_pipeline_id")
      safe_resp_x_backend_id=$(json_escape "$resp_x_backend_id")
      safe_resp_x_pipeline_success=$(json_escape "$resp_x_pipeline_success")

      echo "${MATRIX_SEP}{\"pipeline\": \"${pipeline_id}\", \"variant\": ${safe_variant}, \"try\": ${i}, \"passed\": ${matrix_passed}, \"http_code\": ${matrix_http_code}, \"elapsed_s\": ${matrix_elapsed}, \"req_mode_header\": ${safe_req_mode_header}, \"req_prompt_prefix\": ${safe_req_prompt_prefix}, \"req_model\": ${safe_req_model_name}, \"req_tag\": ${safe_req_tag}, \"curl_cmd\": ${safe_matrix_curl_cmd}, \"resp_x_proxy_mode\": ${safe_resp_x_proxy_mode}, \"resp_x_pipeline_id\": ${safe_resp_x_pipeline_id}, \"resp_x_backend_id\": ${safe_resp_x_backend_id}, \"resp_x_pipeline_success\": ${safe_resp_x_pipeline_success}, \"tokens\": ${matrix_tokens}, \"content\": ${safe_matrix_content}, \"error\": ${safe_matrix_error}, \"log_resolved_hits\": ${resolved_hits}, \"log_finished_hits\": ${finished_hits}, \"log_finished_pipeline_match\": ${finished_pipeline_match}, \"log_finished_primary_request_id\": ${safe_finished_primary_request_id}, \"log_evidence_strength\": ${safe_log_evidence_strength}, \"log_resolved_request_ids\": ${safe_resolved_request_ids}, \"log_finished_request_ids\": ${safe_finished_request_ids}, \"log_resolved_full\": ${safe_resolved_full_text}, \"log_finished_full\": ${safe_finished_full_text}, \"log_resolved_cmd\": ${safe_matrix_log_resolved_cmd}, \"log_finished_cmd\": ${safe_matrix_log_finished_cmd}, \"note\": ${safe_matrix_note}, \"window_from\": \"${matrix_start_iso}\", \"window_to\": \"${matrix_end_iso}\"}" >> "$MATRIX_FILE"
      MATRIX_SEP=","

      sleep 1
    done
  done
done

echo "]" >> "$MATRIX_FILE"

if [ "$MATRIX_TOTAL" -gt 0 ]; then
  MATRIX_PASS_RATE=$(( MATRIX_PASSED * 100 / MATRIX_TOTAL ))
else
  MATRIX_PASS_RATE=0
fi

echo ""
echo "=== Step G 结果 ==="
echo "  入口变体补充验证: ${MATRIX_PASSED}/${MATRIX_TOTAL} 通过"
echo "  跳过: ${MATRIX_SKIPPED}"
echo "  矩阵结果文件: ${MATRIX_FILE}"
echo "  通过率: ${MATRIX_PASS_RATE}%"

cat > /tmp/wizard_runtime_context.json << EOF
{
  "temp_backend_used": ${TEMP_BACKEND_USED},
  "temp_backend_source_id": "${TEMP_BACKEND_SOURCE_ID}",
  "temp_backend_final_id": "${TEMP_BACKEND_FINAL_ID}",
  "temp_backend_reason": "${TEMP_BACKEND_REASON}",
  "temp_backend_signal": "${TEMP_BACKEND_SIGNAL}",
  "final_backend_id": "${TEST_BACKEND_ID}",
  "header_override_supported": "${HEADER_OVERRIDE_SUPPORTED}",
  "temp_backend_fallback_mode": "${TEMP_BACKEND_FALLBACK}"
}
EOF

if [ $PASSED -eq 0 ] && [ $FAILED -gt 0 ]; then
  echo ""
  echo "⚠️ =========================================="
  echo "⚠️  所有流水线均返回空响应（content 空 + tokens=0）"
  echo "⚠️ =========================================="
fi

# ========================================================================
# 导出结果供报告脚本使用
# ========================================================================
echo ""
echo "=========================================="
echo "  测试完成: ${PASSED}/${TOTAL} 通过, ${FAILED}/${TOTAL} 失败"
echo "  数据文件已保存到 /tmp/wizard_test_data.json"
echo "  探测结果已保存到 /tmp/wizard_probe.json"
echo ""
echo "  下一步: 运行报告生成脚本"
echo "    python3 docs/harness/skills/wizard-report.py"
echo "=========================================="

# 写结果到文件供 Python 读取
PASS_RATE=0
if [ "$TOTAL" -gt 0 ]; then
  PASS_RATE=$(( PASSED * 100 / TOTAL ))
fi

cat > /tmp/wizard_test_results.sh << EOF
export PASSED=${PASSED}
export FAILED=${FAILED}
export TOTAL=${TOTAL}
export PASS_RATE=${PASS_RATE}
export MATRIX_PASSED=${MATRIX_PASSED}
export MATRIX_TOTAL=${MATRIX_TOTAL}
export MATRIX_PASS_RATE=${MATRIX_PASS_RATE}
export MATRIX_SKIPPED=${MATRIX_SKIPPED}
export MATRIX_FILE="${MATRIX_FILE}"
export TEMP_BACKEND_USED="${TEMP_BACKEND_USED}"
export TEMP_BACKEND_SOURCE_ID="${TEMP_BACKEND_SOURCE_ID}"
export TEMP_BACKEND_FINAL_ID="${TEMP_BACKEND_FINAL_ID}"
export TEMP_BACKEND_REASON="${TEMP_BACKEND_REASON}"
export TEMP_BACKEND_SIGNAL="${TEMP_BACKEND_SIGNAL}"
export DETAILS="${DETAILS}"
EOF

exit 0
