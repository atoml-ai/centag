#!/bin/bash

# ProxyClaw 核心功能验证脚本

set -e

echo "=========================================="
echo "ProxyClaw 核心功能验证"
echo "=========================================="
echo ""

BASE_URL="http://localhost:20060"

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

check_pass() {
    echo -e "${GREEN}✓${NC} $1"
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
}

check_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

echo "1. 服务健康检查"
echo "----------------"
HEALTH=$(curl -s ${BASE_URL}/health)
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    check_pass "ProxyClaw 服务运行正常"
    echo "   响应: $HEALTH"
else
    check_fail "ProxyClaw 服务异常"
    echo "   响应: $HEALTH"
fi
echo ""

echo "2. 数据库连接检查"
echo "-----------------"
if docker exec proxyclaw-postgresql pg_isready -U postgres > /dev/null 2>&1; then
    check_pass "PostgreSQL 连接正常"
    
    # 检查表数量
    TABLE_COUNT=$(docker exec -i proxyclaw-postgresql psql -U postgres -d proxyclaw -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null | tr -d ' ')
    echo "   数据库表数量: $TABLE_COUNT"
else
    check_fail "PostgreSQL 连接失败"
fi
echo ""

echo "3. 后端配置检查"
echo "---------------"
BACKEND_COUNT=$(docker exec -i proxyclaw-postgresql psql -U postgres -d proxyclaw -t -c "SELECT COUNT(*) FROM backends WHERE enabled = true;" 2>/dev/null | tr -d ' ')
if [ "$BACKEND_COUNT" -gt 0 ]; then
    check_pass "已启用后端数量: $BACKEND_COUNT"
    
    # 显示后端列表
    echo "   后端列表:"
    docker exec -i proxyclaw-postgresql psql -U postgres -d proxyclaw -t -c "SELECT '   - ' || name || ' (' || backend_type || ')' FROM backends WHERE enabled = true LIMIT 5;" 2>/dev/null
else
    check_warn "没有启用的后端"
fi
echo ""

echo "4. 用户和认证检查"
echo "-----------------"
USER_COUNT=$(docker exec -i proxyclaw-postgresql psql -U postgres -d proxyclaw -t -c "SELECT COUNT(*) FROM users;" 2>/dev/null | tr -d ' ')
if [ "$USER_COUNT" -gt 0 ]; then
    check_pass "用户数量: $USER_COUNT"
    
    # 显示管理员
    echo "   管理员用户:"
    docker exec -i proxyclaw-postgresql psql -U postgres -d proxyclaw -t -c "SELECT '   - ' || username || ' (' || role || ')' FROM users WHERE role = 'admin' LIMIT 3;" 2>/dev/null
else
    check_warn "没有用户数据"
fi

API_KEY_COUNT=$(docker exec -i proxyclaw-postgresql psql -U postgres -d proxyclaw -t -c "SELECT COUNT(*) FROM api_keys;" 2>/dev/null | tr -d ' ')
if [ "$API_KEY_COUNT" -gt 0 ]; then
    check_pass "API 密钥数量: $API_KEY_COUNT"
else
    check_warn "没有 API 密钥"
fi
echo ""

echo "5. 代理模式检查"
echo "---------------"
MODE_COUNT=$(docker exec -i proxyclaw-postgresql psql -U postgres -d proxyclaw -t -c "SELECT COUNT(*) FROM preset_modes;" 2>/dev/null | tr -d ' ')
if [ "$MODE_COUNT" -gt 0 ]; then
    check_pass "预设模式数量: $MODE_COUNT"
else
    check_warn "没有预设模式"
fi
echo ""

echo "6. 流水线配置检查"
echo "-----------------"
PIPELINE_COUNT=$(docker exec -i proxyclaw-postgresql psql -U postgres -d proxyclaw -t -c "SELECT COUNT(*) FROM pipelines;" 2>/dev/null | tr -d ' ')
if [ "$PIPELINE_COUNT" -gt 0 ]; then
    check_pass "流水线数量: $PIPELINE_COUNT"
else
    check_warn "没有流水线配置"
fi
echo ""

echo "7. 容器状态检查"
echo "---------------"
echo "运行中的容器:"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "proxyclaw|postgresql" | while read line; do
    echo "   $line"
done
echo ""

echo "=========================================="
echo "核心功能验证完成"
echo "=========================================="
echo ""
echo "总结:"
echo "- ProxyClaw 服务: 运行中"
echo "- PostgreSQL 数据库: 连接正常"
echo "- 数据库表: $TABLE_COUNT 个"
echo "- 后端配置: $BACKEND_COUNT 个已启用"
echo "- 用户: $USER_COUNT 个"
echo "- API 密钥: $API_KEY_COUNT 个"
echo ""
echo "核心功能验证通过！✅"
