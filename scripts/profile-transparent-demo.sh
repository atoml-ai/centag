#!/usr/bin/env bash
# 本地 centag：#t 透明代理 scripts/tools/stream 原样透传演示（mock 上游）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PROXY_URL="${PROXY_URL:-http://localhost:20060}"
MOCK_PORT="${MOCK_PORT:-29998}"

if [ -f "$ROOT/config/secrets/.env" ]; then
  # shellcheck disable=SC1091
  source "$ROOT/config/secrets/.env"
fi
ADMIN_KEY="${LLM_PROXY_ADMIN_API_KEY:-test-key}"

auth() { printf 'Authorization: Bearer %s' "$ADMIN_KEY"; }

wait_health() {
  for _ in $(seq 1 30); do
    curl -sf "$PROXY_URL/health" >/dev/null 2>&1 && return 0
    sleep 1
  done
  echo "[t-demo] proxy not healthy at $PROXY_URL" >&2
  return 1
}

start_mock_upstream() {
  local log=/tmp/t_demo_mock.log body=/tmp/t_demo_mock_body.json
  python3 - "$MOCK_PORT" "$body" "$log" <<'PY' &
import json, sys
from http.server import HTTPServer, BaseHTTPRequestHandler

port, body_path, log_path = int(sys.argv[1]), sys.argv[2], sys.argv[3]

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass
    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)
        with open(body_path, "wb") as f:
            f.write(raw)
        with open(log_path, "w", encoding="utf-8") as f:
            f.write(raw.decode("utf-8", errors="replace"))
        resp = json.dumps({"id": "mock-upstream", "object": "chat.completion", "choices": [{"message": {"content": "ok"}}]})
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(resp.encode())

HTTPServer(("127.0.0.1", port), Handler).serve_forever()
PY
  MOCK_PID=$!
  echo "$MOCK_PID" >/tmp/t_demo_mock.pid
  sleep 0.5
  if ! kill -0 "$MOCK_PID" 2>/dev/null; then
    echo "[t-demo] mock upstream failed to start on port $MOCK_PORT" >&2
    return 1
  fi
}

stop_mock_upstream() {
  if [ -f /tmp/t_demo_mock.pid ]; then
    kill "$(cat /tmp/t_demo_mock.pid)" 2>/dev/null || true
    rm -f /tmp/t_demo_mock.pid
  fi
}

main() {
  trap stop_mock_upstream EXIT
  wait_health
  rm -f /tmp/t_demo_mock_body.json /tmp/t_demo_mock.log
  start_mock_upstream
  sleep 0.5

  local payload outfile hdrfile target
  payload='{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"get_weather"}}],"tool_choice":"auto","stream":true}'
  outfile=/tmp/t_demo_resp.json
  hdrfile=/tmp/t_demo_resp.hdr
  target="http://127.0.0.1:${MOCK_PORT}/v1/chat/completions"

  echo "[t-demo] POST transparent-proxy → $target"
  curl -s --max-time 60 -D "$hdrfile" -o "$outfile" -X POST "$PROXY_URL/v1/chat/completions" \
    -H "$(auth)" -H "Content-Type: application/json" \
    -H "X-Proxy-Mode: transparent-proxy" \
    -H "X-Pipeline-ID: transparent-proxy" \
    -H "X-Target-URL: $target" \
    -d "$payload"

  if [ -s "$outfile" ]; then
    python3 -c "
import json,sys
raw=open('$outfile').read().strip()
if raw.startswith('data:'):
    print('  response: SSE stream (client body has stream=true)')
else:
    try:
        d=json.loads(raw)
        print('  upstream id:', d.get('id'))
        print('  error:', d.get('error'))
    except json.JSONDecodeError:
        print('  response preview:', raw[:120])
" || true
  else
    echo "  response: (empty)"
  fi

  if [ ! -f /tmp/t_demo_mock_body.json ]; then
    echo "[t-demo] ❌ mock upstream received no body" >&2
    exit 1
  fi

  python3 -c "
import json, sys
sent = open('/tmp/t_demo_mock_body.json','rb').read().decode()
want = json.loads('''$payload''')
got = json.loads(sent)
for k in ('tools', 'tool_choice', 'stream'):
    if k not in got:
        print(f'missing field: {k}', file=sys.stderr)
        sys.exit(1)
if got.get('stream') is not True:
    print('stream not true', file=sys.stderr)
    sys.exit(1)
if got['tools'][0]['function']['name'] != 'get_weather':
    print('tools altered', file=sys.stderr)
    sys.exit(1)
print('  mock body preserves scripts/tools/stream/tool_choice')
"

  grep -iE '^(x-pipeline-id|x-target-baseurl):' "$hdrfile" 2>/dev/null || true
  echo "[t-demo] ✅ #t transparent forward preserved request body"
}

main "$@"