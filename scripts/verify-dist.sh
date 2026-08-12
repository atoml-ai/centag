#!/usr/bin/env bash
# Centag dist verification script — probes a running instance via curl.
# Usage:
#   bash scripts/verify-dist.sh [url] [username] [password] [model] [edition]
#   bash scripts/verify-dist.sh http://localhost:20060 admin centag123
#   bash scripts/verify-dist.sh http://localhost:20060 admin centag123 gpt-4o-mini minimal

BASE="${1:-http://localhost:20060}"
USER="${2:-admin}"
PASS="${3:-centag123}"
CLI_MODEL="${4:-}"    # 可选 — 手动指定模型名
EDITION="${5:-}"      # 可选 — 手动指定版本 (minimal/personal/team)

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[1;33m'; cyan='\033[0;36m'; nc='\033[0m'
ok() { echo -e "  ${green}✓${nc} $1"; }
fail() { echo -e "  ${red}✗${nc} $1"; }
info() { echo -e "  ${cyan}→${nc} $1"; }
warn() { echo -e "  ${yellow}⚠${nc} $1"; }
header() { echo -e "\n${cyan}━━━ $1 ━━━${nc}"; }

die() { echo -e "${red}$1${nc}" >&2; exit 1; }

# ── 检测版本 ────────────────────────────────────────────────────────────────
detect_edition() {
  if [ -n "$EDITION" ]; then
    echo "$EDITION"
    return
  fi
  
  # 尝试从 /health 端点检测
  local health_resp
  health_resp=$(curl -sf "$BASE/health" 2>/dev/null)
  if [ -n "$health_resp" ]; then
    local detected
    detected=$(echo "$health_resp" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    print(d.get('edition', 'team'))
except:
    print('team')
" 2>/dev/null || echo "team")
    echo "$detected"
    return
  fi
  
  echo "team"
}

# ── 1. Liveness ──────────────────────────────────────────────────────────────
header "1/6  Liveness / Health"

if data=$(curl -sf "$BASE/ping" 2>/dev/null); then
  ok "GET /ping → $data"
else
  fail "GET /ping — host unreachable at $BASE"
  die "Is the service running?  Try: curl -v $BASE/ping"
fi

if data=$(curl -sf "$BASE/health" 2>/dev/null); then
  echo "$data" | python3 -m json.tool 2>/dev/null || echo "$data"
  ok "GET /health"
else
  fail "GET /health — no response"
fi

# 检测版本
DETECTED_EDITION=$(detect_edition)
info "检测到版本: $DETECTED_EDITION"

# ── 2. Readiness ──────────────────────────────────────────────────────────────
header "2/6  Readiness (DB check)"

if [ "$DETECTED_EDITION" = "minimal" ]; then
  warn "Minimal 版无需数据库就绪检查（文件配置模式）"
  ok "跳过 — minimal 版使用文件配置"
else
  if data=$(curl -sf "$BASE/health/ready" 2>/dev/null); then
    echo "$data" | python3 -m json.tool 2>/dev/null || echo "$data"
    ok "GET /health/ready — DB is reachable"
  else
    fail "GET /health/ready — database might be down"
  fi
fi

# ── 3. Status ─────────────────────────────────────────────────────────────────
header "3/6  Service Status (no auth)"

if [ "$DETECTED_EDITION" = "minimal" ]; then
  warn "Minimal 版无 /api/v1/status 端点"
  # 尝试从 /health 获取版本信息
  if data=$(curl -sf "$BASE/health" 2>/dev/null); then
    edition=$(echo "$data" | python3 -c "import sys,json; print(json.load(sys.stdin).get('edition','?'))" 2>/dev/null || echo "?")
    echo "  edition:    $edition"
    echo "  version:    dev"
    echo "  mode:       file-based (no database)"
    ok "GET /health — minimal 版信息"
  else
    fail "GET /health"
  fi
else
  if data=$(curl -sf "$BASE/api/v1/status" 2>/dev/null); then
    echo "$data" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(f'  edition:    {d.get(\"edition\",\"?\")}')
print(f'  version:    {d.get(\"version\",\"?\")}')
print(f'  uptime:     {d.get(\"uptime\",\"?\")}')
print(f'  start_time: {d.get(\"start_time\",\"?\")}')
" 2>/dev/null || echo "$data"
    ok "GET /api/v1/status"
  else
    fail "GET /api/v1/status"
  fi
fi

# ── 4. Login → JWT ────────────────────────────────────────────────────────────
header "4/6  Authentication (POST /api/auth/login)"

JWT=""
if [ "$DETECTED_EDITION" = "minimal" ]; then
  warn "Minimal 版无需认证（无登录接口）"
  info "Minimal 版所有 API 无需认证即可访问"
else
  login_json=$(printf '{"username":"%s","password":"%s"}' "$USER" "$PASS")
  login_resp=$(curl -sf -X POST "$BASE/api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "$login_json" 2>/dev/null) || {
    fail "POST /api/auth/login — wrong credentials or auth failure"
    warn "Default setup: no preset password (use first-run setup); preset via env LLM_PROXY_ADMIN_PASSWORD"
    JWT=""
  }

  if [ -n "${login_resp:-}" ]; then
    JWT=$(echo "$login_resp" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(d.get('data',{}).get('access_token',''))
" 2>/dev/null || echo "")
    if [ -n "$JWT" ]; then
      user_info=$(echo "$login_resp" | python3 -c "
import sys, json
d = json.load(sys.stdin)['data']['user']
print(f'  user:       {d[\"username\"]} (role={d[\"role\"]}, id={d[\"id\"]})')
" 2>/dev/null || echo "")
      info "$user_info"
      ok "JWT acquired: ${JWT:0:20}…${JWT: -8}"
    else
      fail "JWT extraction failed"
      JWT=""
    fi
  fi
fi

AUTH_HEADER="Authorization: Bearer $JWT"
api() { curl -sf "$BASE$1" -H "$AUTH_HEADER" 2>/dev/null || echo ""; }

# ── 5. Backends & Pipelines ──────────────────────────────────────────────────
header "5/6  Configuration (authenticated)"

if [ "$DETECTED_EDITION" = "minimal" ]; then
  info "Minimal 版 API 无需认证，直接访问"
fi

# 5a. Backends
backends=$(api "/api/v1/backends")
if [ -n "$backends" ]; then
  echo "$backends" | python3 -c "
import sys, json
d = json.load(sys.stdin)
items = d if isinstance(d, list) else d.get('data', d.get('backends', []))
print(f'  backends ({len(items)}):')
for b in items:
    name = b.get('name', b.get('id', '?'))
    btype = b.get('type', b.get('backend_type', '?'))
    url = b.get('base_url', b.get('url', '?'))
    enabled = b.get('is_enabled', b.get('enabled', True))
    status = '${green}enabled${nc}' if enabled else '${red}disabled${nc}'
    print(f'    - {name}  ({btype})  {url}  [{status}]')
" 2>/dev/null || echo "$backends" | python3 -m json.tool 2>/dev/null || echo "$backends"
  ok "GET /api/v1/backends"
else
  fail "GET /api/v1/backends"
fi

# 5b. Pipelines
pipelines=$(api "/api/v1/pipelines")
if [ -n "$pipelines" ]; then
  echo "$pipelines" | python3 -c "
import sys, json
d = json.load(sys.stdin)
items = d if isinstance(d, list) else d.get('data', d.get('pipelines', []))
print(f'  pipelines ({len(items)}):')
for p in items:
    pid = p.get('id', p.get('pipeline_id', '?'))
    name = p.get('name', pid)
    enabled = p.get('is_enabled', p.get('enabled', True))
    status = '${green}enabled${nc}' if enabled else '${red}disabled${nc}'
    print(f'    - {name}  (id={pid})  [{status}]')
" 2>/dev/null || python3 -m json.tool 2>/dev/null || echo "$pipelines"
  ok "GET /api/v1/pipelines"
else
  fail "GET /api/v1/pipelines"
fi

# 5c. Models (OpenAI-compatible) — 保存列表供第6步使用
MODEL_LIST=""
models=$(api "/v1/models")
if [ -n "$models" ]; then
  MODEL_LIST=$(echo "$models" | python3 -c "
import sys, json
d = json.load(sys.stdin)
items = d.get('data', [])
for m in items:
    print(m.get('id',''))
" 2>/dev/null || true)
  echo "$models" | python3 -c "
import sys, json
d = json.load(sys.stdin)
items = d.get('data', [])
print(f'  models ({len(items)}):')
for m in items:
    mid = m.get('id', '?')
    owned = m.get('owned_by', '')
    print(f'    - {mid}  (by {owned})' if owned else f'    - {mid}')
" 2>/dev/null || echo "$models" | python3 -m json.tool 2>/dev/null || echo "$models"
  ok "GET /v1/models"
else
  fail "GET /v1/models"
fi

# ── 6. LLM Proxy 实测 ─────────────────────────────────────────────────────────
header "6/6  LLM Proxy — curl 模拟客户端调用"

if [ "$DETECTED_EDITION" = "minimal" ]; then
  info "Minimal 版无需认证，直接测试代理功能"
fi

# 确定测试用模型：命令行参数 > /v1/models 列表 > 回退
FIRST_MODEL=$(echo "$MODEL_LIST" | head -1)
TEST_MODEL="${CLI_MODEL:-${FIRST_MODEL:-gpt-4o-mini}}"

echo -e "  ${cyan}·${nc} 请求模型: $TEST_MODEL"
if [ "$DETECTED_EDITION" = "minimal" ]; then
  echo -e "  ${cyan}·${nc} 认证方式: 无需认证 (Minimal 版)"
else
  echo -e "  ${cyan}·${nc} 认证方式: JWT (也可以用 API Key: Authorization: Bearer llmproxy_<key>)"
fi

TEST_PAYLOAD=$(printf '{"model":"%s","messages":[{"role":"user","content":"用一句话介绍你自己"}],"max_tokens":100}' "$TEST_MODEL")

echo ""
echo -e "  ${yellow}$ curl -s -X POST $BASE/v1/chat/completions \\${nc}"
echo -e "  ${yellow}  -H 'Content-Type: application/json' \\${nc}"
if [ "$DETECTED_EDITION" != "minimal" ]; then
  echo -e "  ${yellow}  -H 'Authorization: Bearer <token>' \\${nc}"
fi
echo -e "  ${yellow}  -d '$(echo "$TEST_PAYLOAD" | sed "s/'/\\'/g")'${nc}"

if [ "$DETECTED_EDITION" = "minimal" ]; then
  proxy_resp=$(curl -s -X POST "$BASE/v1/chat/completions" \
    -H 'Content-Type: application/json' \
    -d "$TEST_PAYLOAD" 2>&1) || true
else
  proxy_resp=$(curl -s -X POST "$BASE/v1/chat/completions" \
    -H 'Content-Type: application/json' \
    -H "$AUTH_HEADER" \
    -d "$TEST_PAYLOAD" 2>&1) || true
fi

echo ""
echo "$proxy_resp" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    if 'error' in d:
        print(json.dumps(d, indent=2, ensure_ascii=False))
        sys.exit(1)
    choices = d.get('choices', [])
    model = d.get('model', '?')
    usage = d.get('usage', {})
    print(f'  模型:     {model}')
    if choices:
        role = choices[0].get('message',{}).get('role','')
        content = choices[0].get('message',{}).get('content','').strip()
        finish = choices[0].get('finish_reason','')
        print(f'  角色:     {role}')
        print(f'  回复:     {content}')
        print(f'  结束原因: {finish}')
    if usage:
        print(f'  Token:    {usage.get(\"prompt_tokens\",0)} in → {usage.get(\"completion_tokens\",0)} out ({usage.get(\"total_tokens\",0)} total)')
except Exception:
    print(sys.stdin.read() or '(empty response)')
" 2>/dev/null
echo ""

if echo "$proxy_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if 'choices' in d else 1)" 2>/dev/null; then
  ok "POST /v1/chat/completions — 代理转发成功"
else
  fail "POST /v1/chat/completions — 代理返回错误"
  echo ""
  echo -e "  ${yellow}提示:${nc}"
  echo "    · 如果 /v1/models 为空，你的后端可能不支持模型列表查询，请手动指定:"
  echo "      bash scripts/verify-dist.sh $BASE $USER <password> <模型ID> $DETECTED_EDITION"
  echo "    · 通用 curl 测试:"
  echo "      curl -s -X POST $BASE/v1/chat/completions \\"
  echo "        -H 'Content-Type: application/json' \\"
  if [ "$DETECTED_EDITION" != "minimal" ]; then
    echo "        -H 'Authorization: Bearer <JWT 或 API Key>' \\"
  fi
  echo "        -d '{\"model\":\"<模型ID>\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}'"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
header "Done"
echo ""
echo -e "  ${cyan}实例:${nc} $BASE"
echo -e "  ${cyan}版本:${nc} $DETECTED_EDITION"
echo -e "  ${cyan}客户端配置地址:${nc} $BASE/v1/chat/completions"

if [ "$DETECTED_EDITION" = "minimal" ]; then
  echo -e "  ${cyan}客户端配置 API Key:${nc} 无需认证 (Minimal 版)"
  echo -e "  ${cyan}查看模型列表:${nc} curl -s $BASE/v1/models"
  echo -e "  ${cyan}配置管理:${nc} 浏览器打开 $BASE 使用 config-generator"
else
  echo -e "  ${cyan}客户端配置 API Key:${nc} JWT 或 llmproxy_ 开头的 API Key"
  echo -e "  ${cyan}查看模型列表:${nc} curl -sH 'Authorization: Bearer <token>' $BASE/v1/models"
fi
echo ""
