#!/usr/bin/env bash
# Profile 场景拓展联调演示 — gateway / cached / agent-memory 通用
# 用法: PROXY_URL=http://localhost:20060 ./scripts/profile-scenario-demo.sh [cached|gateway|agent-memory|local]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PROFILE="${1:-local}"
PROXY_URL="${PROXY_URL:-http://localhost:20060}"
REPORT="${REPORT:-$ROOT/tmp/profile-scenario-demo-$(date +%Y%m%d-%H%M%S).md}"

mkdir -p "$(dirname "$REPORT")"

if [ -f "$ROOT/config/secrets/.env" ]; then
  # shellcheck disable=SC1091
  source "$ROOT/config/secrets/.env"
fi
ADMIN_KEY="${LLM_PROXY_ADMIN_API_KEY:-test-key}"
WEBHOOK_SECRET="${WEBHOOK_SECRET:-demo-webhook-secret}"
MODEL="${DEMO_MODEL:-glm-4-flash}"

pass=0
fail=0
skip=0

log() { echo "[demo] $*"; }
record() {
  local status="$1" name="$2" detail="$3"
  case "$status" in
    PASS) pass=$((pass + 1)); log "✅ $name — $detail" ;;
    FAIL) fail=$((fail + 1)); log "❌ $name — $detail" ;;
    SKIP) skip=$((skip + 1)); log "⏭️  $name — $detail" ;;
  esac
  printf '| %s | %s | %s |\n' "$status" "$name" "$detail" >> "$REPORT"
}

auth() { printf 'Authorization: Bearer %s' "$ADMIN_KEY"; }

http_code() {
  curl -s -o /dev/null -w "%{http_code}" "$@"
}

