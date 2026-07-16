#!/usr/bin/env bash
# stack PostgreSQL + 本地 centag：#ch 精确缓存命中演示（无需构建 cached Docker 镜像）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PROXY_URL="${PROXY_URL:-http://localhost:20060}"
MODEL="${DEMO_MODEL:-glm-4-flash}"

if [ -f "$ROOT/config/secrets/.env" ]; then
  # shellcheck disable=SC1091
  source "$ROOT/config/secrets/.env"
fi
if [ -f "$ROOT/deploy/stack/.env" ]; then
  # shellcheck disable=SC1091
  source "$ROOT/deploy/stack/.env"
fi
ADMIN_KEY="${LLM_PROXY_ADMIN_API_KEY:-test-key}"
BIGMODEL_KEY="${LLM_PROXY_BIGMODEL_API_KEY:-}"

auth() { printf 'Authorization: Bearer %s' "$ADMIN_KEY"; }

ensure_pg_storage() {
  local has_pg
  has_pg=$(curl -sf -H "$(auth)" "$PROXY_URL/api/v1/storage" | python3 -c "
import json,sys
d=json.load(sys.stdin)
items=d.get('storages') or d.get('data') or []
print('yes' if any(s.get('name')=='pg' and s.get('enabled') for s in items) else 'no')
" 2>/dev/null || echo "no")

  if [ "$has_pg" = "yes" ]; then
    echo "[ch-demo] pg storage already enabled"
    curl -sf -X POST "$PROXY_URL/api/v1/storage/connect" \
      -H "$(auth)" -H "Content-Type: application/json" -d '{"name":"pg"}' >/dev/null || true
    return 0
  fi

  echo "[ch-demo] registering PostgreSQL storage 'pg' (${PG_HOST:-localhost}:${PG_PORT:-5432})..."
  local cfg
  cfg=$(python3 -c "
import json,os
print(json.dumps({
  'name':'pg',
  'type':'postgresql',
  'enabled':True,
  'description':'stack PG cache (exact + semantic)',
  'config': {
    'host': os.environ.get('PG_HOST','localhost'),
    'port': int(os.environ.get('PG_PORT','5432')),
    'user': os.environ.get('PG_USER','postgres'),
    'password': os.environ.get('PG_PASSWORD',''),
    'database': os.environ.get('PG_DATABASE','centag'),
    'ssl_mode': 'disable',
    'max_conn_lifetime': 3600,
    'max_conn_idle_time': 600,
    'max_conns': 10,
    'min_conns': 1,
    'kv_table': 'kv_cache',
    'vector_table': 'vector_cache',
    'vector_dimension': 1024,
    'index_type': 'hnsw'
  }
}))
")

  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$PROXY_URL/api/v1/storage/add" \
    -H "$(auth)" -H "Content-Type: application/json" -d "$cfg")
  if [ "$code" != "200" ]; then
    echo "[ch-demo] storage/add HTTP $code (may already exist)" >&2
  fi
  curl -sf -X POST "$PROXY_URL/api/v1/storage/connect" \
    -H "$(auth)" -H "Content-Type: application/json" -d '{"name":"pg"}' >/dev/null
  curl -sf -X POST "$PROXY_URL/api/v1/storage/set-default" \
    -H "$(auth)" -H "Content-Type: application/json" -d '{"name":"pg"}' >/dev/null
  echo "[ch-demo] pg storage connected"
}

enable_bigmodel() {
  [ -n "$BIGMODEL_KEY" ] || return 0
  curl -sf -X PUT "$PROXY_URL/api/v1/backends/bigmodel" \
    -H "$(auth)" -H "Content-Type: application/json" \
    -d "{\"id\":\"bigmodel\",\"enabled\":true,\"type\":\"openai\",\"base_url\":\"https://open.bigmodel.cn/api/paas/v4\",\"api_key\":\"$BIGMODEL_KEY\",\"weight\":100}" >/dev/null
}

chat_ch() {
  local content="$1" outfile="$2" hdrfile="$3"
  curl -s --max-time 120 -D "$hdrfile" -o "$outfile" -X POST "$PROXY_URL/v1/chat/completions" \
    -H "$(auth)" -H "Content-Type: application/json" \
    -H "X-Proxy-Mode: cache-hit" \
    -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"$content\"}],\"max_tokens\":24}"
}

wait_health() {
  for i in $(seq 1 60); do
    if curl -sf "$PROXY_URL/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "[ch-demo] proxy not healthy at $PROXY_URL" >&2
  return 1
}

main() {
  export PG_HOST="${PG_HOST:-localhost}"
  export PG_PORT="${PG_PORT:-5432}"
  export PG_USER="${PG_USER:-postgres}"
  export PG_DATABASE="${PG_DATABASE:-centag}"
  export PG_PASSWORD="${PG_PASSWORD:-${POSTGRES_PASSWORD:-}}"
  if [ -z "$PG_PASSWORD" ]; then
    echo "[ch-demo] PG_PASSWORD not set (source deploy/stack/.env or config/secrets/.env)" >&2
    exit 1
  fi
  wait_health
  enable_bigmodel
  ensure_pg_storage

  local q="profile-ch-pg-$(date +%s)"
  local f1=/tmp/ch_demo_1.json f2=/tmp/ch_demo_2.json h1=/tmp/ch_demo_1.hdr h2=/tmp/ch_demo_2.hdr

  echo "[ch-demo] 1st request (miss → generate → write PG)..."
  chat_ch "#ch $q" "$f1" "$h1"
  python3 -c "import json; d=json.load(open('$f1')); print('  content:', (d.get('choices') or [{}])[0].get('message',{}).get('content','')[:50]); print('  error:', d.get('error'))"
  grep -iE '^(x-cache|x-cache-hit|x-pipeline-id):' "$h1" 2>/dev/null || true

  sleep 2
  echo "[ch-demo] 2nd request (expect cache hit)..."
  chat_ch "#ch $q" "$f2" "$h2"
  python3 -c "import json; d=json.load(open('$f2')); print('  content:', (d.get('choices') or [{}])[0].get('message',{}).get('content','')[:50]); print('  error:', d.get('error'))"
  grep -iE '^(x-cache|x-cache-hit|x-pipeline-id|x-pipeline-duration-ms):' "$h2" 2>/dev/null || true

  local cache_hit_hdr cache_hdr
  cache_hit_hdr=$(grep -i '^x-cache-hit:' "$h2" 2>/dev/null | tail -1 | awk '{print $2}' | tr -d '\r' || true)
  cache_hdr=$(grep -i '^x-cache:' "$h2" 2>/dev/null | tail -1 | awk '{print $2}' | tr -d '\r' || true)

  if echo "$cache_hit_hdr" | grep -qiE '^true$'; then
    echo "[ch-demo] X-Cache-Hit (2nd) = $cache_hit_hdr"
    echo "[ch-demo] ✅ PG exact cache hit demonstrated (pipeline cache-hit)"
    exit 0
  fi
  if echo "$cache_hdr" | grep -qiE 'HIT'; then
    echo "[ch-demo] X-Cache (2nd) = $cache_hdr"
    echo "[ch-demo] ✅ PG exact cache hit demonstrated (middleware)"
    exit 0
  fi
  echo "[ch-demo] ❌ expected cache hit (X-Cache-Hit=true or X-Cache=HIT-*), got X-Cache-Hit=${cache_hit_hdr:-MISS} X-Cache=${cache_hdr:-MISS}"
  if [ -f "$ROOT/bin/logs/centag.log" ]; then
    grep -iE 'CacheNode|cache_hit|cache hit' "$ROOT/bin/logs/centag.log" | tail -15 || true
  fi
  exit 1
}

main "$@"