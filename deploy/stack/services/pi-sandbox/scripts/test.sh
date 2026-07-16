#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# 测试计数
TESTS_TOTAL=0
TESTS_PASSED=0
TESTS_FAILED=0

# 测试函数
run_test() {
    local test_name="$1"
    local test_cmd="$2"
    
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    
    echo -n "Running test: $test_name... "
    
    if eval "$test_cmd" > /dev/null 2>&1; then
        echo -e "${GREEN}PASSED${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}FAILED${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# 测试 1: 检查 Pi 安装
test_pi_installed() {
    pi --version
}

# 测试 2: 检查 Docker
test_docker_installed() {
    docker --version
}

# 测试 3: 检查 Docker Compose
test_docker_compose_installed() {
    docker-compose --version || docker compose version
}

# 测试 4: 检查 Go
test_go_installed() {
    go version
}

# 测试 5: 检查 .env 文件
test_env_file() {
    [ -f "$PROJECT_DIR/.env" ] || [ -f "$PROJECT_DIR/.env.example" ]
}

# 测试 6: 构建 Docker 镜像
test_build_docker() {
    cd "$PROJECT_DIR/docker"
    docker build -f Dockerfile.pi -t pi-sandbox:test . > /dev/null 2>&1
}

# 测试 7: 启动容器
test_start_container() {
    docker run -d --name pi-test \
        -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-test}" \
        pi-sandbox:test > /dev/null 2>&1
    
    # 等待容器启动
    sleep 3
    
    # 检查容器是否运行
    docker ps | grep -q pi-test
}

# 测试 8: 停止并清理容器
test_cleanup_container() {
    docker stop pi-test > /dev/null 2>&1 || true
    docker rm pi-test > /dev/null 2>&1 || true
}

# 测试 9: RPC 连接测试
test_rpc_connection() {
    # 启动临时容器
    docker run -d --name pi-rpc-test \
        -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-test}" \
        pi-sandbox:test > /dev/null 2>&1
    
    sleep 2
    
    # 发送测试请求
    echo '{"type": "prompt", "message": "Hello"}' | \
        docker exec -i pi-rpc-test pi --mode rpc --no-session 2>&1 | head -1 | grep -q "type"
    
    local result=$?
    
    # 清理
    docker stop pi-rpc-test > /dev/null 2>&1 || true
    docker rm pi-rpc-test > /dev/null 2>&1 || true
    
    return $result
}

# 测试 10: Go 客户端编译
test_go_client_build() {
    cd "$PROJECT_DIR/go-client"
    go build -o /tmp/pi-cli ./cmd/cli > /dev/null 2>&1
    rm -f /tmp/pi-cli
}

# 测试 11: Go 服务器编译
test_go_server_build() {
    cd "$PROJECT_DIR/go-client"
    go build -o /tmp/pi-server ./cmd/server > /dev/null 2>&1
    rm -f /tmp/pi-server
}

# 测试 12: 安全策略测试
test_security_policy() {
    cd "$PROJECT_DIR/go-client"
    go test -v ./pkg/security/... > /dev/null 2>&1
}

# 主测试流程
main() {
    log_info "Starting Pi Sandbox Integration Tests"
    echo ""
    
    # 基础环境检查
    log_info "Phase 1: Environment Checks"
    run_test "Pi installed" test_pi_installed
    run_test "Docker installed" test_docker_installed
    run_test "Docker Compose installed" test_docker_compose_installed
    run_test "Go installed" test_go_installed
    run_test "Env file exists" test_env_file
    
    echo ""
    
    # Docker 测试
    log_info "Phase 2: Docker Tests"
    run_test "Build Docker image" test_build_docker
    run_test "Start container" test_start_container
    
    if [ $TESTS_FAILED -eq 0 ]; then
        run_test "RPC connection" test_rpc_connection
    else
        log_warn "Skipping RPC test due to previous failures"
        TESTS_TOTAL=$((TESTS_TOTAL + 1))
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    
    run_test "Cleanup container" test_cleanup_container
    
    echo ""
    
    # Go 客户端测试
    log_info "Phase 3: Go Client Tests"
    run_test "Build Go CLI" test_go_client_build
    run_test "Build Go Server" test_go_server_build
    run_test "Security policy tests" test_security_policy
    
    echo ""
    
    # 测试结果汇总
    log_info "Test Results Summary"
    echo "===================="
    echo "Total tests:  $TESTS_TOTAL"
    echo -e "Passed:       ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Failed:       ${RED}$TESTS_FAILED${NC}"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        log_info "All tests passed!"
        exit 0
    else
        log_error "Some tests failed"
        exit 1
    fi
}

# 运行测试
main "$@"
