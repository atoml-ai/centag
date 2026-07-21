#!/bin/bash

# 前端独立构建脚本
# 用于前端服务的独立构建和开发

set -euo pipefail

# 颜色定义
RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
NC=$'\033[0m'

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# 获取项目根目录（web/ 的上一级）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WEBUI_DIR="$SCRIPT_DIR"

# shellcheck source=scripts/lib/centag-layout.sh
source "${PROJECT_ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init
STATIC_DIR="${CENTAG_STATIC_DIR}"
BIN_DIR="${CENTAG_EDITION_LIB}"

cd "$PROJECT_ROOT" || exit 1

# 检查 Node.js 环境
check_node() {
    if ! command -v node >/dev/null 2>&1; then
        print_error "Node.js 未安装"
        print_warn "请访问 https://nodejs.org/ 安装 Node.js (建议 >= 18.0.0)"
        exit 1
    fi

    if ! command -v npm >/dev/null 2>&1; then
        print_error "npm 未安装"
        print_warn "请安装 Node.js 来获取 npm"
        exit 1
    fi

    local node_version
    node_version=$(node -v | cut -d'v' -f2 | cut -d'.' -f1)
    if [ "$node_version" -lt 18 ]; then
        local current_v
        current_v=$(node -v)
        print_warn "Node.js 版本较低（${current_v}），建议使用 >= 18.0.0"
    fi

    local current_version
    current_version=$(node -v)
    print_success "Node.js 环境检查通过 (${current_version})"
}

# 安装依赖
install_deps() {
    if [ ! -d "$WEBUI_DIR" ]; then
        print_error "webui 目录不存在: $WEBUI_DIR"
        exit 1
    fi

    cd "$WEBUI_DIR"

    if [ ! -d "node_modules" ]; then
        print_info "安装 Web UI 依赖..."
        npm install
        if [ $? -ne 0 ]; then
            print_error "依赖安装失败"
            exit 1
        fi
        print_success "依赖安装完成"
    fi

    cd "$PROJECT_ROOT"
}

# 开发模式启动
dev() {
    print_info "启动 Web UI 开发服务器..."
    check_node
    install_deps

    cd "$WEBUI_DIR"

    local webui_port=5173
    print_info "检查端口 $webui_port..."

    # 清理可能占用端口的 Node/Vite 进程
    local node_pids
    node_pids=$(ps aux | grep -E "vite|node.*webui|npm.*dev" | grep -v grep | awk '{print $2}' || true)

    if [ -n "$node_pids" ]; then
        print_warn "发现可能相关的 Node/Vite 进程: $node_pids"
        print_info "正在清理..."
        for pid in $node_pids; do
            if kill -0 "$pid" 2>/dev/null; then
                print_info "停止进程 $pid..."
                kill -TERM "$pid" 2>/dev/null || true
            fi
        done
        sleep 2
        for pid in $node_pids; do
            if kill -0 "$pid" 2>/dev/null; then
                print_warn "强制停止进程 $pid..."
                kill -9 "$pid" 2>/dev/null || true
            fi
        done
        sleep 1
    fi

    # 清理可能占用端口的进程
    if command -v lsof >/dev/null 2>&1; then
        local port_pids
        port_pids=$(lsof -ti ":$webui_port" 2>/dev/null || true)
        if [ -n "$port_pids" ]; then
            print_warn "发现占用端口 $webui_port 的进程: $port_pids"
            print_info "正在清理..."
            for pid in $port_pids; do
                if kill -0 "$pid" 2>/dev/null; then
                    print_info "停止进程 $pid..."
                    kill -TERM "$pid" 2>/dev/null || true
                fi
            done
            sleep 2
            port_pids=$(lsof -ti ":$webui_port" 2>/dev/null || true)
            for pid in $port_pids; do
                if kill -0 "$pid" 2>/dev/null; then
                    print_warn "强制停止进程 $pid..."
                    kill -9 "$pid" 2>/dev/null || true
                fi
            done
            sleep 1
            print_success "端口 $webui_port 已清理"
        fi
    fi

    print_info "启动开发服务器 (http://localhost:5173)..."
    VITE_PORT=5173 npm run dev
}

# 生产构建
build() {
    print_info "构建 Web UI (生产环境)..."
    check_node
    install_deps

    cd "$WEBUI_DIR"

    print_info "开始构建 → $STATIC_DIR ..."
    mkdir -p "$STATIC_DIR"
    export CENTAG_INSTALL_ROOT CENTAG_EDITION CENTAG_STATIC_DIR="$STATIC_DIR"
    npm run build

    if [ $? -eq 0 ]; then
        print_success "Web UI 构建完成!"
        print_info "构建产物位置: $STATIC_DIR"
        cd "$PROJECT_ROOT"
    else
        print_error "Web UI 构建失败"
        cd "$PROJECT_ROOT"
        exit 1
    fi
}

# 代码检查
lint() {
    print_info "检查 Web UI 代码..."
    check_node
    install_deps

    cd "$WEBUI_DIR"
    npm run lint
    cd "$PROJECT_ROOT"
}

# 清理构建产物
clean() {
    print_info "清理 Web UI 构建产物..."
    if [ -d "$STATIC_DIR" ]; then
        rm -rf "$STATIC_DIR"
        print_success "清理完成: $STATIC_DIR"
    fi
}

# 帮助信息
help() {
    cat << EOF
${GREEN}Web UI 构建脚本${NC}

${BLUE}用法:${NC} $0 <命令>

${BLUE}可用命令:${NC}
  ${GREEN}dev${NC}       启动开发服务器 (http://localhost:5173)
  ${GREEN}build${NC}     构建生产版本 (输出到 ~/.centag/lib/<edition>/static)
  ${GREEN}lint${NC}      代码检查
  ${GREEN}clean${NC}     清理构建产物
  ${GREEN}help${NC}      显示帮助信息

${BLUE}工作目录:${NC}
  源代码:      $WEBUI_DIR
  构建输出:    $STATIC_DIR
  运行目录:    $BIN_DIR/static

${BLUE}示例:${NC}
  $0 dev          # 开发模式
  $0 build        # 生产构建
  $0 clean        # 清理

EOF
}

# 主函数
main() {
    local cmd="${1:-help}"
    print_warn "build-frontend.sh 为兼容入口，建议使用 make frontend 或 ./start.sh build fe"

    case "$cmd" in
        dev)
            dev
            ;;
        build)
            build
            ;;
        lint)
            lint
            ;;
        clean)
            clean
            ;;
        help|--help|-h)
            help
            ;;
        *)
            print_error "未知命令: $cmd"
            help
            exit 1
            ;;
    esac
}

main "$@"
