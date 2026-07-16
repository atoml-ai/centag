#!/usr/bin/env bash
# stack PostgreSQL + 本地 centag：#rag L1 精确 / L2 语义缓存命中演示
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PROXY_URL="${PROXY_URL:-http://localhost:20060}"
MODEL="${DEMO_MODEL:-glm-4-flash}"
RUN_L2="${RUN_L2:-auto}"

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

sync_rag_pipeline() {
  echo "[rag-demo] syncing rag-mode template (pg + hybrid cache)..."
  local body code
  body=$(python3 -c "
import json, pathlib
try:
    import yaml
except ImportError:
    raise SystemExit('pyyaml required: pip install pyyaml')
p = pathlib.Path('$ROOT/config/initdata/pipeline-templates/18-rag-mode.yaml')
t = yaml.safe_load(p.read_text())
print(json.dumps({
  'id': t['id'],
  'name': t.get('name',''),
  'description': t.get('description',''),
  'shortcut_code': t.get('shortcut_code',''),
  'nodes': t['nodes'],
  'global_config': t.get('global_config') or {},
  'metadata': t.get('metadata') or {},
  'version': t.get('version','1.0'),
}))
")
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$PROXY_URL/api/v1/pipelines?overwrite=true" \
    -H "$(auth)" -H "Content-Type: application/json" -d "$body")
  if [ "$code" != "200" ]; then
    echo "[rag-demo] pipeline sync HTTP $code" >&2
    exit 1
  fi
}

ensure_pg_storage() {
  local has_pg
  has_pg=$(curl -sf -H "$(auth)" "$PROXY_URL/api/v1/storage" | python3 -c "
import json,sys
d=json.load(sys.stdin)
items=d.get('storages') or d.get('data') or []
print('yes' if any(s.get('name')=='pg' and s.get('enabled') for s in items) else 'no')
" 2>/dev/null || echo "no")
  if [ "$has_pg" = "yes" ]; then
    curl -sf -X POST "$PROXY_URL/api/v1/storage/connect" -H "$(auth)" -H "Content-Type: application/json" -d '{"name":"pg"}' >/dev/null || true
    return 0
  fi
  echo "[rag-demo] pg storage not enabled; run profile-ch-cache-demo.sh first" >&2
  exit 1
}

enable_bigmodel() {
  [ -n "$BIGMODEL_KEY" ] || return 0
  curl -sf -X PUT "$PROXY_URL/api/v1/backends/bigmodel" \
    -H "$(auth)" -H "Content-Type: application/json" \
    -d "{\"id\":\"bigmodel\",\"enabled\":true,\"type\":\"openai\",\"base_url\":\"https://open.bigmodel.cn/api/paas/v4\",\"api_key\":\"$BIGMODEL_KEY\",\"weight\":100}" >/dev/null
}

chat_rag() {
  local content="$1" outfile="$2" hdrfile="$3"
  curl -s --max-time 180 -D "$hdrfile" -o "$outfile" -X POST "$PROXY_URL/v1/chat/completions" \
    -H "$(auth)" -H "Content-Type: application/json" \
    -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"#rag $content\"}],\"max_tokens\":48}"
}

cache_hit_hdr() {
  grep -i '^x-cache-hit:' "$1" 2>/dev/null | tail -1 | awk '{print $2}' | tr -d '\r' || true
}

main() {
  curl -sf "$PROXY_URL/health" >/dev/null
  enable_bigmodel
  ensure_pg_storage
  sync_rag_pipeline

  local ts q1 q2
  ts=$(date +%s)
  q1="profile-rag-l1-${ts}: What is the capital of France?"
  q2="profile-rag-l2-${ts}: Please tell me the capital city of France."

  local f1=/tmp/rag_demo_1.json f2=/tmp/rag_demo_2.json h2=/tmp/rag_demo_2.hdr
  local f3=/tmp/rag_demo_3.json f4=/tmp/rag_demo_4.json h4=/tmp/rag_demo_4.hdr

  echo "[rag-demo] 1st #rag request (miss → retrieve → generate → write cache)..."
  chat_rag "$q1" "$f1" /tmp/rag_demo_1.hdr
  python3 -c "import json; d=json.load(open('$f1')); print('  content:', (d.get('choices') or [{}])[0].get('message',{}).get('content','')[:60]); print('  error:', d.get('error'))"

  sleep 2
  echo "[rag-demo] 2nd identical #rag request (expect L1 cache hit)..."
  chat_rag "$q1" "$f2" "$h2"
  python3 -c "import json; d=json.load(open('$f2')); print('  content:', (d.get('choices') or [{}])[0].get('message',{}).get('content','')[:60])"
  grep -iE '^(x-cache-hit|x-pipeline-id|x-pipeline-duration-ms):' "$h2" 2>/dev/null || true

  local hit
  hit=$(cache_hit_hdr "$h2")
  if echo "$hit" | grep -qiE '^true$'; then
    echo "[rag-demo] ✅ L1 exact cache hit on #rag"
  else
    echo "[rag-demo] ⚠️  L1 miss (X-Cache-Hit=${hit:-MISS})"
  fi

  if [ "$RUN_L2" = "0" ] || [ "$RUN_L2" = "false" ]; then
    echo "[rag-demo] RUN_L2=0 — skip L2 paraphrase test"
    exit 0
  fi

  echo "[rag-demo] 3rd #rag paraphrase (seed L2 corpus)..."
  chat_rag "$q2" "$f3" /tmp/rag_demo_3.hdr
  python3 -c "import json; d=json.load(open('$f3')); print('  content:', (d.get('choices') or [{}])[0].get('message',{}).get('content','')[:60]); print('  error:', d.get('error'))"

  sleep 2
  echo "[rag-demo] 4th paraphrase #rag request (expect L1 or L2 cache hit)..."
  chat_rag "$q2" "$f4" "$h4"
  python3 -c "import json; d=json.load(open('$f4')); print('  content:', (d.get('choices') or [{}])[0].get('message',{}).get('content','')[:60])"
  grep -iE '^(x-cache-hit|x-pipeline-id|x-pipeline-duration-ms):' "$h4" 2>/dev/null || true

  hit=$(cache_hit_hdr "$h4")
  if echo "$hit" | grep -qiE '^true$'; then
    echo "[rag-demo] ✅ L2/L1 cache hit on paraphrased #rag"
    exit 0
  fi

  echo "[rag-demo] ⚠️  L2 paraphrase miss (X-Cache-Hit=${hit:-MISS}); needs embedding + pgvector semantic index"
  exit 0
}

main "$@"