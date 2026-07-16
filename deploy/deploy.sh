#!/bin/bash
#
# Centag 应用容器部署（中间件请使用 deploy/stack）
#

set -euo pipefail

readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m'

readonly PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly DOCKER_DIR="${PROJECT_ROOT}/deploy/docker"
readonly SECRETS_DIR="${PROJECT_ROOT}/secrets"

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_docker() {
    print_info "检查 Docker 环境..."
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装"
        exit 1
    fi
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_error "Docker Compose 未安装"
        exit 1
    fi
    if ! docker info &> /dev/null; then
        print_error "Docker 服务未运行"
        exit 1
    fi
    print_success "Docker 环境检查通过"
}

check_env() {
    print_info "检查环境配置..."
    if [[ ! -f "${SECRETS_DIR}/.env" ]]; then
        print_warn "未找到 config/secrets/.env，请从 config/secrets/.env.example 复制并填写（至少 POSTGRES_PASSWORD 等）"
        exit 1
    fi
    set -a
    # shellcheck source=/dev/null
    source "${SECRETS_DIR}/.env"
    set +a
    if [[ -z "${POSTGRES_PASSWORD:-}" ]]; then
        print_warn "POSTGRES_PASSWORD 未设置，应用可能无法连接数据库"
    fi
    print_success "环境配置已加载"
}

deploy_centag() {
    print_info "构建并启动 Centag 容器..."
    cd "${DOCKER_DIR}"
    local compose_cmd="docker-compose"
    if ! command -v docker-compose &> /dev/null; then
        compose_cmd="docker compose"
    fi
    $compose_cmd --env-file "${SECRETS_DIR}/.env" up -d --build centag

    print_info "等待 Centag 就绪..."
    local port="${BACKEND_PORT:-${LLM_PROXY_SERVER_PORT:-20060}}"
    for _ in {1..30}; do
        if curl -sf "http://localhost:${port}/health" &> /dev/null; then
            print_success "Centag 已就绪: http://localhost:${port}"
            return 0
        fi
        sleep 2
    done
    print_error "Centag 启动超时，请检查: docker logs centag"
    return 1
}

show_status() {
    print_info "容器状态:"
    cd "${DOCKER_DIR}"
    local compose_cmd="docker-compose"
    if ! command -v docker-compose &> /dev/null; then
        compose_cmd="docker compose"
    fi
    $compose_cmd --env-file "${SECRETS_DIR}/.env" ps || true
    local port="${BACKEND_PORT:-${LLM_PROXY_SERVER_PORT:-20060}}"
    if curl -sf "http://localhost:${port}/health" &> /dev/null; then
        print_success "Centag /health: OK"
    else
        print_warn "Centag /health: 未就绪"
    fi
}

stop_services() {
    print_info "停止服务..."
    cd "${DOCKER_DIR}"
    local compose_cmd="docker-compose"
    if ! command -v docker-compose &> /dev/null; then
        compose_cmd="docker compose"
    fi
    $compose_cmd --env-file "${SECRETS_DIR}/.env" down
    print_success "已停止"
}

main() {
    case "${1:-deploy}" in
        deploy)
            check_docker
            check_env
            deploy_centag
            show_status
            ;;
        status)
            check_docker
            check_env
            show_status
            ;;
        stop)
            check_docker
            stop_services
            ;;
        *)
            echo "用法: $0 {deploy|status|stop}"
            echo "  deploy  构建并启动 centag（docker/docker-compose.yaml）"
            echo "  status  查看 compose 状态与健康检查"
            echo "  stop    docker compose down"
            echo ""
            echo "PostgreSQL / Redis / Mem0 等请使用子项目 deploy/stack。"
            exit 1
            ;;
    esac
}

main "$@"
