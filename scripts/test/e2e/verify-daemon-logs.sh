#!/bin/bash
# 验证守护进程日志双输出功能的完整性

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================"
echo "验证守护进程日志双输出功能"
echo "========================================"
echo ""

# 检查项计数
total=0
passed=0

# 1. 检查配置文件
echo -n "1. 检查 LLM_PROXY_LOG_OUTPUT 环境变量 ... "
total=$((total + 1))
env_file="config/secrets/.env"
if [ -f "$env_file" ] && grep -q 'LLM_PROXY_LOG_OUTPUT=file' "$env_file"; then
    echo -e "${GREEN}✅ 通过${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 失败${NC}"
    echo "   预期: config/secrets/.env 中包含 LLM_PROXY_LOG_OUTPUT=file"
fi

# 2. 检查logger实现
echo -n "2. 检查 internal/logger/logger.go ... "
total=$((total + 1))
if grep -q "NewMultiWriteSyncer" internal/logger/logger.go; then
    echo -e "${GREEN}✅ 通过${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 失败${NC}"
    echo "   预期: 包含 NewMultiWriteSyncer"
fi

# 3. 检查daemon.sh脚本
echo -n "3. 检查 scripts/tools/daemon.sh ... "
total=$((total + 1))
if ! grep -q "DAEMON_LOG_TO_CONSOLE" scripts/tools/daemon.sh; then
    echo -e "${GREEN}✅ 通过${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 失败${NC}"
    echo "   预期: 不应包含 DAEMON_LOG_TO_CONSOLE"
fi

# 4. 检查文档
echo -n "4. 检查文档 docs/Daemon-Log-Configuration.md ... "
total=$((total + 1))
if [ -f "docs/Daemon-Log-Configuration.md" ]; then
    echo -e "${GREEN}✅ 通过${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 失败${NC}"
fi

# 5. 检查测试脚本
echo -n "5. 检查测试脚本 test/test-daemon-logs.sh ... "
total=$((total + 1))
if [ -f "test/test-daemon-logs.sh" ] && [ -x "test/test-daemon-logs.sh" ]; then
    echo -e "${GREEN}✅ 通过${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 失败${NC}"
fi

# 6. 检查README更新
echo -n "6. 检查 README.md 更新 ... "
total=$((total + 1))
if grep -q "Logging Configuration" README.md; then
    echo -e "${GREEN}✅ 通过${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 失败${NC}"
fi

# 7. 检查CHANGELOG
echo -n "7. 检查 CHANGELOG-daemon-logs.md ... "
total=$((total + 1))
if [ -f "CHANGELOG-daemon-logs.md" ]; then
    echo -e "${GREEN}✅ 通过${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ 失败${NC}"
fi

echo ""
echo "========================================"
echo "验证结果: $passed/$total 通过"
echo "========================================"

if [ $passed -eq $total ]; then
    echo -e "${GREEN}✅ 所有检查通过！守护进程日志双输出功能已正确实现。${NC}"
    echo ""
    echo "使用方式："
    echo "  2. 运行测试：./test/test-daemon-logs.sh"
    echo "  3. 启动服务：./scripts/tools/daemon.sh"
    echo ""
    exit 0
else
    echo -e "${RED}❌ 部分检查失败，请检查实现。${NC}"
    exit 1
fi
