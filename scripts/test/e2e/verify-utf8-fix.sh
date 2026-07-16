#!/bin/bash
# 验证缓存中文乱码修复

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================"
echo -e "验证缓存中文乱码修复"
echo -e "========================================${NC}"
echo ""

# 检查修复
echo -e "${YELLOW}检查代码修复...${NC}"
total=0
passed=0

# 1. 检查 handler.go
echo -n "1. 检查 internal/proxy/handler.go ... "
total=$((total + 1))
if grep -q "runes := \[\]rune(content)" internal/proxy/handler.go; then
    echo -e "${GREEN}✅ 已修复${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 未修复${NC}"
fi

# 2. 检查 middleware/proxy.go
echo -n "2. 检查 internal/middleware/proxy.go ... "
total=$((total + 1))
if grep -q "runes := \[\]rune(content)" internal/middleware/proxy.go; then
    echo -e "${GREEN}✅ 已修复${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 未修复${NC}"
fi

# 3. 检查文档
echo -n "3. 检查修复文档 ... "
total=$((total + 1))
if [ -f "docs/缓存流式响应中文乱码修复.md" ]; then
    echo -e "${GREEN}✅ 存在${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 不存在${NC}"
fi

echo ""
echo -e "${BLUE}========================================"
echo -e "检查结果: $passed/$total 通过"
echo -e "========================================${NC}"
echo ""

if [ $passed -eq $total ]; then
    echo -e "${GREEN}✅ 所有检查通过！${NC}"
    echo ""
    echo -e "${YELLOW}下一步：${NC}"
    echo "1. 重新编译服务："
    echo -e "   ${BLUE}make build${NC}"
    echo ""
    echo "2. 重启服务："
    echo -e "   ${BLUE}./scripts/tools/daemon.sh${NC}"
    echo ""
    echo "3. 测试修复："
    echo "   - 访问 http://localhost:20060"
    echo "   - 打开 AI对话 功能"
    echo "   - 输入中文问题（例如：什么是人工智能？）"
    echo "   - 首次提问：观察响应正常"
    echo "   - ${YELLOW}再次提问相同问题：应该命中缓存且中文正常显示（无乱码）${NC}"
    echo ""
    echo ""
    exit 0
else
    echo -e "${RED}❌ 部分检查失败${NC}"
    exit 1
fi
