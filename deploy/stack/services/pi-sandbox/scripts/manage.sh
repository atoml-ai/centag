#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
DOCKER_DIR="$PROJECT_DIR/docker"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 .env：优先 stack 根目录，其次服务目录
check_env() {
    local stack_env="${PROJECT_DIR}/../../.env"
    if [ -f "$stack_env" ]; then
        return 0
    fi
    if [ ! -f "$PROJECT_DIR/.env" ]; then
        if [ -f "$PROJECT_DIR/.env.example" ]; then
            log_warn ".env 不存在，从 .env.example 创建（建议改用 stack 根目录 .env）"
            cp "$PROJECT_DIR/.env.example" "$PROJECT_DIR/.env"
        else
            log_warn "未找到 .env，请配置 ${stack_env} 或 ${PROJECT_DIR}/.env"
            exit 1
        fi
        log_warn "请编辑 .env 配置 API 密钥后重新运行"
        exit 1
    fi
}

# 检查 Docker 和 Docker Compose
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_error "Docker Compose 未安装"
        exit 1
    fi
}

# 创建必要的目录
create_dirs() {
    log_info "创建必要的目录..."
    mkdir -p "$DOCKER_DIR/workspace"
    mkdir -p "$DOCKER_DIR/extensions"
    mkdir -p "$DOCKER_DIR/skills"
}

# 构建镜像
build() {
    log_info "构建 Pi 沙盒镜像..."
    cd "$DOCKER_DIR"
    
    if docker compose version &> /dev/null; then
        docker compose build
    else
        docker-compose build
    fi
    
    log_info "构建完成"
}

# 启动服务
start() {
    log_info "启动 Pi 沙盒服务..."
    cd "$DOCKER_DIR"
    
    if docker compose version &> /dev/null; then
        docker compose up -d
    else
        docker-compose up -d
    fi
    
    log_info "服务已启动"
    log_info "Pi Agent: pi-sandbox 容器"
    log_info "Go Client: http://localhost:${GO_CLIENT_PORT:-8080}"
    
    # 等待健康检查
    log_info "等待服务就绪..."
    sleep 5
    
    if docker ps | grep -q pi-sandbox; then
        log_info "Pi Agent 运行正常"
    else
        log_error "Pi Agent 启动失败"
        docker logs pi-sandbox
        exit 1
    fi
}

# 停止服务
stop() {
    log_info "停止 Pi 沙盒服务..."
    cd "$DOCKER_DIR"
    
    if docker compose version &> /dev/null; then
        docker compose down
    else
        docker-compose down
    fi
    
    log_info "服务已停止"
}

# 重启服务
restart() {
    stop
    start
}

# 查看日志
logs() {
    cd "$DOCKER_DIR"
    
    if docker compose version &> /dev/null; then
        docker compose logs -f "$@"
    else
        docker-compose logs -f "$@"
    fi
}

# 查看状态
status() {
    log_info "服务状态:"
    docker ps --filter "name=pi-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
}

# 清理
clean() {
    log_warn "这将删除所有容器和卷数据"
    read -p "确定继续? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cd "$DOCKER_DIR"
        
        if docker compose version &> /dev/null; then
            docker compose down -v
        else
            docker-compose down -v
        fi
        
        log_info "清理完成"
    fi
}

# 进入 Pi 容器
shell() {
    log_info "进入 Pi 容器..."
    docker exec -it pi-sandbox bash
}

# 测试连接
test() {
    log_info "测试 Pi 连接..."
    
    # 发送测试请求
    echo '{"type": "prompt", "message": "Hello, respond with OK"}' | \
        docker exec -i pi-sandbox pi --mode rpc --no-session 2>&1 | head -5
    
    if [ $? -eq 0 ]; then
        log_info "连接测试成功"
    else
        log_error "连接测试失败"
        exit 1
    fi
}

# 显示帮助
help() {
    echo "Pi 沙盒管理脚本"
    echo ""
    echo "用法: $0 <command>"
    echo ""
    echo "命令:"
    echo "  build     构建 Docker 镜像"
    echo "  start     启动服务"
    echo "  stop      停止服务"
    echo "  restart   重启服务"
    echo "  logs      查看日志"
    echo "  status    查看状态"
    echo "  shell     进入 Pi 容器"
    echo "  test      测试连接"
    echo "  clean     清理所有数据"
    echo "  help      显示此帮助"
}

# 主函数
main() {
    check_docker
    check_env
    create_dirs
    
    case "${1:-help}" in
        build)
            build
            ;;
        start)
            start
            ;;
        stop)
            stop
            ;;
        restart)
            restart
            ;;
        logs)
            shift
            logs "$@"
            ;;
        status)
            status
            ;;
        shell)
            shell
            ;;
        test)
            test
            ;;
        clean)
            clean
            ;;
        help|*)
            help
            ;;
    esac
}

main "$@"
