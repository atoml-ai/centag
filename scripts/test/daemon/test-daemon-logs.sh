#!/bin/bash
# 测试守护进程日志输出
# 用法: ./test-daemon-logs.sh

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}守护进程日志输出测试${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查是否已编译
if [ ! -f "$PROJECT_ROOT/centag" ]; then
    echo -e "${YELLOW}未找到二进制文件，正在编译...${NC}"
    cd "$PROJECT_ROOT" || exit 1
    make build || {
        echo -e "${RED}❌ 编译失败${NC}"
        exit 1
    }
    echo -e "${GREEN}✅ 编译成功${NC}"
    echo ""
fi

# 清理旧日志
LOG_FILE="$PROJECT_ROOT/logs/centag.log"
if [ -f "$LOG_FILE" ]; then
    echo -e "${YELLOW}清理旧日志文件...${NC}"
    rm -f "$LOG_FILE"
fi

# 测试1: 非Debug模式启动，检查日志是否同时输出到控制台
echo -e "${BLUE}测试1: 非Debug模式启动守护进程${NC}"
echo -e "${YELLOW}预期: 日志应该同时输出到控制台和文件${NC}"
echo ""

# 启动守护进程
echo -e "${GREEN}启动守护进程（后台模式）...${NC}"
cd "$PROJECT_ROOT" || exit 1

# 使用超时运行守护进程，10秒后自动停止
timeout 10s ./scripts/tools/daemon.sh "$PROJECT_ROOT" 2>&1 | tee /tmp/daemon-console.log &
DAEMON_PID=$!

echo -e "${YELLOW}守护进程PID: $DAEMON_PID${NC}"
echo -e "${YELLOW}等待10秒观察日志输出...${NC}"
echo ""

# 等待守护进程启动
sleep 5

# 检查控制台输出
echo -e "${BLUE}检查控制台输出...${NC}"
if grep -q "Starting Proxyclaw Service" /tmp/daemon-console.log 2>/dev/null; then
    echo -e "${GREEN}✅ 控制台有日志输出${NC}"
else
    echo -e "${RED}❌ 控制台没有日志输出${NC}"
fi

# 检查文件输出
echo -e "${BLUE}检查文件输出...${NC}"
if [ -f "$LOG_FILE" ]; then
    if grep -q "Starting Proxyclaw Service" "$LOG_FILE" 2>/dev/null; then
        echo -e "${GREEN}✅ 日志文件有记录${NC}"
    else
        echo -e "${RED}❌ 日志文件没有记录${NC}"
    fi
else
    echo -e "${RED}❌ 日志文件不存在${NC}"
fi

# 等待超时自动停止
wait $DAEMON_PID 2>/dev/null || true

echo ""
echo -e "${BLUE}显示日志文件内容（前20行）:${NC}"
if [ -f "$LOG_FILE" ]; then
    head -20 "$LOG_FILE"
else
    echo -e "${YELLOW}日志文件不存在${NC}"
fi

echo ""
echo -e "${BLUE}显示控制台输出（前20行）:${NC}"
if [ -f /tmp/daemon-console.log ]; then
    head -20 /tmp/daemon-console.log
else
    echo -e "${YELLOW}控制台输出文件不存在${NC}"
fi

# 清理
echo ""
echo -e "${YELLOW}清理测试文件...${NC}"
rm -f /tmp/daemon-console.log

# 停止任何残留的进程
if [ -f "$PROJECT_ROOT/storage/centag.pid" ]; then
    PID=$(cat "$PROJECT_ROOT/storage/centag.pid")
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        echo -e "${YELLOW}停止服务进程 (PID: $PID)...${NC}"
        kill -TERM "$PID" 2>/dev/null || true
        sleep 1
    fi
    rm -f "$PROJECT_ROOT/storage/centag.pid"
fi

if [ -f "$PROJECT_ROOT/storage/centag.daemon.pid" ]; then
    rm -f "$PROJECT_ROOT/storage/centag.daemon.pid"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}测试完成${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}说明：${NC}"
echo -e "1. 如果日志同时出现在控制台和文件中，说明配置正确"
echo -e "2. 可以通过环境变量 LLM_PROXY_LOG_OUTPUT 来控制日志输出方式："
echo -e "   - LLM_PROXY_LOG_OUTPUT=file   - 同时输出到控制台和文件（推荐）"
echo -e "   - LLM_PROXY_LOG_OUTPUT=stdout - 仅输出到控制台"
echo ""