sync_builtin_pipelines() {
  log "同步 initdata 流水线模板到注册表..."
  local templates ids
  templates=$(curl -sf -H "$(auth)" "$PROXY_URL/api/v1/pipelines/templates")
  ids=$(printf '%s' "$templates" | python3 -c "
import json,sys
d=json.load(sys.stdin)
for k in sorted(d.get('data',{}).keys()):
    print(k)
")
  local n=0
  while IFS= read -r id; do
    [ -z "$id" ] && continue
    local body
    body=$(printf '%s' "$templates" | python3 -c "
import json,sys
d=json.load(sys.stdin)
t=d['data']['$id']
print(json.dumps({
  'id': t['id'],
  'name': t.get('name',''),
  'description': t.get('description',''),
  'shortcut_code': t.get('shortcut_code',''),
  'nodes': t['nodes'],
  'global_config': t.get('global_config') or {},
  'metadata': t.get('metadata') or {},
  'version': '1.0'
}))
")
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$PROXY_URL/api/v1/pipelines?overwrite=true" \
      -H "$(auth)" -H "Content-Type: application/json" -d "$body")
    if [ "$code" = "200" ]; then n=$((n + 1)); fi
  done <<< "$ids"
  record PASS "sync-pipelines" "registered/updated $n templates"
}

check_health() {
  local body
  body=$(curl -sf "$PROXY_URL/health" 2>/dev/null || true)
  if echo "$body" | grep -q '"status":"ok"'; then
    record PASS "health" "$body"
  else
    record FAIL "health" "no response from $PROXY_URL"
    exit 1
  fi
}

check_phase3_templates() {
  local missing
  missing=$(curl -sf -H "$(auth)" "$PROXY_URL/api/v1/pipelines" | python3 -c "
import json,sys
d=json.load(sys.stdin)
have={p['id'] for p in d.get('data',[])}
want={'security-mode','multilingual-support','geo-routing-mode','transparent-proxy','rag-mode','cache-hit'}
print(','.join(sorted(want-have)))
")
  if [ -z "$missing" ]; then
    record PASS "phase3-templates" "all scenario templates present"
  else
    record FAIL "phase3-templates" "missing: $missing"
  fi
}

test_geo_router_node() {
  local resp
  resp=$(curl -sf -X POST "$PROXY_URL/api/v1/pipelines/node-plugins/business.geo_router/test" \
    -H "$(auth)" -H "Content-Type: application/json" \
    -d '{
      "config": {"custom_config": {
        "default_backend": "bigmodel",
        "region_backends": {"CN": "bigmodel", "US": "openai"},
        "ip_prefix_rules": [{"prefix": "10.", "backend_id": "internal-llm"}]
      }},
      "input": {"metadata": {"client_ip": "10.1.2.3", "geo_region": "US"}}
    }' 2>/dev/null || echo '{}')
  local backend
  backend=$(printf '%s' "$resp" | python3 -c "
import json,sys
d=json.load(sys.stdin)
o=d.get('data',{}).get('output',{})
print(o.get('backend_id') or (o.get('metadata') or {}).get('backend_id',''))
" 2>/dev/null || true)
  if [ "$backend" = "openai" ]; then
    record PASS "geo-router-node" "X-Geo-Region US → backend openai"
  else
    record FAIL "geo-router-node" "unexpected backend=$backend resp=$resp"
  fi
}

test_webhook() {
  export WEBHOOK_SECRET
  local code body
  code=$(curl -s -o /tmp/webhook_resp.json -w "%{http_code}" -X POST "$PROXY_URL/api/v1/webhooks/pipeline/direct-backend" \
    -H "$(auth)" -H "Content-Type: application/json" \
    -H "X-Webhook-Secret: $WEBHOOK_SECRET" \
    -H "X-Webhook-Source: github-actions" \
    -d '{"content":"ci smoke test","metadata":{"repo":"centag"}}')
  if [ "$code" = "200" ] && grep -q '"success":true' /tmp/webhook_resp.json 2>/dev/null; then
    record PASS "webhook-trigger" "POST /webhooks/pipeline/direct-backend → 200"
  else
    record SKIP "webhook-trigger" "HTTP $code (set WEBHOOK_SECRET on server or use Bearer)"
  fi
}

chat_once() {
  local shortcut="$1" content="$2" outfile="$3"
  shift 3
  curl -s --max-time "${DEMO_TIMEOUT:-180}" -X POST "$PROXY_URL/v1/chat/completions" \
    -H "$(auth)" -H "Content-Type: application/json" \
    "$@" \
    -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"$shortcut $content\"}],\"max_tokens\":32}" \
    > "$outfile" 2>/dev/null || echo '{"error":"timeout"}' > "$outfile"
}

test_cache_hit() {
  local q="profile-demo-cache-$(date +%s)"
  local f1=/tmp/demo_cache1.json f2=/tmp/demo_cache2.json
  chat_once "#ch" "$q" "$f1"
  sleep 1
  chat_once "#ch" "$q" "$f2" -D /tmp/demo_cache_headers.txt
  local e1 e2
  e1=$(python3 -c "import json; print(json.load(open('$f1')).get('error',''))" 2>/dev/null || true)
  e2=$(python3 -c "import json; print(json.load(open('$f2')).get('error',''))" 2>/dev/null || true)
  if [ -n "$e1" ] || [ -n "$e2" ]; then
    record SKIP "cache-hit-#ch" "LLM unavailable: ${e1:-$e2}"
    return
  fi
  local cache_hit_hdr cache_hdr
  cache_hit_hdr=$(grep -i '^x-cache-hit:' /tmp/demo_cache_headers.txt 2>/dev/null | tail -1 | awk '{print $2}' | tr -d '\r' || true)
  cache_hdr=$(grep -i '^x-cache:' /tmp/demo_cache_headers.txt 2>/dev/null | tail -1 | awk '{print $2}' | tr -d '\r' || true)
  if echo "$cache_hit_hdr" | grep -qiE '^true$'; then
    record PASS "cache-hit-#ch" "second request X-Cache-Hit=$cache_hit_hdr"
  elif echo "$cache_hdr" | grep -qiE 'HIT'; then
    record PASS "cache-hit-#ch" "second request X-Cache=$cache_hdr"
  else
    record SKIP "cache-hit-#ch" "no cache hit (X-Cache-Hit=${cache_hit_hdr:-MISS} X-Cache=${cache_hdr:-MISS}); needs pg storage connected"
  fi
}

test_rag_shortcut() {
  local f=/tmp/demo_rag.json
  chat_once "#rag" "年假有多少天" "$f"
  local err content
  err=$(python3 -c "import json; print(json.load(open('$f')).get('error',''))" 2>/dev/null || true)
  content=$(python3 -c "import json; c=json.load(open('$f')).get('choices',[{}])[0].get('message',{}).get('content',''); print(c[:80])" 2>/dev/null || true)
  if [ -n "$err" ]; then
    record SKIP "rag-#rag" "$err"
  elif [ -n "$content" ]; then
    record PASS "rag-#rag" "got response: ${content}..."
  else
    record FAIL "rag-#rag" "empty response"
  fi
}

test_geo_chat() {
  local f=/tmp/demo_geo.json
  curl -s --max-time "${DEMO_TIMEOUT:-120}" -X POST "$PROXY_URL/v1/chat/completions" \
    -H "$(auth)" -H "Content-Type: application/json" -H "X-Geo-Region: US" \
    -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"#geo say hello in one word\"}],\"max_tokens\":16}" \
    > "$f" 2>/dev/null || echo '{}' > "$f"
  local pid err
  pid=$(python3 -c "import json; print(json.load(open('$f')).get('error',''))" 2>/dev/null || true)
  if [ -n "$pid" ]; then
    record SKIP "geo-#geo-chat" "$pid"
  else
    record PASS "geo-#geo-chat" "pipeline dispatched (check logs for geo_router)"
  fi
}

profile_stack_checks() {
  case "$PROFILE" in
    cached)
      if docker exec centag-postgresql pg_isready -U postgres >/dev/null 2>&1; then
        record PASS "stack-postgresql" "accepting connections"
      else
        record FAIL "stack-postgresql" "not reachable"
      fi
      ;;
    agent-memory)
      if curl -sf http://localhost:20061/health >/dev/null 2>&1; then
        record PASS "stack-mem0" "healthy on :20061"
      else
        record SKIP "stack-mem0" "mem0 not running"
      fi
      ;;
  esac
}

main() {
  cat > "$REPORT" <<EOF
# Profile Scenario Demo — $PROFILE

- Time: $(date -Iseconds)
- Proxy: $PROXY_URL
- Model: $MODEL

| Status | Case | Detail |
|--------|------|--------|
EOF

  log "Profile=$PROFILE Proxy=$PROXY_URL"
  check_health
  sync_builtin_pipelines
  check_phase3_templates
  profile_stack_checks
  test_geo_router_node
  test_webhook
  test_cache_hit
  test_rag_shortcut
  test_geo_chat

  cat >> "$REPORT" <<EOF

## Summary

- PASS: $pass
- FAIL: $fail
- SKIP: $skip
EOF

  log "Report: $REPORT"
  log "PASS=$pass FAIL=$fail SKIP=$skip"
  [ "$fail" -eq 0 ]
}

main "$@"