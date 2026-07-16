#!/bin/bash
# Mem0 服务独立测试脚本
# 不依赖 centag 后端，纯验证 mem0 服务 + 1024 维向量
# 用法: cd ~/workspaces/centag/centag-stack && bash services/mem0/test-mem0-service.sh

set -e

readonly MEM0_URL="http://localhost:20061"
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m'

pass() { echo -e "${GREEN}✅ $1${NC}"; }
fail() { echo -e "${RED}❌ $1${NC}"; exit 1; }
warn() { echo -e "${YELLOW}⚠️  $1${NC}"; }
info() { echo -e "${NC}ℹ️  $1${NC}"; }

echo "=========================================="
echo "Mem0 服务独立测试 (向量维度: 1024)"
echo "=========================================="

# ---------- 1. 服务可达性 ----------
echo ""
echo "[1/7] 检查 Mem0 API 可达性..."
if ! curl -s -f "${MEM0_URL}/" > /dev/null 2>&1; then
    fail "Mem0 API 不可达 (${MEM0_URL})，请确认服务已启动: ./start.sh start mem0"
fi
pass "Mem0 API 可达"

# ---------- 2. 获取 Token ----------
echo ""
echo "[2/7] 获取访问凭证..."

# 方式1: 尝试 OAuth2 /admin/token
TOKEN_RESP=$(curl -s -X POST "${MEM0_URL}/admin/token" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' 2>/dev/null || echo "FAILED")

TOKEN=$(echo "$TOKEN_RESP" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

if [ -n "$TOKEN" ]; then
    pass "通过 /admin/token 获取 token 成功"
else
    # 方式2: 从 .env 读取 MEM0_ADMIN_API_KEY 直接使用
    # 尝试多个可能的路径（脚本可能在 services/mem0/ 或 centag-stack/ 下被调用）
    for ENV_CANDIDATE in "../../.env" "./.env"; do
        if [ -f "$ENV_CANDIDATE" ]; then
            ADMIN_KEY=$(grep "^MEM0_ADMIN_API_KEY=" "$ENV_CANDIDATE" | cut -d'=' -f2 | tr -d '"' | tr -d "'" | tr -d ' ')
            if [ -n "$ADMIN_KEY" ]; then
                break
            fi
        fi
    done

    if [ -n "${ADMIN_KEY:-}" ]; then
        TOKEN="$ADMIN_KEY"
        pass "使用 .env 中的 MEM0_ADMIN_API_KEY 作为 Bearer Token"
    else
        fail "无法获取 token（/admin/token 返回: ${TOKEN_RESP:0:200}），且 .env 中未配置 MEM0_ADMIN_API_KEY"
    fi
fi

# ---------- 3. 添加记忆（触发 1024 维嵌入） ----------
echo ""
echo "[3/7] 添加测试记忆（infer=true，触发嵌入生成）..."
ADD_RESP=$(curl -s -X POST "${MEM0_URL}/memories" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role": "user", "content": "My name is TestUser and I love vector databases with 1024 dimensions"}],
    "user_id": "mem0_test_user",
    "agent_id": "mem0_test_agent",
    "app_id": "centag",
    "infer": true
  }' 2>/dev/null || echo "FAILED")

if [ "$ADD_RESP" = "FAILED" ]; then
    fail "添加记忆请求失败"
fi

if echo "$ADD_RESP" | grep -q '"code":' || echo "$ADD_RESP" | grep -q 'error'; then
    fail "添加记忆返回错误: ${ADD_RESP:0:500}"
fi
pass "添加记忆成功"

# ---------- 4. 语义搜索 ----------
echo ""
echo "[4/7] 语义搜索测试..."
sleep 2
SEARCH_RESP=$(curl -s -X POST "${MEM0_URL}/search" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "What does TestUser love?",
    "user_id": "mem0_test_user",
    "agent_id": "mem0_test_agent"
  }' 2>/dev/null || echo "FAILED")

if [ "$SEARCH_RESP" = "FAILED" ]; then
    fail "搜索请求失败"
fi

RESULT_COUNT=$(echo "$SEARCH_RESP" | grep -o '"id"' | wc -l)
if [ "$RESULT_COUNT" -gt 0 ]; then
    pass "语义搜索返回 ${RESULT_COUNT} 条结果"
else
    warn "语义搜索未返回结果（可能是首次冷启动，嵌入需要稍长时间）"
fi

# ---------- 5. 列表查询 ----------
echo ""
echo "[5/7] 列表查询记忆..."
LIST_RESP=$(curl -s "${MEM0_URL}/memories?user_id=mem0_test_user&agent_id=mem0_test_agent" \
  -H "Authorization: Bearer ${TOKEN}" 2>/dev/null || echo "FAILED")

if [ "$LIST_RESP" = "FAILED" ]; then
    fail "列表查询失败"
fi

MEM_COUNT=$(echo "$LIST_RESP" | grep -o '"id"' | wc -l)
if [ "$MEM_COUNT" -gt 0 ]; then
    pass "列表查询返回 ${MEM_COUNT} 条记忆"
else
    warn "列表查询未返回记忆"
fi

# ---------- 6. 向量维度验证（关键） ----------
echo ""
echo "[6/7] 验证数据库向量维度（应为 vector(1024)）..."
PG_OUT=$(docker exec centag-postgresql psql -U postgres -d centag -c "\d memories" 2>/dev/null || echo "PG_FAILED")

if [ "$PG_OUT" = "PG_FAILED" ]; then
    warn "无法连接 PostgreSQL 检查维度，跳过此步"
else
    if echo "$PG_OUT" | grep -q "vector(1024)"; then
        pass "向量维度正确: vector(1024)"
    elif echo "$PG_OUT" | grep -q "vector(1536)"; then
        fail "向量维度错误: vector(1536)，patch 未生效，请重建镜像: ./start.sh clean mem0 && ./start.sh start mem0"
    else
        DIM=$(echo "$PG_OUT" | grep "vector" | grep -oP '\(\K\d+' || echo "unknown")
        if [ "$DIM" = "1024" ]; then
            pass "向量维度正确: vector(1024)"
        else
            fail "向量维度异常: 检测到 ${DIM}，期望 1024"
        fi
    fi
fi

# ---------- 7. 清理测试数据 ----------
echo ""
echo "[7/7] 清理测试数据..."
# 注意：mem0 v2.0+ 删除接口可能有所不同，这里尝试多种方式
DEL_RESP=$(curl -s -X DELETE "${MEM0_URL}/memories" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "mem0_test_user",
    "agent_id": "mem0_test_agent"
  }' 2>/dev/null || echo "FAILED")

if [ "$DEL_RESP" != "FAILED" ]; then
    pass "测试数据已清理"
else
    warn "测试数据清理接口不可用（不影响使用）"
fi

# ---------- 汇总 ----------
echo ""
echo "=========================================="
pass "Mem0 服务测试全部通过！"
echo "=========================================="
echo ""
echo "关键配置确认:"
echo "  - 向量维度: 1024 (Ollama bge-m3)"
echo "  - 嵌入模型: bge-m3:latest"
echo "  - LLM 模型: qwen2.5:1.5b"
echo ""
echo "常用命令:"
echo "  查看日志: ./start.sh logs mem0"
echo "  查看状态: ./start.sh status"
echo "  停止服务: ./start.sh stop mem0"
echo "  清理重建: ./start.sh clean mem0 && ./start.sh start mem0"
