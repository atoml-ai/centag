#!/usr/bin/env bash
# Smoke test for system-update auth/permission checks.
#
# Covers 3 scenarios:
#   1) No token
#   2) Invalid token
#   3) Valid token (if provided)
#
# Usage:
#   CENTAG_VALID_TOKEN=llmproxy_xxx ./scripts/test/e2e/system-update-auth-smoke.sh
#
set -euo pipefail

BASE_URL="${CENTAG_BASE_URL:-http://localhost:20060}"
UPDATE_HISTORY_URL="${BASE_URL}/api/v1/system/update/history"
VALID_TOKEN="${CENTAG_VALID_TOKEN:-}"

if [[ -z "${VALID_TOKEN}" ]]; then
  for key_var in LLM_PROXY_DEFAULT_ADMIN_API_KEY LLM_PROXY_ADMIN_API_KEY CENTAG_API_KEY; do
    candidate="${!key_var:-}"
    if [[ -n "${candidate}" ]]; then
      VALID_TOKEN="${candidate}"
      break
    fi
  done
fi

say() {
  echo "==> $*"
}

warn() {
  echo "warn: $*" >&2
}

check_health() {
  say "检查服务健康: ${BASE_URL}/health"
  if ! curl -fsS "${BASE_URL}/health" >/dev/null; then
    echo "error: 服务不可用，请先启动 Centag (${BASE_URL})" >&2
    exit 1
  fi
}

probe_case() {
  local case_name="$1"
  local auth_value="$2"
  local expect="$3"

  local body_file
  body_file="$(mktemp)"
  local code

  if [[ -n "${auth_value}" ]]; then
    code="$(curl -sS -o "${body_file}" -w "%{http_code}" -H "Authorization: ${auth_value}" "${UPDATE_HISTORY_URL}")"
  else
    code="$(curl -sS -o "${body_file}" -w "%{http_code}" "${UPDATE_HISTORY_URL}")"
  fi

  local body
  body="$(cat "${body_file}")"
  rm -f "${body_file}"

  local pass=false
  case "${expect}" in
    non2xx)
      if [[ ! "${code}" =~ ^2[0-9][0-9]$ ]]; then
        pass=true
      fi
      ;;
    success2xx)
      if [[ "${code}" =~ ^2[0-9][0-9]$ ]]; then
        pass=true
      fi
      ;;
    *)
      echo "error: unsupported expectation ${expect}" >&2
      exit 1
      ;;
  esac

  if [[ "${pass}" == "true" ]]; then
    echo "[PASS] ${case_name} -> HTTP ${code}"
  else
    echo "[FAIL] ${case_name} -> HTTP ${code}"
    echo "       response: ${body}"
    return 1
  fi
}

main() {
  check_health

  say "用例 1: 无 Token（预期非 2xx）"
  probe_case "no-token" "" "non2xx"

  say "用例 2: 错误 Token（预期非 2xx）"
  probe_case "bad-token" "Bearer llmproxy_invalid_token_for_smoke_test" "non2xx"

  if [[ -z "${VALID_TOKEN}" ]]; then
    warn "未提供有效 Token，跳过用例 3（可设置 CENTAG_VALID_TOKEN）"
    exit 0
  fi

  if [[ "${VALID_TOKEN}" =~ ^[Bb]earer[[:space:]]+ ]]; then
    say "用例 3: 正确 Token（预期 2xx）"
    probe_case "valid-token" "${VALID_TOKEN}" "success2xx"
  else
    say "用例 3: 正确 Token（预期 2xx）"
    probe_case "valid-token" "Bearer ${VALID_TOKEN}" "success2xx"
  fi
}

main "$@"
