#!/bin/bash

# Centag - Build, Run, Debug Script (Linux/macOS)

set -euo pipefail
shopt -s nullglob

# Go module proxy (China mirror)
export GOPROXY=${GOPROXY:-https://goproxy.cn,direct}

# Docker BuildKit (加速构建，支持 cache mount)
export DOCKER_BUILDKIT=1

# Colors
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly NC='\033[0m'

# Project config
readonly PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_PORT=20060

cd "$PROJECT_ROOT" || exit 1

# Install-compatible layout (default: ~/.centag — override with CENTAG_INSTALL_ROOT)
# shellcheck source=scripts/lib/centag-layout.sh
source "${PROJECT_ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init
# Compatibility aliases used throughout this script:
#   BIN_DIR     = lib/<edition> (binary + static + storage/logs)
#   SERVER_BIN  = centag-<edition>
BIN_DIR="${CENTAG_EDITION_LIB}"
PACKAGES_DIR="${CENTAG_PACKAGES_DIR}"
STATIC_DIR="${CENTAG_STATIC_DIR}"
SERVER_BIN="${CENTAG_SERVER_BIN}"

# Switch active edition paths (personal|minimal|...).
centag_set_edition() {
    centag_layout_use_edition "$1"
    BIN_DIR="${CENTAG_EDITION_LIB}"
    PACKAGES_DIR="${CENTAG_PACKAGES_DIR}"
    STATIC_DIR="${CENTAG_STATIC_DIR}"
    SERVER_BIN="${CENTAG_SERVER_BIN}"
}

# Allow Go to automatically download required toolchain
export GOTOOLCHAIN=auto

# Add Go bin to PATH
if command -v go >/dev/null 2>&1; then
    export PATH="$PATH:$(go env GOPATH)/bin"
fi

# 产品版本（注入 main.Version / `centag version`）—
# 优先版本分支名 feature/v0.2.7 → v0.2.7，其次 git tag（见 scripts/lib/centag-version.sh）。
# VERSION 仍为构建时间戳，仅用于横幅「构建号」，不写入二进制 Version。
# shellcheck source=scripts/lib/centag-version.sh
source "${PROJECT_ROOT}/scripts/lib/centag-version.sh"
CENTAG_VERSION="$(centag_resolve_version)"
VERSION=$(date '+%Y%m%d-%H%M%S')
BUILD_TIME=$(date '+%Y-%m-%d %H:%M:%S')

# Check Go
check_go() {
    if ! command -v go >/dev/null 2>&1; then
        echo "[ERROR] Go not installed, visit https://go.dev/dl/"
        exit 1
    fi
}

# Kill port
kill_port() {
    local port=$1
    local pids=""
    local found=0

    if [ -z "$port" ]; then
        print_error "Port number required"
        return 1
    fi

    print_info "Checking port $port for running processes..."

    # Docker-aware: if port is mapped by a container, stop it gracefully
    if command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' 2>/dev/null | grep -q .; then
        local container_on_port
        container_on_port=$(docker ps --filter "publish=$port" --format '{{.Names}}' 2>/dev/null | head -1)
        if [ -n "$container_on_port" ]; then
            print_warn "Port $port is used by Docker container: $container_on_port"
            print_info "Stopping container $container_on_port..."
            if docker stop "$container_on_port" >/dev/null 2>&1; then
                print_success "Container $container_on_port stopped"
                return 0
            fi
        fi
    fi

    # 方法1: 使用 lsof (最可靠)
    if command -v lsof >/dev/null 2>&1; then
        pids=$(lsof -ti ":$port" 2>/dev/null || true)

        if [ -n "$pids" ]; then
            # Filter out Docker proxy/hyperkit PIDs to avoid killing Docker
            local filtered_pids=""
            for pid in $pids; do
                local comm
                comm=$(ps -p "$pid" -o comm= 2>/dev/null || true)
                if echo "$comm" | grep -qiE "docker|com\.docker|vpnkit|hyperkit"; then
                    print_info "Skipping Docker-related PID $pid ($comm)"
                else
                    filtered_pids="$filtered_pids $pid"
                fi
            done
            pids="$filtered_pids"

            if [ -n "$pids" ]; then
                print_warn "Found processes on port $port (lsof): $pids"
                found=1

                # 尝试正常终止
                for pid in $pids; do
                    if kill -0 "$pid" 2>/dev/null; then
                        print_info "Sending TERM signal to PID $pid..."
                        kill -TERM "$pid" 2>/dev/null || true
                    fi
                done

                # 等待进程退出
                sleep 2

                # 检查是否还存活,强制杀死
                for pid in $pids; do
                    if kill -0 "$pid" 2>/dev/null; then
                        print_warn "Process $pid still alive, forcing kill..."
                        kill -9 "$pid" 2>/dev/null || true
                    fi
                done

                sleep 1
            fi
        fi
    fi

    # 方法2: 使用 fuser
    if [ $found -eq 0 ] && command -v fuser >/dev/null 2>&1; then
        pids=$(fuser "$port/tcp" 2>/dev/null || true)
        if [ -n "$pids" ]; then
            print_warn "Found processes on port $port (fuser): $pids"
            found=1
            kill -9 $pids 2>/dev/null || true
            sleep 1
        fi
    fi

    # 方法3: 使用 ss
    if [ $found -eq 0 ] && command -v ss >/dev/null 2>&1; then
        pids=$(ss -tlnp 2>/dev/null | grep ":$port " | awk '{print $6}' | sed 's/.*pid=//' | sed 's/,.*//' | grep -E '^[0-9]+$' || true)

        if [ -n "$pids" ]; then
            print_warn "Found processes on port $port (ss): $pids"
            found=1
            echo "$pids" | xargs kill -9 2>/dev/null || true
            sleep 1
        fi
    fi

    # 方法4: 使用 netstat
    if [ $found -eq 0 ] && command -v netstat >/dev/null 2>&1; then
        pids=$(netstat -tlnp 2>/dev/null | grep ":$port " | awk '{print $7}' | sed 's/\/.*//' | grep -E '^[0-9]+$' || true)

        if [ -n "$pids" ]; then
            print_warn "Found processes on port $port (netstat): $pids"
            found=1
            echo "$pids" | xargs kill -9 2>/dev/null || true
            sleep 1
        fi
    fi

    # 方法5: 仅清理当前项目目录下的 centag 进程 (最后手段)
    pids=$(pgrep -f "${BIN_DIR}/${SERVER_BIN}" || true)
    if [ -n "$pids" ]; then
        print_warn "Found centag binary processes: $pids"
        found=1
        for pid in $pids; do
            print_info "Killing centag process $pid..."
            kill -9 "$pid" 2>/dev/null || true
        done
        sleep 1
    fi

    # 最终验证
    if command -v lsof >/dev/null 2>&1; then
        local remaining=$(lsof -ti ":$port" 2>/dev/null || true)
        if [ -n "$remaining" ]; then
            print_error "Failed to kill all processes on port $port: $remaining"
            return 1
        fi
    fi

    if [ $found -eq 1 ]; then
        print_success "Port $port cleaned successfully"
    else
        print_info "No processes found on port $port"
    fi

    return 0
}

# 自动寻找可用端口（递增策略，不删除占用进程/容器）
resolve_backend_port() {
    local port=$BACKEND_PORT
    local max_attempts=10

    for ((i=0; i<=max_attempts; i++)); do
        local candidate=$((port + i))
        local in_use=false

        # Docker 容器映射优先检测（避免 Docker Desktop 残留进程导致误判）
        if command -v docker >/dev/null 2>&1; then
            if docker ps --filter "publish=$candidate" --format '{{.Names}}' 2>/dev/null | grep -q .; then
                in_use=true
            fi
        fi

        if ! $in_use && command -v lsof >/dev/null 2>&1; then
            if lsof -ti ":$candidate" 2>/dev/null | grep -q .; then
                # 检查是否只有 Docker 代理进程残留（无对应运行容器）
                local has_non_docker=false
                for pid in $(lsof -ti ":$candidate" 2>/dev/null); do
                    local comm
                    comm=$(ps -p "$pid" -o comm= 2>/dev/null || true)
                    if ! echo "$comm" | grep -qiE "docker|com\.docker|vpnkit|hyperkit"; then
                        has_non_docker=true
                        break
                    fi
                done
                $has_non_docker && in_use=true
            fi
        fi

        if ! $in_use; then
            if [ "$candidate" -ne "$BACKEND_PORT" ]; then
                print_warn "端口 $BACKEND_PORT 已被占用，自动切换到端口 $candidate"
                BACKEND_PORT=$candidate
            fi
            return 0
        fi
    done

    print_error "端口 $port-$((port+max_attempts)) 均被占用，请手动释放后重试"
    return 1
}

# 清理所有 centag 残留进程（debug 前调用，保证前台独占）
cleanup_residual_processes() {
    local cleaned=0

    # 1. 清理残留的守护进程（daemon.sh）
    local daemon_pids
    daemon_pids=$(pgrep -f "daemon\.sh.*centag" 2>/dev/null || true)
    if [ -n "$daemon_pids" ]; then
        print_warn "清理残留守护进程: $daemon_pids"
        echo "$daemon_pids" | xargs kill -9 2>/dev/null || true
        cleaned=1
    fi
    rm -f "$BIN_DIR/storage/centag.daemon.pid" 2>/dev/null || true

    # 2. 清理残留的 centag 主进程
    local proxy_pids
    proxy_pids=$(pgrep -f "\./centag$|/centag$" 2>/dev/null || true)
    if [ -n "$proxy_pids" ]; then
        print_warn "清理残留 centag 进程: $proxy_pids"
        echo "$proxy_pids" | xargs kill -9 2>/dev/null || true
        cleaned=1
    fi

    # 3. 清理残留的 tail 日志进程
    local tail_pids
    tail_pids=$(pgrep -f "tail.*centag" 2>/dev/null || true)
    if [ -n "$tail_pids" ]; then
        print_warn "清理残留 tail 进程: $tail_pids"
        echo "$tail_pids" | xargs kill -9 2>/dev/null || true
        cleaned=1
    fi

    # 4. 清理残留的 Vite watch 进程
    local vite_pids
    vite_pids=$(pgrep -f "vite.*build.*watch" 2>/dev/null || true)
    if [ -n "$vite_pids" ]; then
        print_warn "清理残留 Vite 进程: $vite_pids"
        echo "$vite_pids" | xargs kill -9 2>/dev/null || true
        cleaned=1
    fi

    # 5. 清理残留的 PID 文件
    rm -f "$BIN_DIR/storage/centag.pid" 2>/dev/null || true

    if [ "$cleaned" -eq 1 ]; then
        sleep 1
        print_success "残留进程已清理"
    fi
}

# 显示消息（统一风格）
print_message() { echo -e "${1}${2}${NC}"; }
print_info()    { print_message "$BLUE" "[INFO] $1"; }
print_success() { print_message "$GREEN" "[SUCCESS] $1"; }
print_error()   { print_message "$RED" "[ERROR] $1"; }
print_warn()    { print_message "$YELLOW" "[WARN] $1"; }
print_title() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}    Centag Manager${NC}"
    echo -e "${BLUE}================================${NC}"
}

# 显示版本信息
show_version() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC}              ${GREEN}Centag${NC}                        ${BLUE}║${NC}"
    echo -e "${BLUE}╠══════════════════════════════════════════════╣${NC}"
    echo -e "${BLUE}║${NC}  ${CYAN}版本:${NC}       ${GREEN}${CENTAG_VERSION}$(printf '%*s' $((28-${#CENTAG_VERSION})) '')${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  ${CYAN}构建号:${NC}     ${GREEN}${VERSION}$(printf '%*s' $((28-${#VERSION})) '')${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  ${CYAN}构建时间:${NC}   ${GREEN}${BUILD_TIME}$(printf '%*s' $((28-${#BUILD_TIME})) '')${BLUE}║${NC}"
    if command -v go >/dev/null 2>&1; then
        local go_ver=$(go version 2>/dev/null | awk '{print $3}')
        echo -e "${BLUE}║${NC}  ${CYAN}Go:${NC}         ${GREEN}${go_ver}$(printf '%*s' $((28-${#go_ver})) '')${BLUE}║${NC}"
    fi
    echo -e "${BLUE}╚══════════════════════════════════════════════╝${NC}"
    echo ""
    print_info "运行 ${GREEN}./start.sh --help${NC} 查看命令列表"
    echo ""
}

# Wizard 读取输入
wizard_read() {
    local prompt="$1"
    local default="${2:-}"
    local input=""

    # 将提示输出到 stderr，这样不会污染 stdout 的返回值
    if [ -n "$default" ]; then
        echo -ne "${CYAN}${prompt} [${default}]: ${NC}" >&2
    else
        echo -ne "${CYAN}${prompt}: ${NC}" >&2
    fi

    # 从 /dev/tty 读取输入，确保即使脚本在管道中也能正常工作
    if [ -t 0 ]; then
        # 标准输入是终端，直接读取
        read -r input
    else
        # 标准输入被重定向，尝试从终端设备读取
        read -r input < /dev/tty 2>/dev/null || input=""
    fi

    # 输出结果
    echo "${input:-$default}"
}

# Wizard 确认提示
wizard_confirm() {
    local prompt="$1"
    local default="${2:-y}"
    local input

    if [ "$default" = "y" ]; then
        echo -ne "${YELLOW}${prompt} [Y/n]: ${NC}"
    else
        echo -ne "${YELLOW}${prompt} [y/N]: ${NC}"
    fi

    read -r input
    input="${input:-$default}"
    if [[ "$input" =~ ^[Yy]$ ]]; then
        return 0
    else
        return 1
    fi
}

# Wizard 显示步骤标题
wizard_step() {
    local step_num="$1"
    local title="$2"
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  步骤 ${step_num}: ${title}${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

# Setup
setup() {
    print_info "Setting up development environment..."
    check_go

    # Set Go proxy for better connectivity in China
    print_info "Configuring Go proxy..."
    print_success "Go proxy set to: https://goproxy.cn"

    # Download dependencies (exclude deploy/stack subproject - it's completely decoupled)
    print_info "Downloading dependencies..."
    local stack_moved=""
    if [ -d "deploy/stack" ]; then
        stack_moved="true"
        mkdir -p /tmp/deploy
        mv deploy/stack /tmp/deploy/stack-temp
    fi
    go mod tidy
    local tidy_result=$?
    if [ -n "$stack_moved" ] && [ -d "/tmp/deploy/stack-temp" ]; then
        mv /tmp/deploy/stack-temp deploy/stack
    fi
    if [ $tidy_result -ne 0 ]; then
        print_error "Failed to download dependencies"
        exit 1
    fi

    mkdir -p "$BIN_DIR"
    mkdir -p "$PACKAGES_DIR"
    mkdir -p "$STATIC_DIR"

    # Copy configuration files and scripts to bin directory
    print_info "Copying configuration files and scripts..."
    make copy-files

    print_success "Environment setup complete"
}

# 检查系统依赖
check_dependencies() {
    local deps_ok=true

    print_message $BLUE "🔍 检查系统依赖..."
    echo ""

    # 检查 Go
    if command -v go >/dev/null 2>&1; then
        local go_version=$(go version | awk '{print $3}')
        print_message $GREEN "  ✅ Go: ${go_version}"
    else
        print_message $RED "  ❌ Go: 未安装"
        print_message $YELLOW "     请访问 https://golang.org/dl/ 安装 Go 1.21+"
        deps_ok=false
    fi

    # 检查 Docker
    if command -v docker >/dev/null 2>&1; then
        local docker_version=$(docker --version | awk '{print $3}' | tr -d ',')
        print_message $GREEN "  ✅ Docker: ${docker_version}"

        # 检查 Docker 是否运行
        if docker info >/dev/null 2>&1; then
            print_message $GREEN "  ✅ Docker 守护进程: 运行中"
        else
            print_message $RED "  ❌ Docker 守护进程: 未运行"
            print_message $YELLOW "     请启动 Docker Desktop 或 dockerd 服务"
            deps_ok=false
        fi
    else
        print_message $YELLOW "  ⚠️  Docker: 未安装（可选，用于 Docker 部署）"
    fi

    # 检查 docker-compose
    if command -v docker-compose >/dev/null 2>&1 || docker compose version >/dev/null 2>&1; then
        print_message $GREEN "  ✅ Docker Compose: 已安装"
    else
        print_message $YELLOW "  ⚠️  Docker Compose: 未安装（可选，用于中间件管理）"
    fi

    # 检查 Node.js（用于 Vue Web UI）
    if command -v node >/dev/null 2>&1; then
        local node_version=$(node -v)
        print_message $GREEN "  ✅ Node.js: ${node_version}"

        # 检查 npm
        if command -v npm >/dev/null 2>&1; then
            local npm_version=$(npm -v)
            print_message $GREEN "  ✅ npm: ${npm_version}"
        else
            print_message $YELLOW "  ⚠️  npm: 未安装"
        fi
    else
        print_message $YELLOW "  ⚠️  Node.js: 未安装（可选，用于 Vue Web UI 开发）"
        print_message $YELLOW "     WebUI 需要 Node.js ^20.19.0 或 >=22.12.0（推荐 nvm use 22）"
    fi

    echo ""

    if $deps_ok; then
        print_message $GREEN "✅ 依赖检查通过"
        return 0
    else
        print_message $YELLOW "⚠️  部分依赖缺失，某些功能可能不可用"
        return 1
    fi
}

# ── 共享：获取发行版构建标签 ──────────────────────────────────────
# 所有构建路径（本地 build、Docker、debug）通过此函数获取统一的 tags
# personal 全功能 tags；team 商业二进制在 centag-pro 构建（开源无 dist/team）
_FULL_FEATURE_TAGS="protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure"

_get_dist_tags() {
    local dist_name="$1"
    case "$dist_name" in
        minimal)
            echo "minimal,protocol_openai,protocol_anthropic,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic"
            ;;
        personal)
            echo "$_FULL_FEATURE_TAGS"
            ;;
        team)
            # Team SKU is not built from open-source dist/; tags used only if pro build reuses this helper via env.
            echo "$_FULL_FEATURE_TAGS"
            ;;
        *)
            echo ""
            ;;
    esac
}

# Team SKU is built only in the private centag-pro repo (no OSS convenience wrapper).
_reject_team_build_in_oss() {
    local via="${1:-build}"
    print_error "开源仓不再提供 Team 构建入口（含转调）。"
    print_info "请在私有仓 centag-pro 构建（用法与本仓对齐）："
    print_info "  cd ../centag-pro   # 或你的 checkout 路径；分支须与 centag 同名"
    print_info "  export CENTAG_ROOT=${PROJECT_ROOT}"
    print_info "  ./start.sh build team                 # 后端 → bin/centag-team"
    print_info "  ./start.sh build fe                   # Team 前端"
    print_info "  ./start.sh build all                  # 后端 + 前端"
    if [ "$via" = "debug" ]; then
        print_info "  ./start.sh debug team                 # 开发模式（后端 + Team 前端 watch）"
    fi
    print_info "详见 centag-pro/README.md"
    exit 1
}

# ── 共享：编译 Go 二进制 ──────────────────────────────────────────
# 参数: src_dir package_path output_name build_tags extra_ldflags
#   src_dir       — 包含 go.mod 的目录（如 dist/minimal 或项目根）
#   package_path  — 要编译的包路径（如 "." 或 "cmd/centag/main.go"）
#   output_name   — 输出二进制文件名（仅文件名，不含路径）
#   build_tags    — 构建标签（逗号分隔，可选）
#   extra_ldflags — 额外的 ldflags（如版本号，可选）
_compile_go_binary() {
    local src_dir="$1"
    local package_path="$2"
    local output_name="$3"
    local build_tags="${4:-}"
    local extra_ldflags="${5:-}"

    cd "$src_dir"

    # 更新依赖
    go mod tidy 2>/dev/null || true

    # 组装 -tags 参数
    local tags_arg=""
    [ -n "$build_tags" ] && tags_arg="-tags '$build_tags'"

    # 组装 -ldflags 参数
    local ldflags_str="-s -w"
    [ -n "$extra_ldflags" ] && ldflags_str="$ldflags_str $extra_ldflags"

    local edition=""
    case "$output_name" in
        centag-*) edition="${output_name#centag-}" ;;
        centag) edition="${CENTAG_EDITION:-personal}"; output_name="centag-${edition}" ;;
        *) edition="${CENTAG_EDITION:-personal}" ;;
    esac
    local out_dir
    out_dir="$(centag_edition_lib "$edition")"
    mkdir -p "$out_dir"

    print_info "编译 ${output_name} → ${out_dir}/ ..."
    eval go build $tags_arg -ldflags \"$ldflags_str\" -o "${out_dir}/${output_name}" "$package_path"

    if [ $? -ne 0 ]; then
        print_error "编译失败: ${output_name}"
        cd "$PROJECT_ROOT"
        exit 1
    fi

    centag_install_edition_links "$edition"

    local size=$(ls -lh "${out_dir}/${output_name}" | awk '{print $5}')
    print_success "${output_name} 编译完成 (${size})"
    print_info "二进制: ${out_dir}/${output_name}"

    cd "$PROJECT_ROOT"
}

# Build
build() {
    local target="${1:-all}"
    local dist="${2:-}"

    case "$target" in
        all)
            print_info "Building all components..."
            webui_build
            build_backend
            print_success "All components built successfully!"
            ;;
        backend)
            print_info "Building backend..."
            build_backend
            ;;
        dist)
            build_distribution "$dist"
            ;;
        webui)
            print_info "Building Web UI..."
            webui_build
            ;;
        *)
            print_error "Unknown build target: $target"
            print_info "Valid targets: all, backend, dist, webui"
            exit 1
            ;;
    esac
}

# Map product edition → dist binary name.
edition_to_dist() {
    case "$1" in
        personal) echo "personal" ;;
        minimal) echo "minimal" ;;
        team) echo "team" ;;
        *) echo "$1" ;;
    esac
}

edition_to_sidecar() {
    case "$1" in
        personal) echo "centag-personal" ;;
        minimal) echo "centag-minimal" ;;
        team) echo "centag-team" ;;
        *) echo "centag-$1" ;;
    esac
}

# Resolve current-host launcher binary (auto-detect GOOS/GOARCH).
resolve_launcher_bin() {
    local goos goarch ext plat
    goos="$(go env GOOS 2>/dev/null || echo "")"
    goarch="$(go env GOARCH 2>/dev/null || echo "")"
    ext=""
    if [ "$goos" = "windows" ]; then
        ext=".exe"
    fi
    plat="${CENTAG_CROSS_DIR}/launcher/${goos}-${goarch}/centag-launcher${ext}"
    if [ -x "$plat" ] || [ -f "$plat" ]; then
        echo "$plat"
        return 0
    fi
    local latest="${CENTAG_BIN_DIR}/centag-launcher${ext}"
    if [ -x "$latest" ] || [ -f "$latest" ]; then
        echo "$latest"
        return 0
    fi
    return 1
}

# Desktop shell (apps/launcher, systray). Product forms: cli | desktop.
# Used via --desktop on build/run.
build_desktop_shell() {
    local script="${PROJECT_ROOT}/scripts/build-launcher.sh"
    if [ ! -x "$script" ]; then
        print_error "未找到构建脚本: $script"
        exit 1
    fi
    print_info "Building desktop shell (CGO) for current host ($(go env GOOS)/$(go env GOARCH))..."
    bash "$script" --desktop
}

resolve_desktop_bin() {
    local goos goarch ext plat latest
    goos="$(go env GOOS 2>/dev/null || echo "")"
    goarch="$(go env GOARCH 2>/dev/null || echo "")"
    ext=""
    if [ "$goos" = "windows" ]; then
        ext=".exe"
    fi
    plat="${CENTAG_CROSS_DIR}/launcher/${goos}-${goarch}/centag-desktop${ext}"
    if [ -x "$plat" ] || [ -f "$plat" ]; then
        echo "$plat"
        return 0
    fi
    latest="${CENTAG_BIN_DIR}/centag-desktop${ext}"
    if [ -x "$latest" ] || [ -f "$latest" ]; then
        echo "$latest"
        return 0
    fi
    return 1
}

# Resolve current-host wrap binary (auto-detect GOOS/GOARCH).
resolve_wrap_bin() {
    local goos goarch ext plat
    goos="$(go env GOOS 2>/dev/null || echo "")"
    goarch="$(go env GOARCH 2>/dev/null || echo "")"
    ext=""
    if [ "$goos" = "windows" ]; then
        ext=".exe"
    fi
    plat="${CENTAG_CROSS_DIR}/wrap/${goos}-${goarch}/centag-wrap${ext}"
    if [ -x "$plat" ] || [ -f "$plat" ]; then
        echo "$plat"
        return 0
    fi
    local latest="${CENTAG_BIN_DIR}/centag-wrap${ext}"
    if [ -x "$latest" ] || [ -f "$latest" ]; then
        echo "$latest"
        return 0
    fi
    return 1
}

# Optional system-proxy helper (apps/wrap). Used via build wrap / --wrap / run wrap.
# Independent go.mod; GOWORK=off. No CGO.
build_wrap_shell() {
    local script="${PROJECT_ROOT}/scripts/build-wrap.sh"
    if [ ! -x "$script" ]; then
        print_error "未找到构建脚本: $script"
        exit 1
    fi
    print_info "Building centag-wrap for current host ($(go env GOOS)/$(go env GOARCH))..."
    bash "$script"
}

# ./start.sh run wrap [--server URL] <enable|disable|doctor|status|run|env> ...
# Ensures binary exists, then execs the real CLI (same args as 真源 centag-wrap).
run_wrap() {
    local wrap_bin
    if ! wrap_bin="$(resolve_wrap_bin)"; then
        print_info "未找到 centag-wrap，先构建..."
        build_wrap_shell
        wrap_bin="$(resolve_wrap_bin)" || {
            print_error "centag-wrap 构建后仍未找到"
            exit 1
        }
    fi
    print_info "运行: ${wrap_bin} $*"
    # macOS bash 3.2 + set -u: empty "${arr[@]}" is "unbound variable"
    if [ "$#" -gt 0 ]; then
        exec "$wrap_bin" "$@"
    else
        exec "$wrap_bin"
    fi
}

# ./start.sh build <personal|minimal> --desktop
# Product forms: cli (default) | desktop (--desktop)
build_with_desktop() {
    local edition="$1"
    case "$edition" in
        personal|minimal) ;;
        team)
            print_error "--desktop 不支持 team（团队版请用 Web/Docker）"
            exit 1
            ;;
        *)
            print_error "--desktop 仅支持 personal / minimal"
            exit 1
            ;;
    esac

    local dist_name
    dist_name="$(edition_to_dist "$edition")"
    local label="$edition"

    print_info "Building ${label} service + desktop shell..."
    build_distribution "$dist_name"
    build_frontend_prod
    build_desktop_shell
    print_success "Ready: $(edition_to_sidecar "$edition") + desktop ($(go env GOOS)/$(go env GOARCH))"
}

# Build Distribution (minimal/personal/team)
build_distribution() {
    local dist_name="${1:-minimal}"

    case "$dist_name" in
        team)
            _reject_team_build_in_oss build
            ;;
        minimal|personal)
            ;;
        "")
            print_error "Please specify distribution: minimal or personal"
            print_info "Usage: $0 build <minimal|personal>"
            print_info "Team：cd ../centag-pro && ./start.sh build team"
            exit 1
            ;;
        *)
            print_error "Unknown distribution: $dist_name"
            print_info "Valid distributions: minimal, personal（Team → centag-pro）"
            exit 1
            ;;
    esac

    local dist_dir="${PROJECT_ROOT}/dist/${dist_name}"
    if [ ! -d "$dist_dir" ]; then
        print_error "Distribution directory not found: $dist_dir"
        exit 1
    fi

    print_info "Building Centag $(echo "$dist_name" | awk '{print toupper(substr($0,1,1)) substr($0,2)}') distribution..."

    local go_tags=$(_get_dist_tags "$dist_name")
    local output_name="centag-${dist_name}"
    local ver_ldflags="-X 'main.Version=${CENTAG_VERSION}' -X 'main.BuildTime=${BUILD_TIME}'"

    _compile_go_binary "$dist_dir" "." "$output_name" "$go_tags" "$ver_ldflags"
}

# Build Backend
build_backend() {
    centag_set_edition "${CENTAG_EDITION:-personal}"
    mkdir -p "$BIN_DIR"
    CENTAG_INSTALL_ROOT="${CENTAG_INSTALL_ROOT}" CENTAG_EDITION="${CENTAG_EDITION}" make build
    centag_install_edition_links "${CENTAG_EDITION}"

    # 检查守护进程是否在运行
    local daemon_pid_file="$BIN_DIR/storage/centag.daemon.pid"
    local service_pid_file="$BIN_DIR/storage/centag.pid"

    local daemon_pid=""
    local service_pid=""
    local daemon_running=0

    if [ -f "$daemon_pid_file" ]; then
        daemon_pid=$(cat "$daemon_pid_file" 2>/dev/null || true)
        if [ -n "$daemon_pid" ] && kill -0 "$daemon_pid" 2>/dev/null; then
            daemon_running=1
        fi
    fi

    if [ -f "$service_pid_file" ]; then
        service_pid=$(cat "$service_pid_file" 2>/dev/null || true)
    fi

    if [ $daemon_running -eq 1 ]; then
        print_info "守护进程正在运行，触发服务更新..."
        # 杀掉服务进程，守护进程会自动重启
        if [ -n "$service_pid" ] && kill -0 "$service_pid" 2>/dev/null; then
            kill -TERM "$service_pid" 2>/dev/null || true
            print_success "已发送重启信号，守护进程将自动拉起新版本"
        else
            # 如果服务 PID 文件无效，尝试通过端口查找并杀掉
            if command -v lsof >/dev/null 2>&1; then
                local found_pid=$(lsof -ti ":$BACKEND_PORT" 2>/dev/null || true)
                if [ -n "$found_pid" ]; then
                    kill -TERM "$found_pid" 2>/dev/null || true
                    print_success "已发送重启信号，守护进程将自动拉起新版本"
                fi
            fi
        fi
    fi
}

# Build All (Backend + Web UI) - 保持向后兼容
build_all() {
    build all
}

# 交叉编译 OTA 服务端二进制（目标平台 ≠ 本机时自动调用）。
# 返回编译后的二进制路径；失败则 exit 1。
_ota_cross_build() {
    local edition="$1" goos="$2" goarch="$3" out_dir="$4"
    local out_bin="${out_dir}/centag-${edition}"
    mkdir -p "$out_dir"

    local ver_ldflags="-X 'main.Version=${CENTAG_VERSION}' -X 'main.BuildTime=${BUILD_TIME}'"
    local tags
    tags="$(_get_dist_tags "$edition")"

    case "$goos" in
        darwin|linux|windows) ;;
        *)
            print_error "不支持的目标系统: $goos（darwin|linux|windows）" >&2
            exit 1
            ;;
    esac
    case "$goarch" in
        amd64|arm64) ;;
        *)
            print_error "不支持的目标架构: $goarch（amd64|arm64）" >&2
            exit 1
            ;;
    esac

    if [ "$edition" = "team" ]; then
        # Team 商业二进制只在 centag-pro 构建（开源仓无 dist/team）
        local pro_root="${PROJECT_ROOT}/../centag-pro"
        if [ ! -f "$pro_root/scripts/build-team.sh" ]; then
            print_error "交叉编译 team 需要私有仓: $pro_root" >&2
            echo "  export CENTAG_ROOT=${PROJECT_ROOT}" >&2
            exit 1
        fi
        print_info "交叉编译 team ${goos}/${goarch}（centag-pro）..." >&2
        ( cd "$pro_root" \
            && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
               ASSEMBLE_OUT_DIR="$out_dir" ASSEMBLE_OUT="$out_bin" \
               CENTAG_ROOT="$PROJECT_ROOT" \
               bash "$pro_root/scripts/build-team.sh" >&2 ) \
            || { print_error "team 交叉编译失败" >&2; exit 1; }
    else
        # personal / minimal：开源 dist/<edition> 交叉编译
        local dist_dir="${PROJECT_ROOT}/dist/${edition}"
        if [ ! -d "$dist_dir" ]; then
print_error "版本目录不存在: $dist_dir" >&2
            exit 1
        fi
        print_info "交叉编译 ${edition} ${goos}/${goarch}（$dist_dir）..." >&2
        ( cd "$dist_dir" \
            && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
               go build -tags "$tags" -ldflags "-s -w $ver_ldflags" -o "$out_bin" . >&2 ) \
            || { print_error "${edition} 交叉编译失败" >&2; exit 1; }
    fi

    [ -f "$out_bin" ] || { print_error "交叉编译未产出 $out_bin" >&2; exit 1; }
    echo "$out_bin"
}

# 解析 pack/ota 的单平台 → 使用哪个源二进制
# $1=平台(goos-goarch 或空=本机) $2=版本 $3=跨平台产物目录
# 返回: 二进制路径
_ota_source_bin() {
    local pf="$1" edition="$2" cache_dir="$3"
    local host_goos host_goarch
    host_goos="$(go env GOOS 2>/dev/null || uname -s | tr '[:upper:]' '[:lower:]')"
    host_goarch="$(go env GOARCH 2>/dev/null || uname -m)"

    if [ -z "$pf" ]; then
        # 本机 → 使用已构建产物
        if [ ! -f "$BIN_DIR/$SERVER_BIN" ]; then
            print_error "未找到已编译的 centag（$BIN_DIR/$SERVER_BIN），请先: ./start.sh build" >&2
            exit 1
        fi
        echo "$BIN_DIR/$SERVER_BIN"
    else
        local pgoos="${pf%%-*}" pgoarch="${pf#*-}"
        if [ "$pgoos" = "$host_goos" ] && [ "$pgoarch" = "$host_goarch" ]; then
            if [ ! -f "$BIN_DIR/$SERVER_BIN" ]; then
                print_error "未找到本机二进制 $BIN_DIR/$SERVER_BIN，请先: ./start.sh build" >&2
                exit 1
            fi
            echo "$BIN_DIR/$SERVER_BIN"
        else
            _ota_cross_build "$edition" "$pgoos" "$pgoarch" "$cache_dir"
        fi
    fi
}

# Pack - 打包更新包（参考 gov-subscribe）
pack() {
    local upload=false
    local platforms=""
    local edition=""
    local package_script="${PROJECT_ROOT}/scripts/release/package.sh"

    # 解析参数（平台统一为 --platforms <goos-goarch,...>，与 cli 一致）
    while [ $# -gt 0 ]; do
        case "$1" in
            --upload)
                upload=true
                shift
                ;;
            --platforms)
                platforms="${2:-}"
                shift 2
                ;;
            --edition)
                edition="${2:-}"
                shift 2
                ;;
            --version)
                shift 2
                ;;
            *)
                print_error "未知参数: $1"
                echo "用法: $0 package ota [--upload] [--platforms <goos-goarch,...>] [--edition <personal|minimal|team>]"
                echo "  --upload     先构建，再打包，然后更新到容器"
                echo "  --platforms  目标平台 goos-goarch，逗号分隔可多个（默认本机）"
                echo "  --edition    版本（默认取当前布局 edition）"
                exit 1
                ;;
        esac
    done

    if [ ! -f "$package_script" ]; then
        print_error "统一打包脚本不存在: $package_script"
        exit 1
    fi

    # 如果指定 --upload，先构建
    if [ "$upload" = true ]; then
        print_info "=== 步骤 1/4: 构建项目 ==="
        build
    fi

    print_info "=== 步骤 2/4: 打包更新包 ==="
    print_info "通过统一脚本打包更新包..."

    # 版本默认值：中心 edition（布局）或 env
    local edition_default="${edition:-${CENTAG_EDITION:-${CENTAG_PACKAGE_EDITION:-personal}}}"

    # --edition 驱动布局：BIN_DIR/STATIC_DIR 切到对应 lib/<edition>，
    # 确保 OTA 包携带该版本的 Web 静态（否则会用默认 personal 的旧静态）。
    if [ -n "$edition_default" ]; then
        centag_set_edition "$edition_default"
    fi

    # 平台列表展开；空 → 默认本机（release 脚本自行推断 goos/goarch）
    local -a ota_platforms=()
    if [ -n "$platforms" ]; then
        IFS=',' read -r -a ota_platforms <<< "$platforms"
    else
        ota_platforms=("")
    fi
    if [ "$upload" = true ] && [ "${#ota_platforms[@]}" -gt 1 ]; then
        print_error "--upload 仅支持单平台发布，请指定 --platforms <单目标> 或省略（默认本机）"
        exit 1
    fi

    # 跨平台编译产物缓存目录（./bin/ota-cross 同级，避免污染正式 BIN）
    local cross_dir="${BIN_DIR}/ota-cross"
    mkdir -p "$cross_dir"

    mkdir -p "$PACKAGES_DIR"
    local package_path=""
    local -a produced=()
    local pf pf_args_=()
    for pf in "${ota_platforms[@]}"; do
        local src_bin
        src_bin="$(_ota_source_bin "$pf" "$edition_default" "$cross_dir")"
        local pf_args=()
        if [ -n "$pf" ]; then
            local pgoos="${pf%%-*}"
            local pgoarch="${pf#*-}"
            pf_args+=(--goos "$pgoos" --goarch "$pgoarch")
        fi
        pf_args+=(--edition "$edition_default")
        pf_args+=(--source-bin "$src_bin")
        package_path="$(
            bash "$package_script" service \
                --version "$VERSION" \
                --build-time "$BUILD_TIME" \
                --source-static "$BIN_DIR/static" \
                --out-dir "$PACKAGES_DIR" \
                "${pf_args[@]}"
        )"
        produced+=("$package_path")
    done
    rm -rf "$cross_dir"

    local package_name
    package_name="$(basename "$package_path" .tar.gz)"

    # 获取文件大小
    local package_size=$(du -h "$package_path" | cut -f1)

    print_success "Package created successfully!"
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}📦 Package Complete!${NC}"
    echo -e "${GREEN}========================================${NC}"
    for produced_path in "${produced[@]}"; do
        echo -e "${GREEN}Package: ${produced_path}${NC}"
        echo -e "${GREEN}Size:    $(du -h "$produced_path" | cut -f1)${NC}"
        if [ -f "${produced_path}.sha256" ]; then
            echo -e "${GREEN}Checksum: ${produced_path}.sha256${NC}"
        fi
        if [ -f "${produced_path}.manifest.json" ]; then
            echo -e "${GREEN}Manifest: ${produced_path}.manifest.json${NC}"
        fi
        echo ""
    done
    echo -e "${GREEN}Version: v${VERSION}${NC}"
    echo ""

    # 如果指定了 --upload，更新到容器
    if [ "$upload" = true ]; then
        echo ""
        print_info "=== 步骤 3/4: 热更新到容器 ==="
        
        # 检查 Docker 是否运行
        if ! docker info >/dev/null 2>&1; then
            print_warn "Docker 未运行或当前用户无权限，请手动更新"
            echo "  请使用 ./start.sh docker up --build 重建设置"
            return
        fi

        # 检查容器是否运行
        if ! docker ps --format '{{.Names}}' | grep -q "^centag$"; then
            print_warn "centag 容器未运行，将重新构建镜像"
            echo ""
            print_info "重新构建并启动容器..."
            cd "$PROJECT_ROOT/deploy/docker"
            if docker-compose up -d --build centag; then
                print_success "容器已重建并启动"
                echo ""
                print_info "查看日志: ./start.sh docker logs centag"
            else
                print_warn "重建容器失败，请手动检查"
                echo "  cd docker && docker-compose up -d --build centag"
            fi
            cd "$PROJECT_ROOT"
            return
        fi

        # 获取服务地址
        local service_url="http://localhost:20060"
        load_env
        local update_token="${CENTAG_UPDATE_TOKEN:-}"
        if [ -z "$update_token" ]; then
            for key_var in LLM_PROXY_DEFAULT_ADMIN_API_KEY LLM_PROXY_ADMIN_API_KEY CENTAG_API_KEY; do
                local candidate="${!key_var:-}"
                if [ -n "$candidate" ]; then
                    update_token="$candidate"
                    break
                fi
            done
        fi

        local auth_header=""
        if [ -n "$update_token" ]; then
            if [[ "$update_token" =~ ^[Bb]earer[[:space:]]+ ]]; then
                auth_header="$update_token"
            else
                auth_header="Bearer $update_token"
            fi
            print_info "检测到更新认证令牌，上传将附带 Authorization 头"
        else
            print_warn "未检测到更新认证令牌（CENTAG_UPDATE_TOKEN / LLM_PROXY_DEFAULT_ADMIN_API_KEY / LLM_PROXY_ADMIN_API_KEY）"
            print_warn "若服务开启鉴权，上传可能返回 401/403"
            print_error "当前未配置可用认证令牌，已跳过上传。请先设置 CENTAG_UPDATE_TOKEN 后重试。"
            return
        fi

        echo ""
        print_info "步骤 3.1/3: 通过 API 接口执行热更新..."
        
        # 检查服务是否可访问
        if ! curl -sf "${service_url}/health" >/dev/null 2>&1; then
            print_error "无法连接到 centag 服务"
            print_info "请检查服务是否正常运行"
            echo "  ./start.sh docker logs centag"
            return
        fi
        print_success "服务健康检查通过"

        # 上传前先做鉴权/权限预检，避免上传大文件后才报 401/403
        print_info "步骤 3.1.1/3: 认证预检..."
        local auth_probe_file
        auth_probe_file="$(mktemp)"
        local auth_probe_code
        if ! auth_probe_code="$(
            curl -sS -o "$auth_probe_file" -w "%{http_code}" \
              -H "Authorization: ${auth_header}" \
              "${service_url}/api/v1/system/update/history" 2>&1
        )"; then
            print_error "认证预检失败（网络或连接错误）"
            echo -e "${RED}curl 错误: ${auth_probe_code}${NC}"
            rm -f "$auth_probe_file"
            return
        fi
        local auth_probe_body
        auth_probe_body="$(cat "$auth_probe_file")"
        rm -f "$auth_probe_file"
        if [[ ! "$auth_probe_code" =~ ^2[0-9][0-9]$ ]]; then
            print_error "认证预检未通过: HTTP ${auth_probe_code}"
            echo -e "${RED}响应体: ${auth_probe_body}${NC}"
            if [ "$auth_probe_code" = "401" ] || [ "$auth_probe_code" = "403" ]; then
                print_warn "请确认 Authorization 使用 Bearer llmproxy_* 或有效 JWT"
                print_warn "可显式设置: CENTAG_UPDATE_TOKEN=llmproxy_xxx ./start.sh pack --upload"
            fi
            return
        fi
        print_success "认证预检通过"

        echo ""
        print_info "步骤 3.2/3: 上传更新包..."

        # 调用热更新 API（保留完整响应，便于诊断）
        local api_response_file
        api_response_file="$(mktemp)"
        local http_code
        local curl_args=(
            -sS
            -o "$api_response_file"
            -w "%{http_code}"
            -X POST "${service_url}/api/v1/system/update"
            -F "package=@${package_path}"
        )
        if [ -n "$auth_header" ]; then
            curl_args+=(-H "Authorization: ${auth_header}")
        fi

        if ! http_code="$(curl "${curl_args[@]}" 2>&1)"; then
            print_error "上传更新包失败（网络或连接错误）"
            echo -e "${RED}curl 错误: ${http_code}${NC}"
            rm -f "$api_response_file"
            return
        fi

        local api_response
        api_response="$(cat "$api_response_file")"
        rm -f "$api_response_file"

        if [[ ! "$http_code" =~ ^2[0-9][0-9]$ ]]; then
            print_error "更新接口返回非成功状态码: HTTP ${http_code}"
            echo -e "${RED}响应体: ${api_response}${NC}"
            if [ "$http_code" = "401" ] || [ "$http_code" = "403" ]; then
                print_warn "请确认 Authorization 是否使用 Bearer llmproxy_* 或有效 JWT"
                print_warn "可显式设置: CENTAG_UPDATE_TOKEN=llmproxy_xxx ./start.sh pack --upload"
            fi
            return
        fi

        if [[ "$api_response" =~ \"success\"[[:space:]]*:[[:space:]]*true ]]; then
            print_success "更新包已上传并开始处理"
        else
            print_warn "接口返回 2xx，但 success 字段不是 true，请关注响应详情"
            echo -e "${YELLOW}响应体: ${api_response}${NC}"
        fi

        echo ""
        print_info "步骤 3.3/3: 等待服务热更新完成..."
        print_info "守护进程正在处理热更新，请稍候..."

        # 等待服务重启（最多等待 30 秒）
        local max_wait=30
        local waited=0
        local service_ready=0
        
        while [ $waited -lt $max_wait ]; do
            if curl -sf "${service_url}/health" >/dev/null 2>&1; then
                service_ready=1
                break
            fi
            sleep 1
            waited=$((waited + 1))
            if [ $((waited % 5)) -eq 0 ]; then
                print_info "等待服务重启... ($waited/${max_wait}秒)"
            fi
        done

        if [ $service_ready -eq 1 ]; then
            echo ""
            print_success "热更新完成，服务已重启"
            echo ""
            echo -e "${GREEN}========================================${NC}"
            echo -e "${GREEN}✅ 热更新成功!${NC}"
            echo -e "${GREEN}========================================${NC}"
            echo ""
            print_info "查看日志: ./start.sh docker logs centag"
            print_info "查看更新历史: ./start.sh docker exec centag cat /app/storage/update-history/*.json"
        else
            print_warn "服务重启超时，请查看日志确认状态"
            echo "  ./start.sh docker logs centag"
        fi
    fi
}

# load_env — 加载环境变量
# 加载顺序：
#   1. deploy/stack/.env（提供 PG/Mem0 等中间件配置）
#   2. config/secrets/.env（本地配置，优先级更高，覆盖 stack 配置）
load_env() {
    local stack_env="$PROJECT_ROOT/deploy/stack/.env"
    local env_file="$PROJECT_ROOT/config/secrets/.env"
    local middleware_file="$PROJECT_ROOT/config/secrets/.env.middleware"

    # Step 1: 优先从 deploy/stack/.env 读取中间件配置
    if [ -f "$stack_env" ]; then
        print_info "加载 deploy/stack 环境变量..."
        set -a
        # shellcheck source=/dev/null
        source "$stack_env"
        set +a
        # 同步关键 PG 配置到标准变量名
        if [ -n "${POSTGRES_HOST:-}" ]; then
            export PG_HOST="$POSTGRES_HOST"
        fi
        if [ -n "${POSTGRES_PORT:-}" ]; then
            export PG_PORT="$POSTGRES_PORT"
        fi
        if [ -n "${POSTGRES_USER:-}" ]; then
            export PG_USER="$POSTGRES_USER"
        fi
        if [ -n "${POSTGRES_PASSWORD:-}" ]; then
            export PG_PASSWORD="$POSTGRES_PASSWORD"
        fi
        if [ -n "${POSTGRES_DB:-}" ]; then
            export PG_DATABASE="$POSTGRES_DB"
        fi
        print_success "已从 deploy/stack/.env 同步 PostgreSQL 配置"
    fi

    # Step 2: 加载本地 config/secrets/.env（优先级更高，覆盖 stack 配置）
    if [ -f "$env_file" ]; then
        print_info "加载 config/secrets/.env（本地配置，优先级更高）..."
        set -a
        # shellcheck source=/dev/null
        source "$env_file"
        set +a
        print_success "环境变量已加载 (config/secrets/.env)"
    elif [ -f "$middleware_file" ]; then
        # Step 3: 兼容旧的 config/secrets/.env.middleware
        print_warn "未找到 config/secrets/.env，回退加载 config/secrets/.env.middleware"
        set -a
        # shellcheck source=/dev/null
        source "$middleware_file"
        set +a
        print_success "环境变量已加载 (config/secrets/.env.middleware)"
    else
        print_warn "未找到 config/secrets/.env，将使用程序内置默认值启动"
        print_info "提示：执行 ./start.sh generate-secrets 可生成密钥配置文件"
    fi
}

# stack — 独立进程加载 deploy/stack/lib（避免与主仓 readonly PROJECT_ROOT 冲突）
stack_cmd() {
    local stack_dir="$PROJECT_ROOT/deploy/stack"
    local stack_lib="$stack_dir/lib/stack.sh"

    if [ ! -f "$stack_lib" ]; then
        print_error "未找到 deploy/stack/lib/stack.sh，请执行: git submodule update --init"
        exit 1
    fi

    if [ $# -eq 0 ]; then
        exec env STACK_ROOT="$stack_dir" STACK_INVOKER="./start.sh stack" STACK_QUIET_CD=1 \
            bash -c 'source "${STACK_ROOT}/lib/stack.sh" && stack_main help'
    else
        exec env STACK_ROOT="$stack_dir" STACK_INVOKER="./start.sh stack" STACK_QUIET_CD=1 \
            bash -c 'source "${STACK_ROOT}/lib/stack.sh" && stack_main "$@"' bash "$@"
    fi
}

# Run
run() {
    load_env
    resolve_backend_port || return 1
    [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build
    print_test_examples
    cd "$BIN_DIR"
    print_info "Starting backend service from: $BIN_DIR (port: $BACKEND_PORT)..."
    ./"$SERVER_BIN"
    cd "$PROJECT_ROOT"
}

# 前台调试：覆盖 secrets 里常见的 LLM_PROXY_LOG_OUTPUT=file（否则 zap 只写文件，终端看不到访问日志）
# 插件 init 使用标准库 log，仍走 stderr，故此前会出现「只有 Plugin initialized、无 request 日志」
# pprof：debug 默认开启 loopback :6060；显式 CENTAG_PPROF=false / LLM_PROXY_PPROF_ENABLED=false 可关闭
centag_export_debug_console_env() {
    export LLM_PROXY_SERVER_MODE=debug
    export LLM_PROXY_LOG_LEVEL=debug
    export LLM_PROXY_LOG_FORMAT=console
    export LLM_PROXY_LOG_OUTPUT=both
    if [ -z "${CENTAG_PPROF:-}" ] && [ -z "${LLM_PROXY_PPROF_ENABLED:-}" ]; then
        export CENTAG_PPROF=true
    fi
}

# 数据库模式检测（支持 PostgreSQL 和 SQLite）
detect_database_mode() {
    local db_driver="${LLM_PROXY_DB_DRIVER:-sqlite}"

    # 如果设置为 auto，根据环境变量自动检测
    if [ "$db_driver" = "auto" ]; then
        if [ -n "${PG_HOST:-}" ] || [ -n "${POSTGRES_HOST:-}" ]; then
            db_driver="postgresql"
        else
            db_driver="sqlite"
        fi
    fi

    export LLM_PROXY_DB_DRIVER="$db_driver"

    if [ "$db_driver" = "postgresql" ]; then
        local pg_host="${PG_HOST:-${POSTGRES_HOST:-localhost}}"
        local pg_port="${PG_PORT:-${POSTGRES_PORT:-5432}}"

        print_info "数据库模式：PostgreSQL"
        print_info "  连接：${pg_host}:${pg_port}/${PG_DATABASE:-${POSTGRES_DB:-centag}}"

        # 检测 PG 是否可达（TCP 端口检测）
        if ! command -v nc >/dev/null 2>&1 && ! command -v timeout >/dev/null 2>&1; then
            print_warn "未安装 nc/timeout，跳过端口可达性检测"
        else
            local pg_reachable=false
            if command -v nc >/dev/null 2>&1; then
                if nc -z -w 3 "$pg_host" "$pg_port" 2>/dev/null; then
                    pg_reachable=true
                fi
            elif command -v timeout >/dev/null 2>&1; then
                if timeout 3 bash -c "echo > /dev/tcp/${pg_host}/${pg_port}" 2>/dev/null; then
                    pg_reachable=true
                fi
            fi

            if [ "$pg_reachable" = false ]; then
                echo ""
                print_error "PostgreSQL 不可达: ${pg_host}:${pg_port}"
                echo ""
                print_info "解决方案："
                echo "  1. 启动中间件: ./start.sh stack start base"
                echo "  2. 或确认 PG_HOST 配置正确（检查 config/secrets/.env）"
                echo "  3. 或设置 LLM_PROXY_DB_DRIVER=sqlite 使用 SQLite 数据库"
                echo ""
                exit 1
            fi
        fi
    elif [ "$db_driver" = "sqlite" ]; then
        local sqlite_path="${SQLITE_PATH:-$BIN_DIR/storage/centag.db}"
        print_info "数据库模式：SQLite"
        print_info "  路径：${sqlite_path}"
        
        # 确保 SQLite 数据库目录存在
        local sqlite_dir
        sqlite_dir=$(dirname "$sqlite_path")
        if [ ! -d "$sqlite_dir" ]; then
            mkdir -p "$sqlite_dir"
            print_info "已创建 SQLite 数据库目录：${sqlite_dir}"
        fi
    else
        print_error "不支持的数据库驱动: $db_driver"
        print_info "支持的驱动：postgresql, sqlite"
        exit 1
    fi
}

# Debug - 开发模式：先构建后端（可选 desktop 外壳）+ Vite watch + debug 日志启动
# WSL2 环境下 5173 端口不可达，改用 vite build --watch 直接输出到 bin/static/
# 前端文件变化后 Vite 自动重建，刷新浏览器 (localhost:20060) 即可看到最新内容
#
# 用法（与 build/run 风格一致）:
#   ./start.sh debug                      # 默认 personal CLI
#   ./start.sh debug minimal              # 精简 WebUI + centag-minimal
#   ./start.sh debug personal             # 显式 personal CLI
#   ./start.sh debug personal --desktop   # 构建 sidecar+desktop，以 debug 模式开托盘
#   Team：cd ../centag-pro && ./start.sh debug team
debug() {
    local edition="${1:-personal}"
    local with_desktop="${2:-false}"
    local with_docker="${3:-false}"

    # ── Docker 调试模式（前台容器）────────────────────────────
    if $with_docker; then
        _debug_docker "$edition"
        return
    fi

    # ── minimal 分支：精简 WebUI + centag-minimal ─────────────────
    if [ "$edition" = "minimal" ]; then
        _debug_minimal "$with_desktop"
        return
    fi

    # ── team：开源仓不再构建/转调；请到 centag-pro ────────────────
    if [ "$edition" = "team" ]; then
        if $with_desktop; then
            print_error "--desktop 不支持 team"
            exit 1
        fi
        _reject_team_build_in_oss debug
    fi

    # ── personal：开源全功能二进制 + webui 前端 ───────────────────

    load_env

    # 自动检测数据库模式
    detect_database_mode

    # 强制 debug 模式 + 控制台输出格式，便于开发时直接查看日志
    # 开发模式下同时写文件与 stdout，避免仅 file 时启动失败在终端无输出
    centag_export_debug_console_env

    # 覆盖 secrets 中的 edition；对齐 ~/.centag/lib/personal
    centag_set_edition personal
    export INITDATA_PATH="${PROJECT_ROOT}/config/initdata"

    # ── 清理所有残留进程（保证前台独占）──────────────────────────
    cleanup_residual_processes

    rm -f "$BIN_DIR/storage/centag.pid" 2>/dev/null || true

    resolve_backend_port || return 1

    check_go
    print_info "编译后端 (edition=personal)..."
    build backend
    if $with_desktop; then
        print_info "编译 desktop 外壳..."
        build_desktop_shell
    fi

    # 检查前端依赖
    check_node
    local webui_dir="${PROJECT_ROOT}/web"
    if [ ! -d "$webui_dir/node_modules" ] || [ "$webui_dir/package.json" -nt "$webui_dir/node_modules/.package-lock.json" ]; then
        print_info "安装 Web UI 依赖..."
        cd "$webui_dir" && npm install && cd "$PROJECT_ROOT"
    fi

    # 确保 lib/<edition>/static 目录存在
    mkdir -p "$BIN_DIR/static"

    # 后台启动 Vite watch：监听 web/src，构建到 lib/<edition>/static
    print_info "启动前端 watch 构建（变化后刷新浏览器即可生效）..."
    cd "$webui_dir"
    CENTAG_STATIC_DIR="$BIN_DIR/static" npx vite build --watch --outDir "$BIN_DIR/static" --emptyOutDir false > /tmp/centag-vite.log 2>&1 &
    local vite_pid=$!
    cd "$PROJECT_ROOT"

    # 等待首次构建完成
    print_info "等待首次构建完成..."
    local waited=0
    while [ $waited -lt 30 ]; do
        if grep -q "built in" /tmp/centag-vite.log 2>/dev/null; then
            break
        fi
        sleep 1
        waited=$((waited + 1))
    done

    if ! kill -0 "$vite_pid" 2>/dev/null; then
        print_error "前端构建失败，请查看日志: /tmp/centag-vite.log"
        cat /tmp/centag-vite.log
        exit 1
    fi
    print_success "前端构建就绪，Vite 监听文件变化中..."

    # 退出时自动清理 Vite（PID 写入全局，避免 EXIT 在函数返回后遇 set -u unbound）
    DEBUG_VITE_PID="$vite_pid"
    cleanup_debug_vite() {
        trap - EXIT INT TERM
        if [ -n "${DEBUG_VITE_PID:-}" ]; then
            kill "$DEBUG_VITE_PID" 2>/dev/null || true
            DEBUG_VITE_PID=
        fi
        print_info '已停止前端 watch 进程'
    }
    trap cleanup_debug_vite EXIT INT TERM

    echo ""
    print_info "════════════════════════════════════════"
    print_info "  开发模式已启动"
    print_info "  产品版本:    personal"
    if $with_desktop; then
        print_info "  形态:        desktop（托盘 + sidecar）"
    else
        print_info "  形态:        cli（前台 sidecar）"
    fi
    print_info "  访问地址:    http://localhost:$BACKEND_PORT"
    if [ "${CENTAG_PPROF:-}" = "true" ] || [ "${LLM_PROXY_PPROF_ENABLED:-}" = "true" ] || [ "${LLM_PROXY_PPROF_ENABLED:-}" = "1" ]; then
        print_info "  pprof:       http://127.0.0.1:6060/debug/pprof/（debug 默认开；CENTAG_PPROF=false 可关）"
    fi
    print_info "  前端变化后:  刷新浏览器即可看到最新内容"
    print_info "  后端变化后:  下次执行 debug 会先自动编译；也可 ./start.sh build be 单独编译"
    print_info "  按 Ctrl+C 停止所有服务"
    print_info "════════════════════════════════════════"
    echo ""

    if $with_desktop; then
        _debug_run_desktop personal "$SERVER_BIN"
    else
        # 前台启动后端（日志直接输出到控制台）
        cd "$BIN_DIR"
        CENTAG_EDITION=personal ./"$SERVER_BIN"
        cd "$PROJECT_ROOT"
    fi
}

# debug --desktop：前台跑托盘外壳（勿 exec，以便 EXIT trap 能停掉 vite）
_debug_run_desktop() {
    local edition="$1"
    local sidecar_name="$2"
    local sidecar="${BIN_DIR}/${sidecar_name}"
    local desktop_bin

    if [ ! -x "$sidecar" ] && [ ! -f "$sidecar" ]; then
        print_error "sidecar 不存在: $sidecar"
        exit 1
    fi
    if ! desktop_bin="$(resolve_desktop_bin)"; then
        print_info "未找到 desktop 外壳，先构建..."
        build_desktop_shell
        desktop_bin="$(resolve_desktop_bin)" || {
            print_error "desktop 外壳构建后仍未找到"
            exit 1
        }
    fi

    if [ -z "${LLM_PROXY_ADMIN_PASSWORD:-}" ]; then
        print_warn "未检测到 LLM_PROXY_ADMIN_PASSWORD；首轮启动将通过初始化向导设置管理员密码"
    fi

    print_info "启动 desktop (debug) edition=${edition}"
    print_info "  desktop: ${desktop_bin}"
    print_info "  sidecar: ${sidecar}"
    print_info "  日志:    debug → 控制台 + sidecar 日志文件（launcher 用户数据目录）"
    # 继承 centag_export_debug_console_env；launcher 不再覆盖 LOG_OUTPUT/FORMAT
    "$desktop_bin" -edition="$edition" -bin="$sidecar"
}

# ./start.sh run <personal|minimal> [--desktop]
# Without flags: CLI (foreground sidecar).
# --desktop: tray/menu shell + sidecar (product desktop form).
run_edition() {
    local edition="$1"
    shift || true
    local with_desktop=false
    local extra_args=()
    for arg in "$@"; do
        case "$arg" in
            --desktop)
                with_desktop=true
                ;;
            --) ;;
            *)
                extra_args+=("$arg")
                ;;
        esac
    done

    case "$edition" in
        personal|minimal) ;;
        team)
            if $with_desktop; then
                print_error "--desktop 不支持 team"
                exit 1
            fi
            ;;
        *)
            print_error "未知发行版: $edition"
            exit 1
            ;;
    esac

    local dist_name sidecar_name run_edition
    dist_name="$(edition_to_dist "$edition")"
    sidecar_name="$(edition_to_sidecar "$edition")"
    run_edition="$edition"

    local sidecar="${BIN_DIR}/${sidecar_name}"
    if [ ! -x "$sidecar" ] && [ ! -f "$sidecar" ]; then
        print_info "未找到 ${sidecar_name}，先构建 ${dist_name}..."
        build_distribution "$dist_name"
    fi

    load_env
    export INITDATA_PATH="${PROJECT_ROOT}/config/initdata"

    if ! $with_desktop; then
        print_info "启动 ${run_edition} CLI（daemon）: ${sidecar}"
        print_info "前台调试请用: ./start.sh debug ${run_edition}"
        export CENTAG_EDITION="${run_edition}"
        "${PROJECT_ROOT}/scripts/tools/daemon.sh" "$BIN_DIR"
        return $?
    fi

    if [ ! -d "${BIN_DIR}/static" ] || [ ! -f "${BIN_DIR}/static/index.html" ]; then
        print_info "构建前端静态资源 → ${BIN_DIR}/static ..."
        build_frontend_prod
    fi

    local desktop_bin
    if ! desktop_bin="$(resolve_desktop_bin)"; then
        print_info "未找到 desktop 外壳，先构建..."
        build_desktop_shell
        desktop_bin="$(resolve_desktop_bin)" || {
            print_error "desktop 外壳构建后仍未找到"
            exit 1
        }
    fi

    if [ -z "${LLM_PROXY_ADMIN_PASSWORD:-}" ]; then
        print_warn "未检测到 LLM_PROXY_ADMIN_PASSWORD；首轮启动将通过初始化向导设置管理员密码"
    else
        print_info "已加载管理员口令环境变量（来自 config/secrets/.env）"
    fi

    print_info "启动 desktop edition=${run_edition} platform=$(go env GOOS)/$(go env GOARCH)"
    print_info "  desktop: ${desktop_bin}"
    print_info "  sidecar: ${sidecar}"
    print_info "  data: 用户数据目录（与 lib/<edition> 开发库分离；见 apps/launcher/README）"
    # macOS bash 3.2 + set -u: empty "${arr[@]}" is "unbound variable"
    if [ ${#extra_args[@]} -gt 0 ]; then
        exec "$desktop_bin" -edition="$run_edition" -bin="$sidecar" "${extra_args[@]}"
    else
        exec "$desktop_bin" -edition="$run_edition" -bin="$sidecar"
    fi
}

# ── Minimal 调试：精简 WebUI（vite build）+ centag-minimal 后端 ─────────────
# $1: with_desktop (true|false)
_debug_minimal() {
    local with_desktop="${1:-false}"

    load_env
    detect_database_mode
    centag_export_debug_console_env
    centag_set_edition minimal
    export INITDATA_PATH="${PROJECT_ROOT}/config/initdata"

    cleanup_residual_processes
    rm -f "$BIN_DIR/storage/centag.pid" 2>/dev/null || true
    resolve_backend_port || return 1

    check_go
    print_info "编译 minimal 发行版后端..."
    build_distribution "minimal"
    if [ "$with_desktop" = "true" ]; then
        print_info "编译 desktop 外壳..."
        build_desktop_shell
    fi

    check_node
    local webui_dir="${PROJECT_ROOT}/web"
    if [ ! -d "$webui_dir/node_modules" ] || [ "$webui_dir/package.json" -nt "$webui_dir/node_modules/.package-lock.json" ]; then
        print_info "安装 Web UI 依赖..."
        cd "$webui_dir" && npm install && cd "$PROJECT_ROOT"
    fi

    mkdir -p "$BIN_DIR/static"
    print_info "构建精简管理台前端 → $BIN_DIR/static ..."
    cd "$webui_dir"
    npx vite build --outDir "$BIN_DIR/static" --emptyOutDir true
    local build_rc=$?
    cd "$PROJECT_ROOT"
    if [ $build_rc -ne 0 ] || [ ! -f "$BIN_DIR/static/index.html" ]; then
        print_error "前端构建失败（需要 $BIN_DIR/static/index.html）"
        exit 1
    fi
    print_success "前端构建完成"

    # 可选：watch 热更新静态资源
    print_info "启动前端 watch 构建（变化后刷新浏览器即可）..."
    cd "$webui_dir"
    npx vite build --watch --outDir "$BIN_DIR/static" --emptyOutDir false > /tmp/centag-vite-minimal.log 2>&1 &
    local vite_pid=$!
    cd "$PROJECT_ROOT"
    DEBUG_VITE_PID="$vite_pid"
    cleanup_debug_vite_minimal() {
        trap - EXIT INT TERM
        if [ -n "${DEBUG_VITE_PID:-}" ]; then
            kill "$DEBUG_VITE_PID" 2>/dev/null || true
            DEBUG_VITE_PID=
        fi
        print_info '已停止前端 watch'
    }
    trap cleanup_debug_vite_minimal EXIT INT TERM

    echo ""
    print_info "════════════════════════════════════════"
    print_info "  minimal 精简模式已启动"
    if [ "$with_desktop" = "true" ]; then
        print_info "  形态:        desktop（托盘 + sidecar）"
    else
        print_info "  形态:        cli（前台 sidecar）"
    fi
    print_info "  前端:        WebUI (edition=minimal)"
    print_info "  访问地址:    http://localhost:$BACKEND_PORT/static/"
    if [ "${CENTAG_PPROF:-}" = "true" ] || [ "${LLM_PROXY_PPROF_ENABLED:-}" = "true" ] || [ "${LLM_PROXY_PPROF_ENABLED:-}" = "1" ]; then
        print_info "  pprof:       http://127.0.0.1:6060/debug/pprof/（debug 默认开；CENTAG_PPROF=false 可关）"
    fi
    print_info "  首次进入:    设置管理密码后登录"
    print_info "  页面:        概览 / 后端 / 策略 / 设置"
    print_info "  按 Ctrl+C 停止"
    print_info "════════════════════════════════════════"
    echo ""

    if [ "$with_desktop" = "true" ]; then
        _debug_run_desktop minimal centag-minimal
    else
        cd "$BIN_DIR"
        CENTAG_EDITION=minimal ./centag-minimal
        cd "$PROJECT_ROOT"
    fi
}

# ── Docker 调试模式：构建镜像并前台运行容器 ─────────────────────────
# $1: edition (personal|minimal)
_debug_docker() {
    local edition="$1"
    local dist_name
    dist_name="$(edition_to_dist "$edition")"

    load_env

    print_info "Docker 调试模式: ${edition}"
    echo ""

    # 确保 Docker 可用
    check_docker

    # 构建 Docker 镜像
    _dist_docker_build "$dist_name" "" ""

    # 移除旧容器（避免 docker run --name 冲突）
    if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^centag-${dist_name}$"; then
        print_info "移除旧容器 centag-${dist_name}..."
        docker rm -f "centag-${dist_name}" >/dev/null 2>&1 || true
        sleep 1
    fi

    # 自动寻找可用端口
    resolve_backend_port

    local tag="centag-${dist_name}:latest"
    local mitm_port="${LLM_PROXY_SYSTEM_PROXY_PORT:-8081}"

    # 确保数据卷存在
    docker volume inspect "centag-${dist_name}-storage" >/dev/null 2>&1 || docker volume create "centag-${dist_name}-storage"
    docker volume inspect "centag-${dist_name}-logs" >/dev/null 2>&1 || docker volume create "centag-${dist_name}-logs"
    docker volume inspect "centag-${dist_name}-certs" >/dev/null 2>&1 || docker volume create "centag-${dist_name}-certs"

    echo ""
    print_info "════════════════════════════════════════"
    print_info "  Docker 调试模式已启动"
    print_info "  产品版本:    ${edition}"
    print_info "  访问地址:    http://localhost:${BACKEND_PORT}"
    print_info "  按 Ctrl+C 停止容器"
    print_info "════════════════════════════════════════"
    echo ""

    exec docker run -it --rm \
        --name "centag-${dist_name}" \
        --env-file "${PROJECT_ROOT}/config/secrets/.env" \
        -e CENTAG_EDITION="${dist_name}" \
        -e CENTAG_IN_DOCKER=1 \
        -e CENTAG_DATA_DIR=/app/storage \
        -e LLM_PROXY_DB_DRIVER=sqlite \
        -e SQLITE_PATH=/app/storage/centag.db \
        -e MEMORY_STORE_ROOT=/app/storage/memory-store \
        -e LLM_PROXY_LOG_OUTPUT=both \
        -e LLM_PROXY_LOG_FORMAT=console \
        -e LLM_PROXY_LOG_PATH=/app/logs \
        -p "${BACKEND_PORT}:20060" \
        -p "${mitm_port}:8081" \
        -v "centag-${dist_name}-storage:/app/storage" \
        -v "centag-${dist_name}-logs:/app/logs" \
        -v "centag-${dist_name}-certs:/app/bin/certs" \
        "$tag"
}

# Daemon
daemon() {
    load_env
    detect_database_mode
    start_backend_background
}

# Daemon Debug
daemon-debug() {
    load_env
    detect_database_mode
    centag_export_debug_console_env
    resolve_backend_port || return 1
    [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1
    print_test_examples
    print_info "Starting with daemon in debug mode from: $BIN_DIR..."
    DAEMON_DEBUG=true "${PROJECT_ROOT}/scripts/tools/daemon.sh" "$BIN_DIR"
}


# Print Test Examples
print_test_examples() {
    echo ""
    print_info "========================================"
    print_info "测试脚本示例"
    print_info "========================================"
    echo ""
    print_info "1. 测试聊天请求（非流式）:"
    echo "   curl -X POST \"http://localhost:${BACKEND_PORT}/v1/chat/completions\" \\"
    echo "     -H \"Content-Type: application/json\" \\"
    echo "     -d '{\"model\": \"qwen2.5:1.5b\", \"messages\": [{\"role\": \"user\", \"content\": \"你好\"}], \"stream\": false}'"
    echo ""
    print_info "2. 测试聊天请求（流式）:"
    echo "   curl -X POST \"http://localhost:${BACKEND_PORT}/v1/chat/completions\" \\"
    echo "     -H \"Content-Type: application/json\" \\"
    echo "     -d '{\"model\": \"qwen2.5:1.5b\", \"messages\": [{\"role\": \"user\", \"content\": \"你好\"}], \"stream\": true}'"
    echo ""
    print_info "3. 测试模型列表:"
    echo "   curl http://localhost:${BACKEND_PORT}/v1/models"
    echo ""
    print_info "4. 查看监控数据:"
    echo "   curl http://localhost:${BACKEND_PORT}/api/v1/monitor/dashboard"
    echo ""
    print_info "5. 查看缓存统计:"
    echo "   curl http://localhost:${BACKEND_PORT}/api/v1/cache/stats"
    echo ""
    print_info "6. 运行端到端缓存测试:"
    echo "   bash ./test/test-e2e.sh"
    echo ""
    print_info "7. 访问 Web 管理界面:"
    echo "   http://localhost:${BACKEND_PORT}/"
    echo ""
}

# Daemon Stop
daemon-stop() {
    print_info "Stopping daemon..."
    local daemon_pid_file="$BIN_DIR/storage/centag.daemon.pid"
    if [ -f "$daemon_pid_file" ]; then
        local daemon_pid=$(cat "$daemon_pid_file" 2>/dev/null || true)
        if [ -n "$daemon_pid" ] && kill -0 "$daemon_pid" 2>/dev/null; then
            kill -TERM "$daemon_pid" 2>/dev/null || true
            sleep 2
            if kill -0 "$daemon_pid" 2>/dev/null; then
                kill -KILL "$daemon_pid" 2>/dev/null || true
            fi
            print_success "Daemon stopped"
        else
            print_warn "Daemon not running"
        fi
        rm -f "$daemon_pid_file"
    else
        print_warn "Daemon PID file not found"
    fi
}

# Daemon Status
_daemon_status() {
    local daemon_pid_file="$BIN_DIR/storage/centag.daemon.pid"
    if [ -f "$daemon_pid_file" ]; then
        local daemon_pid=$(cat "$daemon_pid_file" 2>/dev/null || true)
        if [ -n "$daemon_pid" ] && kill -0 "$daemon_pid" 2>/dev/null; then
            print_success "Daemon running (PID: $daemon_pid)"
        else
            print_warn "Daemon PID file exists but process not running"
        fi
    else
        print_warn "Daemon not running (no PID file)"
    fi
}

# Generate Secrets — 生成 config/secrets/.env（主服务密钥与 PG 元库）
generate_secrets() {
    print_info "Generating Centag secrets (config/secrets/.env)..."
    if [ -x "${PROJECT_ROOT}/scripts/generate-secrets.sh" ]; then
        "${PROJECT_ROOT}/scripts/generate-secrets.sh" "$@"
        return $?
    fi
    if [ -x "${PROJECT_ROOT}/scripts/ops/generate-secrets.sh" ]; then
        "${PROJECT_ROOT}/scripts/ops/generate-secrets.sh" "$@"
        return $?
    fi
    print_error "generate-secrets 脚本不存在: scripts/generate-secrets.sh 或 scripts/ops/generate-secrets.sh"
    return 1
}

# Init Secrets - 初始化并生成 config/secrets/.env
init_secrets() {
    print_info "Initializing config/secrets/.env..."
    
    # 仅以 config/secrets/.env 为「已配置」判断（与 load_env 主配置一致）
    if [ -f "$PROJECT_ROOT/config/secrets/.env" ]; then
        print_warn "config/secrets/.env 已存在"
        read -r -p "是否重新生成并覆盖? (y/N): " confirm
        if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
            print_info "跳过生成"
            return 0
        fi
    else
        print_info "未检测到 config/secrets/.env，将生成新密钥配置（默认直接生成，不询问）"
    fi
    
    if [ -x "${PROJECT_ROOT}/scripts/generate-secrets.sh" ]; then
        "${PROJECT_ROOT}/scripts/generate-secrets.sh" "$@"
        return $?
    fi
    if [ -x "${PROJECT_ROOT}/scripts/ops/generate-secrets.sh" ]; then
        "${PROJECT_ROOT}/scripts/ops/generate-secrets.sh" "$@"
        return $?
    fi
    print_error "generate-secrets 脚本不存在: scripts/generate-secrets.sh 或 scripts/ops/generate-secrets.sh"
    return 1
}

# Clean — 清理构建产物 / 已部署的安装布局（~/.centag）
# 用法:
#   ./start.sh clean                 # 清理当前 edition 的 lib/<edition>（构建产物）
#   ./start.sh clean build           # 同上
#   ./start.sh clean install [-y]    # 停止进程并删除 CENTAG_INSTALL_ROOT（默认 ~/.centag）
#   ./start.sh clean deploy [-y]     # 同 install（部署安装布局）
#   ./start.sh clean all [-y]        # build + install
clean() {
    local target="build"
    local assume_yes=0
    local arg
    for arg in "$@"; do
        case "$arg" in
            -y|--yes) assume_yes=1 ;;
            build|install|deploy|all|help|-h|--help) target="$arg" ;;
            *)
                print_error "未知 clean 目标: $arg"
                echo "用法: $0 clean [build|install|deploy|all] [-y|--yes]"
                return 1
                ;;
        esac
    done

    case "$target" in
        help|-h|--help)
            _help_clean
            return 0
            ;;
        build)
            _clean_build
            ;;
        install|deploy)
            _clean_install "$assume_yes"
            ;;
        all)
            _clean_build
            _clean_install "$assume_yes"
            ;;
    esac
}

_clean_build() {
    print_info "清理当前发行版构建产物: $BIN_DIR"
    if [ -d "$BIN_DIR" ]; then
        rm -rf "$BIN_DIR"
        print_success "已删除 $BIN_DIR"
    else
        print_warn "目录不存在，跳过: $BIN_DIR"
    fi
    # 同步清理 bin/ 下指向该 edition 的包装/链接（保留其它 edition）
    local wrapper_link="${CENTAG_BIN_DIR}/centag-${CENTAG_EDITION}"
    local wrapper_link_exe="${wrapper_link}.exe"
    rm -f "$wrapper_link" "$wrapper_link_exe" 2>/dev/null || true
    print_success "构建产物清理完成（需要时请重新 ./start.sh build）"
}

_clean_install() {
    local assume_yes="${1:-0}"
    local root="${CENTAG_INSTALL_ROOT:-}"
    if [ -z "$root" ]; then
        print_error "CENTAG_INSTALL_ROOT 未设置"
        return 1
    fi

    # 安全：禁止删到仓库根或明显危险路径
    local root_abs project_abs
    root_abs="$(cd "$root" 2>/dev/null && pwd)" || root_abs="$root"
    project_abs="$(cd "$PROJECT_ROOT" 2>/dev/null && pwd)" || project_abs="$PROJECT_ROOT"
    if [ "$root_abs" = "$project_abs" ] || [ "$root_abs" = "/" ] || [ "$root_abs" = "$HOME" ]; then
        print_error "拒绝删除危险路径: $root_abs"
        return 1
    fi
    case "$root_abs" in
        "$project_abs"/*)
            print_error "拒绝删除位于仓库内的路径: $root_abs"
            return 1
            ;;
    esac

    if [ ! -e "$root" ]; then
        print_warn "安装根目录不存在，无需清理: $root"
        return 0
    fi

    print_warn "将删除已部署/安装布局（含二进制、Web 静态、运行时 DB/日志、release 产物等）:"
    echo "  $root"
    print_info "不会触碰仓库内 config/secrets/.env"
    if [ "$assume_yes" != "1" ]; then
        if [ ! -t 0 ]; then
            print_error "非交互环境请加 -y/--yes 确认删除"
            return 1
        fi
        local ans=""
        read -r -p "确认删除以上目录？输入 yes 继续: " ans || true
        if [ "$ans" != "yes" ]; then
            print_warn "已取消"
            return 1
        fi
    fi

    print_info "先停止本机 centag 进程..."
    stop 2>/dev/null || true
    # wrap 若占用系统代理，尽量关闭（失败忽略）
    if [ -x "${CENTAG_BIN_DIR}/centag-wrap" ]; then
        "${CENTAG_BIN_DIR}/centag-wrap" disable 2>/dev/null || true
    fi

    print_info "删除 $root ..."
    rm -rf "$root"
    print_success "已清除部署文件: $root"
}

# Status
status() {
    print_info "Service status:"
    command -v lsof >/dev/null 2>&1 && {
        lsof -ti ":$BACKEND_PORT" >/dev/null && print_success "Backend running" || print_warn "Backend not running"
    }

    # 检查守护进程
    local daemon_pid_file="$BIN_DIR/storage/centag.daemon.pid"
    if [ -f "$daemon_pid_file" ]; then
        local daemon_pid=$(cat "$daemon_pid_file" 2>/dev/null || true)
        if [ -n "$daemon_pid" ] && kill -0 "$daemon_pid" 2>/dev/null; then
            print_success "Daemon running (PID: $daemon_pid)"
        else
            print_warn "Daemon PID file exists but not running"
        fi
    else
        print_warn "Daemon not running"
    fi
}

# Stop - 确保完全停止所有服务
stop() {
    print_info "Stopping all services..."

    # 1. 先尝试优雅停止守护进程
    daemon-stop

    # 2. 杀掉端口上的进程
    kill_port $BACKEND_PORT

    # 3. 确保所有 centag 二进制进程都被清理
    print_info "Cleaning up any remaining centag processes..."
    local remaining_pids=$(ps aux | grep -E ".centag/lib/.*/centag-|/centag-personal|/centag-minimal|/centag " | grep -v grep | grep -v "start.sh" | awk '{print $2}' || true)
    if [ -n "$remaining_pids" ]; then
        print_warn "Found remaining processes: $remaining_pids"
        for pid in $remaining_pids; do
            print_info "Killing process $pid..."
            kill -9 "$pid" 2>/dev/null || true
        done
        sleep 1
    fi

    # 4. 最终验证
    if command -v lsof >/dev/null 2>&1; then
        local port_check=$(lsof -ti ":$BACKEND_PORT" 2>/dev/null || true)
        if [ -n "$port_check" ]; then
            print_error "Warning: Port $BACKEND_PORT still in use by PID: $port_check"
        fi
    fi

    local process_check=$(ps aux | grep -E ".centag/lib/.*/centag-|/centag-personal|/centag-minimal|/centag " | grep -v grep | grep -v "start.sh" || true)
    if [ -n "$process_check" ]; then
        print_error "Warning: Some centag processes may still be running:"
        echo "$process_check"
    else
        print_success "All services stopped successfully"
    fi
}

# Stop Backend Only - 精确停止后端服务（不影响前端或其他 edition）
_stop_backend_only() {
    print_info "Stopping backend service..."
    daemon-stop 2>/dev/null || true
    kill_port "$BACKEND_PORT"
    rm -f "$BIN_DIR/storage/centag.pid" 2>/dev/null || true
    print_success "Backend stopped"
}

# Force Stop - 强制停止所有相关进程
force-stop() {
    print_warn "Force stopping all centag related processes..."

    # 1. 强制杀死所有 centag 二进制进程
    local all_pids=$(ps aux | grep -E ".centag/lib/.*/centag-|/centag-personal|/centag-minimal|/centag " | grep -v grep | grep -v "start.sh" | awk '{print $2}' || true)
    if [ -n "$all_pids" ]; then
        print_info "Found centag binary processes: $all_pids"
        for pid in $all_pids; do
            # 显示进程信息
            local proc_info=$(ps -p "$pid" -o comm= 2>/dev/null || true)
            print_info "Force killing PID $pid ($proc_info)..."
            kill -9 "$pid" 2>/dev/null || true
        done
    fi

    # 2. 清理守护进程 PID 文件
    local daemon_pid_file="$BIN_DIR/storage/centag.daemon.pid"
    local service_pid_file="$BIN_DIR/storage/centag.pid"
    rm -f "$daemon_pid_file" "$service_pid_file"

    # 3. 强制清理端口
    if command -v lsof >/dev/null 2>&1; then
        local port_pids=$(lsof -ti ":$BACKEND_PORT" 2>/dev/null || true)
        if [ -n "$port_pids" ]; then
            print_info "Force killing processes on port $BACKEND_PORT: $port_pids"
            for pid in $port_pids; do
                kill -9 "$pid" 2>/dev/null || true
            done
        fi
    fi

    sleep 2

    # 4. 验证
    local remaining=$(ps aux | grep -E ".centag/lib/.*/centag-|/centag-personal|/centag-minimal|/centag " | grep -v grep | grep -v "start.sh" || true)
    if [ -n "$remaining" ]; then
        print_error "Some processes still running:"
        echo "$remaining"
        return 1
    else
        print_success "All processes forcefully stopped"
    fi
}

# Restart
restart() {
    print_info "Restarting daemon..."
    stop
    sleep 1
    daemon
}

# Format
fmt() {
    print_info "Formatting code..."
    go fmt ./...
    print_success "Code formatted"
}

# ============================================
# Web UI 相关命令
# ============================================

load_nvm_if_present() {
    export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
    if [ -s "$NVM_DIR/nvm.sh" ]; then
        # shellcheck disable=SC1090
        . "$NVM_DIR/nvm.sh"
        return 0
    fi
    return 1
}

# WebUI（Vite 8）要求与 web/package.json engines 一致：^20.19.0 || >=22.12.0
node_meets_webui_requirement() {
    if ! command -v node >/dev/null 2>&1; then
        return 1
    fi
    local ver major minor patch
    ver=$(node -v | sed 's/^v//')
    major=$(echo "$ver" | cut -d. -f1)
    minor=$(echo "$ver" | cut -d. -f2)
    patch=$(echo "$ver" | cut -d. -f3 | cut -d- -f1)
    major=${major:-0}
    minor=${minor:-0}
    patch=${patch:-0}

    if [ "$major" -gt 22 ]; then
        return 0
    fi
    if [ "$major" -eq 22 ] && { [ "$minor" -gt 12 ] || { [ "$minor" -eq 12 ] && [ "$patch" -ge 0 ]; }; }; then
        return 0
    fi
    if [ "$major" -eq 20 ] && { [ "$minor" -gt 19 ] || { [ "$minor" -eq 19 ] && [ "$patch" -ge 0 ]; }; }; then
        return 0
    fi
    return 1
}

# 通过 nvm 切换到项目 .nvmrc 或已安装的兼容版本
nvm_use_compatible_node() {
    load_nvm_if_present || return 1

    local nvmrc_dir=""
    if [ -f "${PROJECT_ROOT}/web/.nvmrc" ]; then
        nvmrc_dir="${PROJECT_ROOT}/web"
    elif [ -f "${PROJECT_ROOT}/.nvmrc" ]; then
        nvmrc_dir="${PROJECT_ROOT}"
    fi

    if [ -n "$nvmrc_dir" ]; then
        local old_pwd="$PWD"
        cd "$nvmrc_dir" || return 1
        if nvm use --silent 2>/dev/null || nvm install --silent 2>/dev/null; then
            cd "$old_pwd" || return 1
            node_meets_webui_requirement && return 0
        fi
        cd "$old_pwd" || return 1
    fi

    local candidate
    for candidate in 22 24 20; do
        if nvm use --silent "$candidate" 2>/dev/null; then
            node_meets_webui_requirement && return 0
        fi
        if nvm install --silent "$candidate" 2>/dev/null; then
            nvm use --silent "$candidate" 2>/dev/null || true
            node_meets_webui_requirement && return 0
        fi
    done
    return 1
}

ensure_node_for_webui() {
    load_nvm_if_present || true

    if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1 && node_meets_webui_requirement; then
        return 0
    fi

    if command -v node >/dev/null 2>&1 && ! node_meets_webui_requirement; then
        print_warn "当前 Node.js（$(node -v)）不满足 WebUI 要求（^20.19.0 或 >=22.12.0），尝试通过 nvm 切换..."
        if nvm_use_compatible_node; then
            export CENTAG_NODE_SWITCHED=1
            print_success "已切换 Node.js 至 $(node -v)"
            return 0
        fi
    fi

    if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
        print_warn "未检测到 Node.js/npm，准备自动安装（nvm + Node.js）..."
        local installer="${PROJECT_ROOT}/scripts/ops/install-nodejs.sh"
        if [ ! -f "$installer" ]; then
            print_error "自动安装脚本不存在: $installer"
            print_warn "请先安装 Node.js (需要 ^20.19.0 或 >=22.12.0)，或补齐 scripts/ops/install-nodejs.sh"
            exit 1
        fi
        bash "$installer" --node-version 22 --with-pnpm || {
            print_error "Node.js 自动安装失败"
            exit 1
        }
        load_nvm_if_present || true
    fi

    if ! node_meets_webui_requirement; then
        if nvm_use_compatible_node; then
            export CENTAG_NODE_SWITCHED=1
        fi
    fi
}

check_node() {
    ensure_node_for_webui

    if ! command -v node >/dev/null 2>&1; then
        print_error "Node.js 未安装"
        print_warn "WebUI 需要 Node.js ^20.19.0 或 >=22.12.0（与 web/package.json 一致）"
        print_warn "可执行: nvm install 22 && nvm use 22 && nvm alias default 22"
        exit 1
    fi

    if ! command -v npm >/dev/null 2>&1; then
        print_error "npm 未安装"
        exit 1
    fi

    if ! node_meets_webui_requirement; then
        print_error "Node.js 版本不满足 WebUI 要求: 当前 $(node -v)，需要 ^20.19.0 或 >=22.12.0"
        print_warn "你已通过 nvm 安装更高版本时，可执行:"
        print_warn "  cd ${PROJECT_ROOT} && nvm use"
        print_warn "  nvm alias default 22   # 可选：将默认版本改为 22"
        exit 1
    fi

    print_success "Node.js 环境检查通过 ($(node -v))"
}

# Web UI 开发模式
webui_dev() {
    print_info "启动 Web UI 开发服务器..."
    ensure_node_for_webui
    check_node

    local webui_dir="${PROJECT_ROOT}/web"

    if [ ! -d "$webui_dir" ]; then
        print_error "webui 目录不存在: $webui_dir"
        exit 1
    fi

    cd "$webui_dir"

    # 检查依赖
    if [ ! -d "node_modules" ]; then
        print_info "安装 Web UI 依赖..."
        npm install
        if [ $? -ne 0 ]; then
            print_error "依赖安装失败"
            exit 1
        fi
        print_success "依赖安装完成"
    fi

    # 清理可能占用 5173-5180 端口的 Vite/Node 进程
    local webui_port=5173
    print_info "检查端口 $webui_port..."

    # 在 WSL 环境下，Vite 可能会错误地报告端口占用
    # 我们需要查找所有可能的 vite/node 进程并清理
    local node_pids=$(ps aux | grep -E "vite|node.*webui|npm.*dev" | grep -v grep | awk '{print $2}' || true)

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
        # 强制清理还在运行的进程
        for pid in $node_pids; do
            if kill -0 "$pid" 2>/dev/null; then
                print_warn "强制停止进程 $pid..."
                kill -9 "$pid" 2>/dev/null || true
            fi
        done
        sleep 1
    fi

    # 清理可能占用 5173 端口的进程
    local port_pids=""
    if command -v lsof >/dev/null 2>&1; then
        port_pids=$(lsof -ti ":$webui_port" 2>/dev/null || true)
    fi

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
        # 强制清理还在运行的进程
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

    print_info "启动开发服务器 (http://localhost:5173)..."

    # 在 WSL 环境下，Vite 的端口检测可能有问题
    # 我们需要绕过 Vite 的端口检测机制
    # 使用 VITE_PORT 环境变量指定端口
    VITE_PORT=5173 npm run dev
}

# Web UI 构建
webui_build() {
    print_info "构建 Web UI..."
    ensure_node_for_webui
    check_node

    local webui_dir="${PROJECT_ROOT}/web"

    if [ ! -d "$webui_dir" ]; then
        print_error "webui 目录不存在: $webui_dir"
        exit 1
    fi

    cd "$webui_dir"

    # Node 版本变更后必须重装 native 依赖（rolldown/vite 等）
    local node_tag_file="node_modules/.installed-node-version"
    local current_node
    current_node=$(node -v)
    if [ "${CENTAG_NODE_SWITCHED:-}" = "1" ] \
        || [ ! -d "node_modules" ] \
        || [ ! -f "$node_tag_file" ] \
        || [ "$(cat "$node_tag_file" 2>/dev/null)" != "$current_node" ]; then
        print_info "安装 Web UI 依赖（Node ${current_node}）..."
        rm -rf node_modules
        if [ -f "package-lock.json" ]; then
            npm ci
        else
            npm install
        fi
        if [ $? -ne 0 ]; then
            print_error "依赖安装失败"
            exit 1
        fi
        mkdir -p node_modules
        echo "$current_node" > "$node_tag_file"
        print_success "依赖安装完成"
    fi

    print_info "开始构建..."
    export CENTAG_STATIC_DIR="${STATIC_DIR}"
    export CENTAG_INSTALL_ROOT CENTAG_EDITION
    npm run build

    if [ $? -eq 0 ]; then
        print_success "Web UI 构建完成!"
        print_info "构建产物位置: ${STATIC_DIR}"
        cd "$PROJECT_ROOT"

        # 静态文件已直接构建到 lib/<edition>/static，无需同步
        print_success "静态文件位置: ${STATIC_DIR}/"
    else
        print_error "Web UI 构建失败"
        cd "$PROJECT_ROOT"
        exit 1
    fi
}

# Web UI 代码检查
webui_lint() {
    print_info "检查 Web UI 代码..."
    ensure_node_for_webui
    check_node

    local webui_dir="${PROJECT_ROOT}/web"

    if [ ! -d "$webui_dir" ]; then
        print_error "webui 目录不存在: $webui_dir"
        exit 1
    fi

    cd "$webui_dir"

    if [ ! -d "node_modules" ]; then
        print_info "安装 Web UI 依赖..."
        npm install
    fi

    npm run lint
    cd "$PROJECT_ROOT"
}

webui_clean() {
    print_info "清理 Web UI 构建产物..."
    local static_dir="${STATIC_DIR}"
    rm -rf "$static_dir"
    print_success "清理完成"
}

# Test - 运行测试
test() {
    print_info "Running tests..."

    # 检查 Go 环境
    if ! command -v go >/dev/null 2>&1; then
        print_error "Go not installed"
        return 1
    fi

    # 运行 Go 单元测试
    cd "$PROJECT_ROOT"
    if go test ./... -v -timeout=30s; then
        print_success "All tests passed"
    else
        print_error "Some tests failed"
        return 1
    fi
}

# ============================================
# 服务启动抽象函数 - 统一管理后端/前端启动逻辑
# ============================================

# 启动后端服务（前台）
start_backend_foreground() {
    load_env
    resolve_backend_port || return 1
    [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1
    print_test_examples
    cd "$BIN_DIR"
    print_info "Starting backend from: $BIN_DIR (port: $BACKEND_PORT)..."
    ./"$SERVER_BIN"
    cd "$PROJECT_ROOT"
}

# 启动后端服务（后台/守护进程）
start_backend_background() {
    load_env
    resolve_backend_port || return 1
    [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1
    print_test_examples
    print_info "Starting daemon from: $BIN_DIR..."
    "${PROJECT_ROOT}/scripts/tools/daemon.sh" "$BIN_DIR"
}

# 启动前端开发服务器
start_frontend_dev() {
    webui_dev
}

# 启动全部开发服务（后台后端 + 前台前端）
_run_all_dev() {
    load_env
    detect_database_mode
    centag_export_debug_console_env

    resolve_backend_port || return 1
    [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1

    print_info "Starting backend in background (port: $BACKEND_PORT)..."
    cd "$BIN_DIR"
    nohup ./"$SERVER_BIN" > logs/centag.log 2>&1 &
    local be_pid=$!
    cd "$PROJECT_ROOT"

    sleep 2
    if kill -0 "$be_pid" 2>/dev/null; then
        print_success "Backend started (PID: $be_pid)"
    else
        print_error "Backend failed to start, check ${BIN_DIR}/logs/centag.log"
        return 1
    fi

    print_info "Starting frontend dev server..."
    webui_dev
}

# 构建生产版本前端
build_frontend_prod() {
    webui_build
}

# 显示 all 模式说明（因为需要两个终端）
show_all_mode_info() {
    print_warn "⚠️  'all' 模式需要两个终端窗口同时运行："
    echo ""
    print_info "终端 1 - 后端服务："
    echo "  ./start.sh run backend"
    echo ""
    print_info "终端 2 - 前端开发服务器："
    echo "  ./start.sh run frontend"
    echo ""
    print_info "或者使用生产版本前端（集成在后端）："
    echo "  ./start.sh run backend"
    echo ""
    print_info "生产版本前端访问: http://localhost:$BACKEND_PORT"
    print_info "开发服务器访问: http://localhost:5173"
    echo ""
}

# ============================================
# Docker 相关命令
# ============================================

# 检查 Docker
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装"
        print_warn "请访问 https://docs.docker.com/get-docker/ 安装Docker"
        exit 1
    fi

    if ! docker info >/dev/null 2>&1; then
        print_error "Docker 未运行"
        print_warn "请启动Docker服务"
        exit 1
    fi

    print_success "Docker 环境检查通过"
}

# Docker 构建镜像
# ── 统一发行版 Docker 构建 ──────────────────────────────────────────
# 参数: dist_name tag initdata_path
#   dist_name   — minimal|personal|team
#   tag         — 镜像标签（可选，默认 centag-<dist_name>:latest）
#   initdata_path — 自定义 initdata.zip 路径（可选）
_dist_docker_build() {
    local dist_name="${1:-minimal}"
    local tag="${2:-}"
    local initdata_path="${3:-}"

    if [[ ! "$dist_name" =~ ^(minimal|personal|team)$ ]]; then
        print_error "无效的发行版名称: $dist_name (支持: minimal, personal, team)"
        exit 1
    fi
    if [ "$dist_name" = "team" ]; then
        print_error "Team Docker 镜像请在 centag-pro 仓库构建（开源仓已删除 dist/team）。"
        print_info "见 centag-pro/README.md；本地: cd ../centag-pro && ./start.sh build team"
        exit 1
    fi

    if [ -z "$tag" ]; then
        tag="centag-${dist_name}:latest"
    fi

    # 所有版本都构建前端（前端通过 CENTAG_EDITION 自动适配 UI）
    local include_frontend="true"

    # 准备 initdata archive（存放到 temp 目录）
    local initdata_archive_flag=false
    local initdata_temp_dir="/tmp/centag-initdata-$$"
    mkdir -p "$initdata_temp_dir"

    if [ -n "$initdata_path" ]; then
        if [ ! -f "$initdata_path" ]; then
            print_error "initdata 文件不存在: $initdata_path"
            rm -rf "$initdata_temp_dir"
            exit 1
        fi
        cp "$initdata_path" "$initdata_temp_dir/initdata.zip"
        initdata_archive_flag=true
        print_info "使用自定义 initdata: $initdata_path"
    else
        # 创建默认 initdata.zip
        print_info "生成默认 initdata.zip..."
        (
            cd "$initdata_temp_dir"
            # personal/minimal → common only；team → common + team
            mkdir -p pipeline-templates/common
            for f in "$PROJECT_ROOT"/config/initdata/pipeline-templates/common/*.yaml; do
                [ -f "$f" ] && cp "$f" pipeline-templates/common/
            done
            case "${dist_name}" in
                team|centag-pro|pro)
                    mkdir -p pipeline-templates/team
                    for f in "$PROJECT_ROOT"/config/initdata/pipeline-templates/team/*.yaml; do
                        [ -f "$f" ] && cp "$f" pipeline-templates/team/
                    done
                    ;;
            esac

            # 首启无预置后端（由 WebUI「添加 Provider」配置；勿塞无 Key 的占位后端）
            cat > initial-backends.yaml << 'INITDATA_EOF'
version: "2.0"
description: First-boot empty backends — add providers in WebUI
backends: []
INITDATA_EOF

            zip -r initdata.zip .
        )
        initdata_archive_flag=true
        print_info "默认 initdata 已生成（backends 种子随 Profile，默认空）"
    fi

    # 构建
    local build_tags=$(_get_dist_tags "$dist_name")
    print_info "构建 Docker 镜像: ${tag} (dist=${dist_name}, frontend=${include_frontend})..."
    docker build \
        --build-arg DIST_NAME="$dist_name" \
        --build-arg INCLUDE_FRONTEND="$include_frontend" \
        --build-arg INITDATA_ARCHIVE="$initdata_archive_flag" \
        --build-arg BUILD_TAGS="$build_tags" \
        --build-context initdata="$initdata_temp_dir" \
        -t "$tag" \
        -f deploy/docker/Dockerfile.dist \
        .
    local rc=$?
    rm -rf "$initdata_temp_dir"

    if [ $rc -eq 0 ]; then
        print_success "镜像构建完成: ${tag}"
    else
        print_error "镜像构建失败"
        exit 1
    fi
}

# ── Docker 运行容器（按版本）─────────────────────────────────────────
# 参数: dist_name port initdata_path reset_data
#   reset_data=true 时，清空宿主机 var/docker-data/<dist_name>/storage 下的旧库/密码，
#   让容器以当前 config/secrets/.env 的 LLM_PROXY_ADMIN_PASSWORD 重新 seed。
_dist_docker_run() {
    local dist_name="${1:-minimal}"
    local port="${2:-20060}"
    local initdata_path="${3:-}"
    local reset_data="${4:-false}"

    if [[ ! "$dist_name" =~ ^(minimal|personal|team)$ ]]; then
        print_error "无效的发行版名称: $dist_name (支持: minimal, personal, team)"
        exit 1
    fi
    if [ "$dist_name" = "team" ]; then
        print_error "Team Docker 请在 centag-pro 仓库执行: cd ../centag-pro && ./start.sh profile team up"
        exit 1
    fi

    local tag="centag-${dist_name}:latest"
    # 检查镜像是否存在，不存在则先构建
    if ! docker image inspect "$tag" >/dev/null 2>&1; then
        print_info "镜像 ${tag} 不存在，先执行构建..."
        _dist_docker_build "$dist_name" "" "$initdata_path"
    fi

    load_env

    # 确保没有同名容器残留（否则 docker run --name 冲突）
    if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^centag-${dist_name}$"; then
        print_info "移除旧容器 centag-${dist_name}..."
        docker rm -f "centag-${dist_name}" >/dev/null 2>&1 || true
        sleep 1  # 等待端口释放
    fi

    # 共享 resolve_backend_port 统一端口检测逻辑
    resolve_backend_port
    port=$BACKEND_PORT

    print_info "启动容器: ${tag} (端口 ${port})..."
    # 覆盖 secrets 里常见的 LLM_PROXY_LOG_OUTPUT=file：否则 zap 只写
    # /app/bin/logs，docker logs 只能看到 entrypoint/插件 std 初始化行。
    # 与 docker-compose.yaml 对齐：both + console + /app/logs。
    # 20060=API/Web；8081=系统代理 MITM（wrap run / PAC 需要映射到宿主机）
    #
    # 持久化（personal/minimal，非 PG）：
    #   二进制在 /app/bin/centag，相对路径 ./storage 会落到 /app/bin/storage（不可持久）。
    #   必须用绝对 SQLITE_PATH=/app/storage/centag.db，并挂载宿主机目录。
    local mitm_port="${LLM_PROXY_SYSTEM_PROXY_PORT:-8081}"
    if [ "$reset_data" = "true" ]; then
        print_warn "重置 ${dist_name} 数据卷..."
        docker volume rm -f "centag-${dist_name}-storage" "centag-${dist_name}-logs" "centag-${dist_name}-certs" 2>/dev/null || true
        print_info "已删除数据卷，容器将重新 seed"
    fi

    # 确保数据卷存在
    docker volume inspect "centag-${dist_name}-storage" >/dev/null 2>&1 || docker volume create "centag-${dist_name}-storage"
    docker volume inspect "centag-${dist_name}-logs" >/dev/null 2>&1 || docker volume create "centag-${dist_name}-logs"
    docker volume inspect "centag-${dist_name}-certs" >/dev/null 2>&1 || docker volume create "centag-${dist_name}-certs"

    print_info "数据卷:"
    print_info "  - centag-${dist_name}-storage (SQLite + 配置)"
    print_info "  - centag-${dist_name}-logs"
    print_info "  - centag-${dist_name}-certs (MITM CA)"
    print_info "启动容器 (后台): centag-${dist_name}"
    docker run -d --rm \
        --name "centag-${dist_name}" \
        --env-file "${PROJECT_ROOT}/config/secrets/.env" \
        -e CENTAG_EDITION="${dist_name}" \
        -e CENTAG_IN_DOCKER=1 \
        -e CENTAG_DATA_DIR=/app/storage \
        -e LLM_PROXY_DB_DRIVER=sqlite \
        -e SQLITE_PATH=/app/storage/centag.db \
        -e MEMORY_STORE_ROOT=/app/storage/memory-store \
        -e LLM_PROXY_LOG_OUTPUT=both \
        -e LLM_PROXY_LOG_FORMAT=console \
        -e LLM_PROXY_LOG_PATH=/app/logs \
        -p "${port}:20060" \
        -p "${mitm_port}:8081" \
        -v "centag-${dist_name}-storage:/app/storage" \
        -v "centag-${dist_name}-logs:/app/logs" \
        -v "centag-${dist_name}-certs:/app/bin/certs" \
        "$tag"

    echo ""
    print_success "容器 centag-${dist_name} 已后台启动"
    print_info "查看日志: docker logs -f centag-${dist_name}"
    print_info "停止容器: docker stop centag-${dist_name}"
}

# Docker Compose：附加 config/secrets/.env 作为「项目级」变量，供 compose 文件中 ${VAR} 插值（与容器内 env_file 无关）
docker_compose_invoke() {
    local compose_cmd="$1"
    shift
    local env_args=""
    if [ -f "$PROJECT_ROOT/config/secrets/.env" ]; then
        env_args="--env-file $PROJECT_ROOT/config/secrets/.env"
    fi
    eval "$compose_cmd $env_args $@"
}

# Docker Compose 启动（本仓库 compose 仅含 centag；中间件见 deploy/stack）
docker_up() {
    local edition="${1:-personal}"
    if [[ ! "$edition" =~ ^(personal|minimal)$ ]]; then
        print_error "不支持的 edition: $edition（仅支持 personal / minimal）"
        exit 1
    fi
    local image="centag-${edition}:latest"
    check_docker

    if [ ! -f "$PROJECT_ROOT/config/secrets/.env" ]; then
        print_warn "未找到 config/secrets/.env，正在自动生成认证配置..."
        "${PROJECT_ROOT}/scripts/ops/generate-secrets.sh" --same-password
    fi

    load_env

    if ! docker image inspect "$image" >/dev/null 2>&1; then
        print_warn "主服务镜像 ${image} 不存在，正在构建..."
        _dist_docker_build "$edition" "" ""
    fi

    # 检查 docker-compose 命令
    local compose_cmd="docker-compose"
    if ! command -v docker-compose >/dev/null 2>&1; then
        compose_cmd="docker compose"
    fi

    if ! $compose_cmd version >/dev/null 2>&1; then
        print_error "docker-compose 未安装"
        print_warn "请访问 https://docs.docker.com/compose/install/ 安装docker-compose"
        exit 1
    fi

    print_info "启动 Centag 容器: ${GREEN}${edition}${NC}"
    echo ""

    cd "$PROJECT_ROOT/deploy/docker"

    CENTAG_EDITION="$edition" docker_compose_invoke "$compose_cmd" up -d
    cd "$PROJECT_ROOT"

    print_success "服务已启动"
    echo ""
    print_info "服务信息:"
    echo "  - Centag: http://localhost:${LLM_PROXY_SERVER_PORT:-20060}"
    echo ""
    print_info "查看日志: ./start.sh docker logs"
    print_info "查看状态: ./start.sh docker status"
    print_info "停止服务: ./start.sh docker down"
}

# Docker Compose 停止
docker_down() {
    check_docker

    local compose_cmd="docker-compose"
    if ! command -v docker-compose >/dev/null 2>&1; then
        compose_cmd="docker compose"
    fi

    print_info "停止 Docker Compose 服务..."
    cd "$PROJECT_ROOT/deploy/docker"
    
    # 根据操作系统选择对应的 debug 配置
    local os_type=$(uname -s)
    local debug_compose_file=""
    if [ "$os_type" = "Darwin" ]; then
        debug_compose_file="docker-compose.debug.yaml"
    else
        debug_compose_file="docker-compose.debug.host.yaml"
    fi
    
    # 如果存在 debug 配置文件，先用 debug 配置停止
    if [ -f "$debug_compose_file" ]; then
        docker_compose_invoke "$compose_cmd" -f docker-compose.yaml -f "$debug_compose_file" down --remove-orphans 2>/dev/null || true
    fi
    # 再用默认配置停止（清理残留）
    docker_compose_invoke "$compose_cmd" down --remove-orphans
    cd "$PROJECT_ROOT"
    print_success "服务已停止"
}

# Docker Debug 模式启动（挂载本地编译的二进制）
docker_debug() {
    if [ -n "${1:-}" ]; then
        print_warn "已忽略多余参数「$1」：./start.sh docker debug 不再接受 profile。"
    fi
    check_docker

    # 自动检查并生成认证配置
    if [ ! -f "$PROJECT_ROOT/config/secrets/.env" ]; then
        print_warn "未找到 config/secrets/.env，正在自动生成认证配置..."
        "${PROJECT_ROOT}/scripts/ops/generate-secrets.sh" --same-password
    fi

    # 统一加载 config/secrets/.env
    load_env

    # 检查镜像是否存在
    if ! docker image inspect centag-personal:latest >/dev/null 2>&1; then
        print_warn "主服务镜像 centag-personal:latest 不存在，正在构建..."
        _dist_docker_build personal "" ""
    fi

    # 编译本地二进制（强制为 linux/amd64 架构，容器需要）
    cd "$PROJECT_ROOT"
    mkdir -p bin

    # 检测当前平台
    local build_arch=$(uname -m)
    local need_cross_compile=false

    # 检查是否需要交叉编译
    if [ "$build_arch" = "arm64" ] || [ "$build_arch" = "aarch64" ]; then
        print_info "检测到 Apple Silicon (arm64)，将交叉编译为 linux/amd64..."
        need_cross_compile=true
    elif [ "$build_arch" = "x86_64" ]; then
        # 即使是 x86_64 的 macOS，也需要交叉编译为 Linux
        print_info "检测到 macOS x86_64，将交叉编译为 linux/amd64..."
        need_cross_compile=true
    fi

    # 始终交叉编译为 linux/amd64，确保与容器兼容
    if [ "$need_cross_compile" = true ]; then
        print_info "正在交叉编译为 linux/amd64 架构..."
        mkdir -p "$BIN_DIR"
        eval "GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags '$_FULL_FEATURE_TAGS' -o \"$BIN_DIR/$SERVER_BIN\" ./cmd/centag/main.go"
    else
        print_info "正在编译为 linux/amd64 架构..."
        mkdir -p "$BIN_DIR"
        eval "CGO_ENABLED=0 go build -tags '$_FULL_FEATURE_TAGS' -o \"$BIN_DIR/$SERVER_BIN\" ./cmd/centag/main.go"
    fi

    if [ $? -eq 0 ]; then
        # 验证编译结果（macOS 和 Linux 的 file 命令输出格式不同）
        local compiled_arch=$(file -b "$BIN_DIR/$SERVER_BIN" 2>/dev/null || echo "")
        if echo "$compiled_arch" | grep -qE "ELF|Linux"; then
            print_success "编译成功: $BIN_DIR/centag ($(echo $compiled_arch | cut -d, -f1))"
        else
            print_error "编译结果不是 Linux 二进制: $compiled_arch"
            exit 1
        fi
    else
        print_error "编译失败"
        exit 1
    fi

    # 构建前端（如果 web/dist 不存在或需要更新）
    print_info "正在构建前端..."
    cd "$PROJECT_ROOT/web"
    if [ ! -d "dist" ] || [ "$(find dist -type f | wc -l)" -eq 0 ]; then
        print_info "前端 dist 目录为空，执行构建..."
        npm run build
        if [ $? -ne 0 ]; then
            print_error "前端构建失败"
            exit 1
        fi
        print_success "前端构建成功"
    else
        print_info "前端 dist 目录已存在，跳过构建（删除 web/dist 可强制重建）"
    fi
    cd "$PROJECT_ROOT"

    # 检查 docker-compose 命令
    local compose_cmd="docker-compose"
    if ! command -v docker-compose >/dev/null 2>&1; then
        compose_cmd="docker compose"
    fi

    if ! $compose_cmd version >/dev/null 2>&1; then
        print_error "docker-compose 未安装"
        exit 1
    fi

    cd "$PROJECT_ROOT/deploy/docker"

    # 检测操作系统，选择对应的 debug 配置
    local os_type=$(uname -s)
    local debug_compose_file=""
    if [ "$os_type" = "Darwin" ]; then
        debug_compose_file="docker-compose.debug.yaml"
        print_info "检测到 macOS，使用 Docker 网络模式..."
    else
        debug_compose_file="docker-compose.debug.host.yaml"
        print_info "检测到 Linux，使用 host 网络模式..."
    fi

    # 确保 debug 脚本有执行权限
    chmod +x entrypoint.debug.sh

    print_info "======================================"
    print_info "  Debug 模式启动（挂载本地 bin 目录）"
    print_info "======================================"
    echo ""
echo "  本地二进制: $BIN_DIR/centag"
    echo ""
    echo "  1. 本地修改代码 -> make build  # → ~/.centag/lib/personal/centag-personal"
    echo "  2. ./start.sh docker restart"
    echo ""

    # 启动服务（使用 debug compose 文件，--force-recreate 强制重建容器）
    print_info "使用 $debug_compose_file 启用调试模式..."
    docker_compose_invoke "$compose_cmd" -f docker-compose.yaml -f "$debug_compose_file" up -d --force-recreate
    cd "$PROJECT_ROOT"

    print_success "Debug 模式服务已启动"
    echo ""
    print_info "服务信息:"
    echo "  - Centag: http://localhost:20060"
    echo ""
    print_info "工作流程:"
    echo "  1. 本地修改代码（Go 后端或 Vue 前端）"
    echo "  2. ./start.sh docker debug  # 自动构建并重启"
    echo ""
    print_info "提示: 删除 web/dist 目录可强制重建前端"
    echo ""
    print_info "查看日志: ./start.sh docker logs"
    print_info "停止服务: ./start.sh docker down"
}

# Docker 重启主服务（用于 debug 模式更新二进制）
docker_restart() {
    check_docker

    local compose_cmd="docker-compose"
    if ! command -v docker-compose >/dev/null 2>&1; then
        compose_cmd="docker compose"
    fi

    local service="${1:-centag}"
    cd "$PROJECT_ROOT/deploy/docker"

    # 根据操作系统选择对应的 debug 配置
    local os_type=$(uname -s)
    local debug_compose_file=""
    local is_debug_mode=false
    
    if [ "$os_type" = "Darwin" ]; then
        debug_compose_file="docker-compose.debug.yaml"
    else
        debug_compose_file="docker-compose.debug.host.yaml"
    fi

    # 检查是否是 debug 模式
    if [ -f "$debug_compose_file" ]; then
        is_debug_mode=true
        print_info "检测到 debug 模式配置 ($debug_compose_file)..."
    fi

    if [ "$is_debug_mode" = true ]; then
        # 如果指定了服务，先检查本地二进制
        if [ "$service" = "centag" ]; then
            if [ -f "$BIN_DIR/$SERVER_BIN" ]; then
                print_info "本地二进制已更新: $BIN_DIR/centag"
            fi
        fi

        # 重启服务（保留 debug compose 配置）
        print_info "重启 $service ..."
        docker_compose_invoke "$compose_cmd" -f docker-compose.yaml -f "$debug_compose_file" restart "$service"
        cd "$PROJECT_ROOT"
        print_success "$service 已重启"
        echo ""
        print_info "查看日志: ./start.sh docker logs $service"
    else
        # 普通模式
        print_info "重启 $service ..."
        cd "$PROJECT_ROOT/deploy/docker"
        docker_compose_invoke "$compose_cmd" restart "$service"
        cd "$PROJECT_ROOT"
        print_success "$service 已重启"
    fi
}

# Docker 查看日志
docker_logs() {
    check_docker

    local compose_cmd="docker-compose"
    if ! command -v docker-compose >/dev/null 2>&1; then
        compose_cmd="docker compose"
    fi

    cd "$PROJECT_ROOT/deploy/docker"

    # 根据操作系统选择对应的 debug 配置
    local os_type=$(uname -s)
    local debug_compose_file=""
    local is_debug_mode=false
    
    if [ "$os_type" = "Darwin" ]; then
        debug_compose_file="docker-compose.debug.yaml"
    else
        debug_compose_file="docker-compose.debug.host.yaml"
    fi

    # 检查是否是 debug 模式
    if [ -f "$debug_compose_file" ]; then
        is_debug_mode=true
    fi

    if [ "$is_debug_mode" = true ]; then
        if [ -n "${1:-}" ]; then
            docker_compose_invoke "$compose_cmd" -f docker-compose.yaml -f "$debug_compose_file" logs -f "$1"
        else
            docker_compose_invoke "$compose_cmd" -f docker-compose.yaml -f "$debug_compose_file" logs -f
        fi
    else
        if [ -n "${1:-}" ]; then
            docker_compose_invoke "$compose_cmd" logs -f "$1"
        else
            docker_compose_invoke "$compose_cmd" logs -f
        fi
    fi
}

# Docker 状态
docker_status() {
    check_docker

    print_info "Docker 容器状态:"
    echo ""

    local compose_cmd="docker-compose"
    if ! command -v docker-compose >/dev/null 2>&1; then
        compose_cmd="docker compose"
    fi

    cd "$PROJECT_ROOT/deploy/docker"

    # 根据操作系统选择对应的 debug 配置
    local os_type=$(uname -s)
    local debug_compose_file=""
    local is_debug_mode=false
    
    if [ "$os_type" = "Darwin" ]; then
        debug_compose_file="docker-compose.debug.yaml"
    else
        debug_compose_file="docker-compose.debug.host.yaml"
    fi

    # 检查是否是 debug 模式
    if [ -f "$debug_compose_file" ]; then
        is_debug_mode=true
    fi

    if [ "$is_debug_mode" = true ]; then
        docker_compose_invoke "$compose_cmd" -f docker-compose.yaml -f "$debug_compose_file" ps
    else
        docker_compose_invoke "$compose_cmd" ps
    fi
    cd "$PROJECT_ROOT"
}

# Docker 清理
docker_clean() {
    check_docker

    local compose_cmd="docker-compose"
    if ! command -v docker-compose >/dev/null 2>&1; then
        compose_cmd="docker compose"
    fi

    print_warn "将删除所有容器、镜像、数据卷，确认继续？(y/N)"
    read -r confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        print_info "操作已取消"
        return
    fi

    print_info "清理 Docker 资源..."
    cd "$PROJECT_ROOT/deploy/docker"

    # 根据操作系统选择对应的 debug 配置
    local os_type=$(uname -s)
    local debug_compose_file=""
    local is_debug_mode=false
    
    if [ "$os_type" = "Darwin" ]; then
        debug_compose_file="docker-compose.debug.yaml"
    else
        debug_compose_file="docker-compose.debug.host.yaml"
    fi

    # 检查是否是 debug 模式
    if [ -f "$debug_compose_file" ]; then
        is_debug_mode=true
    fi

    if [ "$is_debug_mode" = true ]; then
        docker_compose_invoke "$compose_cmd" -f docker-compose.yaml -f "$debug_compose_file" down -v --rmi all
    else
        docker_compose_invoke "$compose_cmd" down -v --rmi all
    fi
    cd "$PROJECT_ROOT"

    # 删除主服务镜像
    docker rmi centag-personal:latest 2>/dev/null || true
    docker rmi centag-minimal:latest 2>/dev/null || true

    print_success "清理完成"
}

# Docker 打包镜像
docker_pack() {
    check_docker
    print_info "打包 Docker 镜像..."

    # 生成日期时间标签
    local timestamp=$(date +"%Y%m%d-%H%M")
    local package_name="centag-docker-${timestamp}"
    local package_dir="release/${package_name}"

    # 确保镜像存在
    if ! docker image inspect centag-personal:latest >/dev/null 2>&1; then
        print_warn "镜像不存在，正在构建..."
        _dist_docker_build personal "" ""
    fi

    # 创建打包目录
    mkdir -p "$package_dir"

    # 导出主服务镜像
    print_info "导出主服务镜像..."
    if docker save -o "${package_dir}/centag-image.tar" "centag-personal:latest"; then
        print_success "主服务镜像导出成功"
    else
        print_error "主服务镜像导出失败"
        exit 1
    fi

    # 中间件镜像由 deploy/stack 单独分发，此处仅打包主服务镜像与 compose。

    # 复制 docker-compose.yml
    print_info "复制配置文件..."
    mkdir -p "${package_dir}"
    cp deploy/docker/docker-compose.yaml "${package_dir}/docker-compose.yaml"
    print_success "配置文件复制完成"

    # 生成加载脚本
    print_info "生成加载脚本..."
    cat > "${package_dir}/load-images.sh" << 'LOADEOF'
#!/bin/bash
# 加载 Docker 镜像脚本

echo "加载 Docker 镜像..."

# 加载主服务镜像
if [ -f "centag-image.tar" ]; then
    echo "加载主服务镜像..."
    docker load -i centag-image.tar
fi

echo "镜像加载完成！"
echo ""
echo "使用以下命令启动服务:"
echo "  docker compose up -d"
LOADEOF
    chmod +x "${package_dir}/load-images.sh"

    # 创建 README
    cat > "${package_dir}/README.md" << 'READMEEOF'
# Centag Docker 部署包

## 目录结构

- `centag-image.tar` - 主服务镜像
- `docker-compose.yaml` - Docker Compose 配置文件（仅 centag 服务）
- `load-images.sh` - 镜像加载脚本

中间件（PostgreSQL、Redis、Mem0 等）请使用 **deploy/stack** 子项目单独部署。

## 部署步骤

### 1. 加载镜像

```bash
./load-images.sh
```

### 2. 配置环境变量

在仓库 `config/secrets/.env` 中配置数据库与可选依赖地址后，使用 compose 的 `--env-file` 启动（与主仓库 `./start.sh docker up` 一致）。

### 3. 启动服务

```bash
docker compose --env-file ../config/secrets/.env up -d
```

### 4. 查看日志 / 停止

```bash
docker compose logs -f
docker compose down
```

## 访问地址

- Web / API: http://localhost:20060（端口以 `LLM_PROXY_SERVER_PORT` 为准）
READMEEOF

    # 计算包大小
    local package_size=$(du -sh "$package_dir" | cut -f1)

    print_success "打包完成！"
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}📦 Package Complete!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}Package: ${package_dir}${NC}"
    echo -e "${GREEN}Size: ${package_size}${NC}"
    echo ""
    print_info "使用 ./load-images.sh 加载镜像"
}


# ═══════════════════════════════════════════════════════════
# 帮助系统
# ═══════════════════════════════════════════════════════════

# 命令列表（无参数 / --help / help 时显示）
show_short_help() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC}              ${GREEN}Centag Manager${NC}                     ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  ${CYAN}版本: ${CENTAG_VERSION}$(printf '%*s' $((31-${#CENTAG_VERSION})) '')${BLUE}║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${GREEN}用法: ./start.sh <命令> [参数...]${NC}"
    echo ""
    echo -e "${YELLOW}命令列表:${NC}"
    echo ""

    # ── 快速开始 ──
    echo -e "  ${GREEN}wizard${NC} | ${GREEN}w${NC} | ${GREEN}-w${NC}              交互式初始化向导（推荐新手）"
    echo ""

    # ── 服务管理 ──
    echo -e "  ${CYAN}── 服务管理 ──────────────────────────────────────────${NC}"
    echo -e "  ${GREEN}run${NC}      <be|fe|all> [--desktop] [--docker]  运行服务"
    echo -e "  ${GREEN}run${NC}      <personal|minimal> [--desktop] [--docker]  运行发行版"
    echo -e "  ${GREEN}run${NC}      wrap [子命令...]     系统代理出口 CLI（PAC/CA）"
    echo -e "  ${GREEN}daemon${NC}   [backend|stop|debug|status]  后台守护进程模式"
    echo -e "  ${GREEN}debug${NC} [personal|minimal] [--desktop]  开发模式（先构建+debug 启动）"
    echo -e "  ${GREEN}stop${NC}     [be|fe|daemon|all]   停止服务"
    echo -e "  ${GREEN}status${NC}                           查看服务状态"
    echo -e "  ${GREEN}logs${NC}                             查看服务日志"
    echo ""

    # ── 构建 ──
    echo -e "  ${CYAN}── 构建 ──────────────────────────────────────────────${NC}"
    echo -e "  ${GREEN}build${NC}    <all|be|fe>             开发构建（默认 all）"
    echo -e "  ${GREEN}build${NC}    <personal|minimal> [--desktop] [--docker] [--wrap]"
    echo -e "  ${GREEN}build${NC}    wrap                仅构建 centag-wrap"
    echo -e "  ${GREEN}clean${NC}    [build|install|all] [-y] 清理构建产物 / 已部署文件"
    echo -e "  ${GREEN}pack${NC}     [--upload]              打包服务端更新包（旧别名）"
    echo -e "  ${GREEN}package${NC}  ota [--platforms <目标平台> --edition <ed>] [--upload]   服务端 OTA 更新包（原 pack）"
    echo -e "  ${GREEN}package${NC}  <cli|desktop> <os> [arch]   部署包，平台参数统一用 --platforms"
    echo -e "  ${GREEN}test${NC}                             运行单元测试"
    echo ""

    # ── 环境 ──
    echo -e "  ${CYAN}── 环境配置 ──────────────────────────────────────────${NC}"
    echo -e "  ${GREEN}init${NC}                             初始化开发环境"
    echo -e "  ${GREEN}env${NC}      gen [--force]           生成密钥配置文件"
    echo ""

    # ── Stack & Docker ──
    echo -e "  ${CYAN}── Stack 中间件 & Docker ────────────────────────────${NC}"
    echo -e "  ${GREEN}stack${NC}    <start|stop|status|...>   中间件编排 (PG/Redis/ES/...)"
    echo -e "  ${GREEN}docker${NC}   <up|down|logs|status|...>  容器生命周期管理"
    echo ""

    # ── 其他 ──
    echo -e "  ${CYAN}── 其他 ──────────────────────────────────────────────${NC}"
    echo -e "  ${GREEN}webui${NC}    <dev|build|lint|clean>    Vue 前端开发"
    echo ""

    echo -e "  ${YELLOW}提示:${NC} 运行 ${GREEN}./start.sh <命令> --help${NC} 查看命令的详细用法"
    echo -e "        运行 ${GREEN}./start.sh --version${NC} 查看版本信息"
    echo ""
}

# 命令详细帮助分发
show_command_help() {
    local cmd="$1"
    echo ""
    case "$cmd" in
        wizard|w|-w|--wizard)  _help_wizard ;;
        init)          _help_init ;;
        build)         _help_build ;;
        run)           _help_run ;;
        daemon)        _help_daemon ;;
        debug)         _help_debug ;;
        stop)          _help_stop ;;
        status)        _help_status ;;
        logs)          _help_logs ;;
        clean)         _help_clean ;;
        stack)         _help_stack ;;
        docker)        _help_docker ;;
        webui)         _help_webui ;;
        pack)          _help_pack ;;
        package)       _help_package ;;
        test)          _help_test ;;
        env)           _help_env ;;
        *)
            print_error "未知命令: '$cmd'"
            echo ""
            show_short_help
            exit 1
            ;;
    esac
    echo ""
}

# ═══════════════════════════════════════════════════════════
# 各命令详细帮助函数
# ═══════════════════════════════════════════════════════════

_help_wizard() {
    echo -e "${GREEN}命令: wizard | w | -w${NC}"
    echo -e "       ${YELLOW}交互式初始化向导${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh wizard"
    echo -e "  ./start.sh w"
    echo -e "  ./start.sh -w"
    echo ""
    echo -e "${CYAN}说明:${NC}"
    echo -e "  引导完成环境配置、项目构建、数据库检查和启动服务，适合初次使用。"
    echo ""
    echo -e "${CYAN}向导流程:${NC}"
    echo -e "  1. 依赖检查 (Go / Docker / Node.js)"
    echo -e "  2. 密钥配置文件 (.env)"
    echo -e "  3. 项目构建"
    echo -e "  4. PostgreSQL 状态检查"
    echo -e "  5. 服务启动"
}

_help_init() {
    echo -e "${GREEN}命令: init${NC}"
    echo -e "       ${YELLOW}初始化开发环境${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh init"
    echo ""
    echo -e "${CYAN}说明:${NC}"
    echo -e "  下载依赖、复制配置、初始化数据库模板等。运行后即可编译启动。"
}

_help_build() {
    echo -e "${GREEN}命令: build${NC}"
    echo -e "       ${YELLOW}构建项目 / 发行版${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh build <目标> [--desktop] [--docker] [--wrap]"
    echo ""
    echo -e "${CYAN}开发构建:${NC}"
    echo -e "  ${GREEN}all${NC}             构建全部（后端 + 生产版前端） 【默认】"
    echo -e "  ${GREEN}be${NC} | backend     仅构建后端服务"
    echo -e "  ${GREEN}fe${NC} | frontend   构建 Vue 前端"
    echo ""
    echo -e "${CYAN}发行版构建:${NC}"
    echo -e "  ${GREEN}personal${NC}  个人全功能（默认 SQLite）"
    echo -e "  ${GREEN}minimal${NC}   轻量单机（文件配置，无 DB）"
    echo -e "  ${GREEN}team${NC}      团队版（中间件外置：PG/向量等）"
    echo -e "  ${GREEN}wrap${NC}  仅构建本机/员工系统代理工具 centag-wrap"
    echo ""
    echo -e "${CYAN}产品形态:${NC}"
    echo -e "  默认 = ${GREEN}cli${NC}（centag-cli / 前台二进制）"
    echo -e "  ${GREEN}--desktop${NC}        额外构建桌面外壳 centag-desktop（CGO/systray；仅 personal/minimal）"
    echo -e "  ${GREEN}--docker${NC}         构建 Docker 镜像（替代 docker build）"
    echo -e "  ${GREEN}--wrap${NC}    额外构建 centag-wrap（可与 personal/minimal 同用）"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh build personal              # CLI 二进制"
    echo -e "  ./start.sh build personal --desktop    # CLI sidecar + desktop 外壳"
    echo -e "  ./start.sh build personal --docker     # Docker 镜像"
    echo -e "  ./start.sh build personal --wrap       # CLI + 系统代理 CLI"
    echo -e "  ./start.sh build wrap"
    echo -e "  ./start.sh build minimal --desktop"
    echo -e "  ./start.sh build be"
    echo ""
    echo -e "${YELLOW}提示:${NC} Docker 镜像也可用: ./start.sh build personal --docker"
}

_help_run() {
    echo -e "${GREEN}命令: run${NC}"
    echo -e "       ${YELLOW}启动服务（前台运行）${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh run <服务> [--desktop] [--docker]"
    echo -e "  ./start.sh run wrap [子命令...] [选项...]"
    echo ""
    echo -e "${CYAN}服务:${NC}"
    echo -e "  ${GREEN}be${NC} | backend        启动后端服务 (端口 20060)"
    echo -e "  ${GREEN}fe${NC} | frontend      启动 Vue 开发服务器 (端口 5173)"
    echo -e "  ${GREEN}all${NC}                后端(后台) + 前端(前台)"
    echo -e "  ${GREEN}personal${NC}           个人版发行包"
    echo -e "  ${GREEN}minimal${NC}             minimal 发行包"
    echo -e "  ${GREEN}wrap${NC}            系统代理出口 CLI"
    echo ""
    echo -e "${CYAN}选项:${NC}"
    echo -e "  ${GREEN}--desktop${NC}        以桌面外壳启动（菜单栏/托盘；仅 personal/minimal）"
    echo -e "  ${GREEN}--docker${NC}         以 Docker 容器启动（替代 docker run）"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh run be                      # 后端守护（非 debug 默认；前台用 debug）"
    echo -e "  ./start.sh run fe                      # 前端开发服务器"
    echo -e "  ./start.sh run all                     # 后端+前端"
    echo -e "  ./start.sh run personal                # CLI"
    echo -e "  ./start.sh run personal --desktop      # desktop（托盘）"
    echo -e "  ./start.sh run personal --docker       # Docker 容器"
    echo -e "  ./start.sh run minimal --desktop"
    echo -e "  ./start.sh run wrap run --server http://192.168.1.10:20060 -- opencode"
    echo ""
    echo -e "${YELLOW}注意:${NC} 开发模式: ./start.sh run all（自动后台后端+前台前端）"
}

_help_daemon() {
    echo -e "${GREEN}命令: daemon${NC}"
    echo -e "       ${YELLOW}后台守护进程模式（自动重启）${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh daemon [子命令]"
    echo ""
    echo -e "${CYAN}子命令:${NC}"
    echo -e "  ${GREEN}backend${NC} | ${GREEN}be${NC}      启动后端守护进程 【默认】"
    echo -e "  ${GREEN}stop${NC}              停止守护进程"
    echo -e "  ${GREEN}debug${NC}             守护进程调试模式"
    echo -e "  ${GREEN}status${NC}            查看守护进程状态"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh daemon            # 启动守护进程"
    echo -e "  ./start.sh daemon stop       # 停止守护进程"
    echo -e "  ./start.sh daemon status     # 查看状态"
}

_help_debug() {
    echo -e "${GREEN}命令: debug${NC}"
    echo -e "       ${YELLOW}开发调试模式（先构建再以 debug 启动）${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh debug [personal|minimal] [--desktop|--docker]"
    echo ""
    echo -e "${CYAN}发行版:${NC}"
    echo -e "  ${GREEN}personal${NC}           CENTAG_EDITION=personal + 开源全功能二进制（默认）"
    echo -e "  ${GREEN}minimal${NC}             精简 WebUI + centag-minimal"
    echo -e "  ${YELLOW}team${NC}                本仓拒绝；请: cd ../centag-pro && ./start.sh debug team"
    echo ""
    echo -e "${CYAN}形态:${NC}"
    echo -e "  ${GREEN}(默认) cli${NC}          前台 sidecar"
    echo -e "  ${GREEN}--desktop${NC}           托盘外壳拉起 sidecar（仍为 debug 日志 + 前端 watch）"
    echo -e "  ${GREEN}--docker${NC}            构建并前台运行 Docker 容器（端口自动清理）"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh debug                      # personal CLI"
    echo -e "  ./start.sh debug minimal              # minimal CLI"
    echo -e "  ./start.sh debug personal --desktop   # personal + desktop"
    echo -e "  ./start.sh debug minimal --desktop    # minimal + desktop"
    echo -e "  ./start.sh debug personal --docker    # personal Docker 容器（前台）"
    echo -e "  ./start.sh debug minimal --docker     # minimal Docker 容器（前台）"
}

_help_stop() {
    echo -e "${GREEN}命令: stop${NC}"
    echo -e "       ${YELLOW}停止运行中的服务${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh stop [目标] [--docker]"
    echo ""
    echo -e "${CYAN}目标:${NC}"
    echo -e "  ${GREEN}be${NC} | backend     仅停止后端服务"
    echo -e "  ${GREEN}fe${NC} | frontend   仅停止 Vue 开发服务器"
    echo -e "  ${GREEN}daemon${NC}          仅停止守护进程"
    echo -e "  ${GREEN}all${NC}             停止所有服务 【默认】"
    echo -e "  ${GREEN}personal${NC} | ${GREEN}minimal${NC} ${YELLOW}--docker${NC}  停止 Docker 容器"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh stop              # 停止所有服务"
    echo -e "  ./start.sh stop be           # 仅停止后端"
    echo -e "  ./start.sh stop daemon       # 仅停止守护进程"
    echo -e "  ./start.sh stop personal --docker  # 停止 Docker 容器"
    echo -e "  ./start.sh stop minimal --docker   # 停止 minimal Docker 容器"
}

_help_status() {
    echo -e "${GREEN}命令: status${NC}"
    echo -e "       ${YELLOW}查看服务运行状态${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh status"
    echo ""
    echo -e "${CYAN}说明:${NC}"
    echo -e "  检查后端、前端、AI Assistant 和守护进程的运行状态。"
}

_help_logs() {
    echo -e "${GREEN}命令: logs${NC}"
    echo -e "       ${YELLOW}查看服务日志${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh logs"
    echo ""
    echo -e "${CYAN}说明:${NC}"
    echo -e "  显示后端服务 PID 和日志文件位置。如需实时查看日志，可使用:"
    echo -e "    tail -f ${CENTAG_EDITION_LIB}/logs/centag.log"
    echo -e "    tail -f ${CENTAG_EDITION_LIB}/storage/logs/*.log"
}

_help_clean() {
    echo -e "${GREEN}命令: clean${NC}"
    echo -e "       ${YELLOW}清理构建产物或已部署的安装布局${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh clean                 # 清理当前 edition 构建产物（lib/<edition>）"
    echo -e "  ./start.sh clean build           # 同上"
    echo -e "  ./start.sh clean install [-y]    # 删除安装根目录（默认 ~/.centag）"
    echo -e "  ./start.sh clean deploy [-y]     # 同 install"
    echo -e "  ./start.sh clean all [-y]        # 构建产物 + 安装布局"
    echo ""
    echo -e "${CYAN}说明:${NC}"
    echo -e "  ${GREEN}build${NC}   删除 \${CENTAG_EDITION_LIB}（当前: $BIN_DIR）"
    echo -e "  ${GREEN}install${NC} 停止进程后删除 \${CENTAG_INSTALL_ROOT}（当前: ${CENTAG_INSTALL_ROOT}）"
    echo -e "           含 bin/、lib/、var/（packages/release/cross）等；不删仓库 secrets"
    echo -e "  ${GREEN}-y${NC}      跳过交互确认（脚本/CI 用）"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh clean install -y      # 清除本机已安装/部署的 ~/.centag"
}

_help_stack() {
    echo -e "${GREEN}命令: stack${NC}"
    echo -e "       ${YELLOW}中间件服务编排（委托 deploy/stack）${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh stack <子命令> [参数...]"
    echo ""
    echo -e "${CYAN}常用子命令:${NC}"
    echo -e "  ${GREEN}start${NC} base         启动基础中间件 (PG/Redis/ES/Qdrant/Neo4j/Ollama)"
    echo -e "  ${GREEN}start${NC} mem0         启动 Mem0 全栈（含依赖）"
    echo -e "  ${GREEN}stop${NC}               停止所有中间件"
    echo -e "  ${GREEN}status${NC}             查看状态"
    echo -e "  ${GREEN}health${NC}             健康检查"
    echo -e "  ${GREEN}logs${NC} <服务>        查看指定服务日志"
    echo -e "  ${GREEN}ensure${NC} <服务...>   确保依赖已运行（Profile modular 模式）"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh stack start base"
    echo -e "  ./start.sh stack status"
    echo -e "  ./start.sh stack logs postgres"
}

_help_docker() {
    echo -e "${GREEN}命令: docker${NC}"
    echo -e "       ${YELLOW}容器生命周期管理（Docker Compose）${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh docker <子命令> [参数]"
    echo ""
    echo -e "${CYAN}Compose 操作:${NC}"
    echo -e "  ${GREEN}up${NC} [personal|minimal]   启动 Centag 容器（默认 personal）"
    echo -e "  ${GREEN}down${NC}                    停止并清理容器"
    echo -e "  ${GREEN}logs${NC} [service]          查看容器日志"
    echo -e "  ${GREEN}status${NC}                  查看容器状态"
    echo -e "  ${GREEN}restart${NC} [service]       重启容器"
    echo -e "  ${GREEN}debug${NC}                   启动 Debug 模式（挂载本地 bin）"
    echo -e "  ${GREEN}clean${NC}                   清理所有容器/镜像/数据卷"
    echo -e "  ${GREEN}pack${NC}                    打包镜像为 tar.gz"
    echo ""
    echo -e "${CYAN}发行版构建/运行:${NC}"
    echo -e "  ${GREEN}build${NC} <personal|minimal>   构建 Docker 镜像"
    echo -e "  ${GREEN}run${NC}   <personal|minimal>   运行 Docker 容器"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh docker build personal        # 构建镜像"
    echo -e "  ./start.sh docker up personal           # 启动容器"
    echo -e "  ./start.sh docker up minimal            # 启动 minimal 版"
    echo -e "  ./start.sh docker down                  # 停止容器"
    echo ""
}

_help_webui() {
    echo -e "${GREEN}命令: webui${NC}"
    echo -e "       ${YELLOW}Vue 前端开发管理${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh webui <子命令>"
    echo ""
    echo -e "${CYAN}子命令:${NC}"
    echo -e "  ${GREEN}dev${NC}     启动 Vue 开发服务器 (端口 5173)"
    echo -e "  ${GREEN}build${NC}   构建 Vue 生产版本"
    echo -e "  ${GREEN}lint${NC}    代码检查"
    echo -e "  ${GREEN}clean${NC}   清理构建产物"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh webui dev"
    echo -e "  ./start.sh webui build"
}

_help_pack() {
    echo -e "${GREEN}命令: pack${NC}（旧别名 → 推荐: package ota）"
    echo -e "       ${YELLOW}打包服务端更新包${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh package ota [选项...]   （推荐）"
    echo -e "  ./start.sh pack [选项...]          （旧别名）"
    echo ""
    echo -e "${CYAN}选项:${NC}"
    echo -e "  ${GREEN}--upload${NC}        打包并上传热更新（需设置认证 Token；仅支持单平台）"
    echo -e "  ${GREEN}--platforms${NC}    目标平台 goos-goarch，逗号分隔可多个（默认本机）；跨平台自动交叉编译"
    echo -e "  ${GREEN}--edition${NC}      版本 personal|minimal|team（默认布局 edition）"
    echo ""
    echo -e "${CYAN}说明:${NC}"
    echo -e "  --platforms 目标 ≠ 本机时自动交叉编译（team 委托 centag-pro，personal/minimal 用本仓 dist）。"
    echo ""
    echo -e "${CYAN}认证优先级:${NC}"
    echo -e "  CENTAG_UPDATE_TOKEN > LLM_PROXY_DEFAULT_ADMIN_API_KEY"
    echo -e "  > LLM_PROXY_ADMIN_API_KEY > CENTAG_API_KEY"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  # 本机打包"
    echo -e "  ./start.sh package ota"
    echo -e "  # 指定平台/版本（跨平台自动交叉编译）"
    echo -e "  ./start.sh package ota --platforms linux-amd64 --edition team"
    echo -e "  ./start.sh package ota --platforms linux-amd64,linux-arm64 --edition personal"
    echo -e "  # 打包并热更新（需认证 Token，单平台）"
    echo -e "  CENTAG_UPDATE_TOKEN=xxx ./start.sh package ota --platforms linux-arm64 --edition personal --upload"
}

_help_package() {
    echo -e "${GREEN}命令: package${NC}"
    echo -e "       ${YELLOW}部署包 = 形态 × 系统 × 架构；服务端 OTA = ota${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh package ota [--upload]     服务端 OTA 更新包（原 pack，可热更新）"
    echo -e "  ./start.sh package <form> <os> [arch] [选项...]   部署包"
    echo -e "  ./start.sh package list"
    echo ""
    echo -e "${CYAN}维度:${NC}"
    echo -e "  form  ${GREEN}ota${NC} | ${GREEN}cli${NC} | ${GREEN}desktop${NC}   （ota=OTA，cli=命令行，desktop=桌面含托盘）"
    echo -e "  os    ${GREEN}macos${NC} | ${GREEN}linux${NC} | ${GREEN}windows${NC} | ${GREEN}fnos${NC} | ${GREEN}docker${NC}"
    echo -e "  arch  ${GREEN}amd64${NC} | ${GREEN}arm64${NC} | ${GREEN}host${NC} | ${GREEN}all${NC}（desktop 默认 host；cli/linux 默认 all）"
    echo ""
    echo -e "${CYAN}平台参数（统一用 --platforms <goos-goarch,...>，跨平台自动交叉编译）:${NC}"
    echo -e "  ota        --platforms linux-amd64,linux-arm64 --edition <personal|minimal|team> --upload"
    echo -e "  cli*       --platforms darwin-amd64,darwin-arm64 --components <personal|minimal> --version <v>"
    echo -e "             --skip-frontend --desktop"
    echo ""
    echo -e "${CYAN}其他可选参数（按打包脚本透传）:${NC}"
    echo -e "  desktop    --version <v> --skip-frontend  跳过前端构建（用已有 static）"
    echo -e "  fnos       --mode <native|docker> --edition <minimal|personal|team> --arch <amd64|arm64>"
    echo -e "             --output <dir> --image-prefix <registry/>"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  # 服务端 OTA（原 pack；平台参数用 --platforms，同 cli）"
    echo -e "  ./start.sh package ota"
    echo -e "  ./start.sh package ota --platforms linux-amd64 --edition team"
    echo -e "  ./start.sh package ota --platforms linux-amd64,linux-arm64 --edition personal"
    echo -e "  CENTAG_UPDATE_TOKEN=xxx ./start.sh package ota --platforms linux-arm64 --edition personal --upload"
    echo ""
    echo -e "  # 桌面（须本机对应系统）"
    echo -e "  ./start.sh package desktop macos"
    echo -e "  ./start.sh package desktop macos host --version v0.2.7 --skip-frontend"
    echo -e "  ./start.sh package desktop windows"
    echo ""
    echo -e "  # CLI（可交叉编译，默认 all 双架构）"
    echo -e "  ./start.sh package cli linux"
    echo -e "  ./start.sh package cli linux amd64"
    echo -e "  ./start.sh package cli linux arm64 --version v0.2.7 --components minimal"
    echo -e "  ./start.sh package cli macos all --platforms darwin-amd64,darwin-arm64"
    echo -e "  ./start.sh package cli docker"
    echo ""
    echo -e "  # fnOS（.fpk；edition/mode 默认取 packaging.env）"
    echo -e "  ./start.sh package cli fnos amd64 --edition personal"
    echo -e "  ./start.sh package cli fnos amd64 --edition team --mode native"
    echo -e "  ./start.sh package cli fnos arm64 --mode docker --image-prefix ghcr.io/marmotcai/"
    echo ""
    echo -e "  # 总览"
    echo -e "  ./start.sh package list"
}

_help_test() {
    echo -e "${GREEN}命令: test${NC}"
    echo -e "       ${YELLOW}运行测试${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh test"
    echo ""
    echo -e "${CYAN}说明:${NC}"
    echo -e "  运行项目的单元测试和集成测试。"
}

_help_env() {
    echo -e "${GREEN}命令: env${NC}"
    echo -e "       ${YELLOW}环境变量/密钥配置管理${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh env <子命令> [选项]"
    echo ""
    echo -e "${CYAN}子命令:${NC}"
    echo -e "  ${GREEN}gen${NC} [--force]      生成密钥配置文件 config/secrets/.env"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh env gen"
    echo -e "  ./start.sh env gen --force   # 强制覆盖已有配置"
}

# ============================================
# 类型标准化 - 将简写转换为标准名称
# ============================================

normalize_type() {
    local type="$1"
    case "$type" in
        be|backend|后端)
            echo "backend"
            ;;
        fe|frontend|web|webui|前端)
            echo "frontend"
            ;;
        all|全部)
            echo "all"
            ;;
        personal|个人版)
            echo "personal"
            ;;
        minimal|精简版)
            echo "minimal"
            ;;
        team|团队版)
            echo "team"
            ;;
        *)
            echo "$type"
            ;;
    esac
}

# ============================================
# 向导功能
# ============================================

# Wizard 环境配置
wizard_env_config() {
    wizard_step "2" "环境配置"

    print_message $CYAN "💡 配置项目环境"
    echo ""

    # 须先于 setup/copy-files：生成 SQLite 时会读取 POSTGRES_PASSWORD 等变量
    # 仅以 config/secrets/.env 为准（与 load_env 主配置一致）；仅有 .env.middleware 时仍视为未就绪，默认 [Y/n] 生成
    if [ ! -f "$PROJECT_ROOT/config/secrets/.env" ]; then
        print_message $YELLOW "⚠️  config/secrets/.env 不存在（主密钥配置缺失）"
        if wizard_confirm "是否生成密钥配置文件?" "y"; then
            generate_secrets --same-password
        fi
    else
        print_info "config/secrets/.env 已存在，跳过生成（如需重新生成请执行: ./start.sh generate-secrets）"
    fi

    # 再初始化 bin / 复制静态资源（make copy-files 可能先放入模板库）
    if [ ! -d "$BIN_DIR" ]; then
        print_message $YELLOW "⚠️  bin 目录不存在，需要进行初始化"
        if wizard_confirm "是否运行 setup 初始化环境?" "y"; then
            setup
        fi
    fi
}

# 追踪向导流程中 Vue 前端是否已构建，避免步骤 4 重复构建
WIZARD_VUE_BUILT=false

# Wizard 构建选项
wizard_check_pg() {
    wizard_step "4" "PostgreSQL 状态检查"

    print_message $CYAN "💡 检查 PostgreSQL 数据库是否已运行"
    echo ""

    local pg_running=false
    if command -v docker >/dev/null 2>&1; then
        if docker ps --format '{{.Names}}' 2>/dev/null | grep -qiE 'postgres|postgresql'; then
            pg_running=true
        fi
    fi

    if [ "$pg_running" = true ]; then
        print_success "✅ 检测到运行中的 PostgreSQL 相关容器，无需操作"
        return 0
    fi

    print_message $YELLOW "⚠️  未在 docker ps 中检测到明显的 PostgreSQL 容器名"
    echo ""
    print_message $CYAN "PostgreSQL 与 Mem0 等中间件已迁移到子项目 deploy/stack，例如:"
    echo "    cd deploy/stack && ./start.sh start base"
    echo ""
    if wizard_confirm "我已在其它终端启动数据库，继续向导" "y"; then
        return 0
    fi
    print_message $YELLOW "请先启动数据库后再运行本向导或 ./start.sh run be"
}

wizard_build() {
    wizard_step "3" "项目构建"

    print_message $CYAN "💡 构建后端服务和前端资源"
    echo ""

    # 检查是否有 Vue 前端目录
    local has_vue=false
    if [ -d "$PROJECT_ROOT/web" ]; then
        has_vue=true
    fi

    # 选择构建目标
    echo -e "${CYAN}选择要构建的服务:${NC}"
    echo "  1 | all   - 全部构建 (后端 + Vue前端)"
    echo "  2 | be    - 仅构建后端"
    if [ "$has_vue" = true ]; then
        echo "  3 | vue   - 仅构建 Vue 前端"
        echo "  4 | skip  - 跳过构建"
    else
        echo "  3 | skip  - 跳过构建"
    fi
    echo ""
    echo -e "${YELLOW}提示: 可直接输入数字或简写 (如: 1 或 all)${NC}"
    echo ""

    local choice=$(wizard_read "请输入选项" "1")

    # 标准化输入（去除前后空格，转小写）
    choice=$(echo "$choice" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')

    case "$choice" in
        1|all)
            print_message $BLUE "🏗️  开始完整构建 (包含 Vue 前端)..."
            build all
            WIZARD_VUE_BUILT=true
            ;;
        2|be|backend)
            print_message $BLUE "🏗️  构建后端..."
            build backend
            # 询问是否构建 Vue 前端（仅构建后端服务后）
            if [ "$has_vue" = true ]; then
                echo ""
                if wizard_confirm "是否额外构建 Vue 前端?" "n"; then
                    build webui
                    WIZARD_VUE_BUILT=true
                fi
            fi
            ;;
        3|vue)
            if [ "$has_vue" = true ]; then
                print_message $BLUE "🏗️  构建 Vue 前端..."
                build webui
                WIZARD_VUE_BUILT=true
            else
                print_message $YELLOW "⚠️  跳过构建步骤"
            fi
            ;;
        4|skip|3)
            print_message $YELLOW "⚠️  跳过构建步骤"
            ;;
        *)
            if [ "$has_vue" = true ]; then
                print_message $YELLOW "⚠️  无效选项 '$choice'，跳过构建"
                print_message $YELLOW "    有效选项: 1/all, 2/be, 3/vue, 4/skip"
            else
                print_message $YELLOW "⚠️  无效选项 '$choice'，跳过构建"
                print_message $YELLOW "    有效选项: 1/all, 2/be, 3/skip"
            fi
            ;;
    esac
}

# Wizard 运行模式
wizard_run_mode() {
    wizard_step "5" "运行服务"

    print_message $CYAN "💡 选择服务的运行方式"
    echo ""

    # 检查是否有 Vue 前端目录
    local has_vue=false
    if [ -d "$PROJECT_ROOT/web" ]; then
        has_vue=true
    fi

    # 选择运行模式
    echo -e "${CYAN}运行模式:${NC}"
    echo "  1 | bg     - 后台运行 (推荐日常使用)"
    echo "  2 | debug  - 前台调试模式 (查看实时日志)"
    echo "  3 | daemon - 守护进程模式 (自动重启)"
    echo "  4 | docker - Docker 容器模式"
    echo "  5 | skip   - 跳过运行"
    echo ""

    local run_mode=$(wizard_read "请输入选项" "1")
    run_mode=$(echo "$run_mode" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')

    case "$run_mode" in
        5|skip)
            print_message $YELLOW "⚠️  跳过运行"
            return
            ;;
        4|docker)
            print_message $BLUE "🐳 Docker 容器模式..."
            echo ""
            print_info "使用 Docker Compose 启动所有服务"
            echo ""
            docker_up all
            return
            ;;
    esac

    # 选择要运行的服务
    echo ""
    echo -e "${CYAN}选择要运行的服务:${NC}"
    echo "  1 | all   - 全部服务 (后端 + 前端)"
    echo "  2 | be    - 仅启动后端"
    if [ "$has_vue" = true ]; then
        echo "  3 | dev   - 开发模式 (后端 + Vue 开发服务器)"
        echo "  4 | vue   - 仅启动 Vue 开发服务器"
    fi
    echo ""

    local choice=$(wizard_read "请输入选项" "1")
    choice=$(echo "$choice" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')

    case "$run_mode" in
        1|bg)
            print_message $BLUE "🚀 后台运行模式..."
            echo ""

            case "$choice" in
                1|all)
                    print_info "启动全部服务..."
                    print_info "后端端口: $BACKEND_PORT"
                    print_info "前端访问: http://localhost:$BACKEND_PORT"
                    echo ""
                    if wizard_confirm "是否现在启动?" "y"; then
                        load_env
                        resolve_backend_port || continue
                        [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1
                        cd "$BIN_DIR"
                        nohup ./"$SERVER_BIN" > logs/centag.log 2>&1 &
                        cd "$PROJECT_ROOT"
                        sleep 2
                        if lsof -ti ":$BACKEND_PORT" >/dev/null 2>&1; then
                            print_success "✅ 服务已启动 (后台运行)"
                            print_info "日志文件: ${BIN_DIR}/logs/centag.log"
                            print_info "停止服务: ./start.sh stop all"
                        else
                            print_error "❌ 服务启动失败，请查看日志"
                        fi
                    fi
                    ;;
                2|be|backend)
                    print_info "启动后端服务..."
                    print_info "后端端口: $BACKEND_PORT"
                    echo ""
                    if wizard_confirm "是否现在启动?" "y"; then
                        load_env
                        resolve_backend_port || continue
                        [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1
                        cd "$BIN_DIR"
                        nohup ./"$SERVER_BIN" > logs/centag.log 2>&1 &
                        cd "$PROJECT_ROOT"
                        sleep 2
                        if lsof -ti ":$BACKEND_PORT" >/dev/null 2>&1; then
                            print_success "✅ 后端服务已启动 (后台运行)"
                            print_info "日志文件: ${BIN_DIR}/logs/centag.log"
                            print_info "停止服务: ./start.sh stop backend"
                        else
                            print_error "❌ 后端服务启动失败"
                        fi
                    fi
                    ;;
                3|dev)
                    if [ "$has_vue" = true ]; then
                        print_info "开发模式: 后端 + Vue 开发服务器..."
                        echo ""
                        print_warn "⚠️  开发模式需要两个终端窗口:"
                        print_warn "     终端1: ./start.sh run backend"
                        print_warn "     终端2: ./start.sh run frontend (或 ./start.sh webui dev)"
                        echo ""
                        if wizard_confirm "是否现在在当前终端启动后端?" "y"; then
                            print_info "请在另一个终端运行: ./start.sh run frontend"
                            echo ""
                            load_env
                            resolve_backend_port || continue
                            [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1
                            print_test_examples
                            cd "$BIN_DIR"
                            ./"$SERVER_BIN"
                            cd "$PROJECT_ROOT"
                        else
                            print_info "您可以在两个终端分别运行以下命令:"
                            print_info "  终端1: ./start.sh run backend"
                            print_info "  终端2: ./start.sh run frontend"
                        fi
                    else
                        print_warn "⚠️  未检测到 Vue 前端目录"
                    fi
                    ;;
                4|vue)
                    if [ "$has_vue" = true ]; then
                        print_info "启动 Vue 开发服务器..."
                        print_info "前端开发服务器: http://localhost:5173"
                        echo ""
                        if wizard_confirm "是否现在启动?" "y"; then
                            webui_dev
                        fi
                    else
                        print_warn "⚠️  未检测到 Vue 前端目录"
                    fi
                    ;;
            esac
            ;;
        2|debug)
            print_message $BLUE "🔧 前台调试模式..."
            echo ""

            case "$choice" in
                1|all)
                    print_info "调试全部服务..."
                    print_info "后端端口: $BACKEND_PORT"
                    print_info "前端访问: http://localhost:$BACKEND_PORT"
                    echo ""
                    print_warn "提示: all 模式使用生产版本前端，集成在后端"
                    echo ""
                    if wizard_confirm "是否现在启动?" "y"; then
                        load_env
                        centag_export_debug_console_env
                        resolve_backend_port || continue
                        [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1
                        print_test_examples
                        cd "$BIN_DIR"
                        print_info "按 Ctrl+C 停止服务"
                        ./"$SERVER_BIN"
                        cd "$PROJECT_ROOT"
                    fi
                    ;;
                2|be|backend)
                    print_info "调试后端服务..."
                    print_info "后端端口: $BACKEND_PORT"
                    echo ""
                    if wizard_confirm "是否现在启动?" "y"; then
                        load_env
                        centag_export_debug_console_env
                        resolve_backend_port || continue
                        [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1
                        print_test_examples
                        cd "$BIN_DIR"
                        print_info "按 Ctrl+C 停止服务"
                        ./"$SERVER_BIN"
                        cd "$PROJECT_ROOT"
                    fi
                    ;;
                3|dev)
                    if [ "$has_vue" = true ]; then
                        print_info "开发模式: 后端 + Vue 开发服务器..."
                        echo ""
                        print_warn "⚠️  开发模式需要两个终端窗口:"
                        print_warn "     终端1: ./start.sh debug backend"
                        print_warn "     终端2: ./start.sh run frontend (或 ./start.sh webui dev)"
                        echo ""
                        if wizard_confirm "是否现在在当前终端启动后端?" "y"; then
                            print_info "请在另一个终端运行: ./start.sh run frontend"
                            echo ""
                            load_env
                            centag_export_debug_console_env
                            resolve_backend_port || continue
                            [ ! -f "$BIN_DIR/$SERVER_BIN" ] && build backend >/dev/null 2>&1
                            print_test_examples
                            cd "$BIN_DIR"
                            print_info "按 Ctrl+C 停止服务"
                            ./"$SERVER_BIN"
                            cd "$PROJECT_ROOT"
                        else
                            print_info "您可以在两个终端分别运行以下命令:"
                            print_info "  终端1: ./start.sh debug backend"
                            print_info "  终端2: ./start.sh run frontend"
                        fi
                    else
                        print_warn "⚠️  未检测到 Vue 前端目录"
                    fi
                    ;;
                4|vue)
                    if [ "$has_vue" = true ]; then
                        print_info "启动 Vue 开发服务器..."
                        print_info "前端开发服务器: http://localhost:5173"
                        echo ""
                        if wizard_confirm "是否现在启动?" "y"; then
                            webui_dev
                        fi
                    else
                        print_warn "⚠️  未检测到 Vue 前端目录"
                    fi
                    ;;
            esac
            ;;
        3|daemon)
            print_message $BLUE "🛡️  守护进程模式 (自动重启)..."
            echo ""

            case "$choice" in
                1|all)
                    print_info "守护进程启动全部服务..."
                    print_info "后端端口: $BACKEND_PORT"
                    print_info "前端访问: http://localhost:$BACKEND_PORT"
                    echo ""
                    if wizard_confirm "是否现在启动?" "y"; then
                        if [ "$WIZARD_VUE_BUILT" != "true" ]; then
                            webui_build 2>/dev/null || true
                        fi
                        daemon
                    fi
                    ;;
                2|be|backend)
                    print_info "守护进程启动后端服务..."
                    print_info "后端端口: $BACKEND_PORT"
                    echo ""
                    if wizard_confirm "是否现在启动?" "y"; then
                        daemon
                    fi
                    ;;
                3|dev)
                    print_warn "⚠️  守护进程模式不支持开发模式"
                    print_info "建议使用后台模式或调试模式"
                    ;;
                4|vue)
                    print_warn "⚠️  守护进程模式不支持独立启动 Vue 开发服务器"
                    print_info "建议使用后台模式"
                    ;;
            esac
            ;;
    esac
}

# Wizard 完成
wizard_finish() {
    wizard_step "完成" "项目初始化完成"

    echo -e "${GREEN}🎉 项目初始化流程已完成！${NC}"
    echo ""

    # 显示 Web UI 登录信息
    print_message $CYAN "🔑 Web UI 登录信息:"
    echo ""

    local secrets_file="$PROJECT_ROOT/config/secrets/.env"
    [ ! -f "$secrets_file" ] && secrets_file="$PROJECT_ROOT/config/secrets/.env.middleware"

    if [ -f "$secrets_file" ]; then
        local admin_user=$(grep "^LLM_PROXY_ADMIN_USERNAME=" "$secrets_file" | cut -d'=' -f2)
        local admin_pass=$(grep "^LLM_PROXY_ADMIN_PASSWORD=" "$secrets_file" | cut -d'=' -f2)

        if [ -n "$admin_user" ] && [ -n "$admin_pass" ]; then
            echo -e "  ${GREEN}✅${NC} 用户名: $admin_user"
            echo -e "  ${GREEN}✅${NC} 密码:   $admin_pass"
            echo ""
            print_warn "提示: 首次登录后建议修改密码"
            echo ""
        else
            print_warn "未找到 Web UI 管理员凭据配置"
            print_info "默认用户名: admin"
            print_info "未预置默认口令；请通过首次启动的初始化向导设置密码"
            print_info "或运行 ./start.sh env gen 生成新凭据"
            echo ""
        fi
    else
        print_warn "未找到密钥配置文件"
        print_info "默认用户名: admin"
        print_info "未预置默认口令；请通过首次启动的初始化向导设置密码"
        print_info "或运行 ./start.sh env gen 生成新凭据"
        echo ""
    fi

    print_message $CYAN "📋 常用命令参考:"
    echo ""
    echo -e "${YELLOW}查看状态:${NC}"
    echo "  ./start.sh status          # 查看服务状态"
    echo ""
    echo -e "${YELLOW}管理服务:${NC}"
    echo "  ./start.sh run backend      # 启动后端"
    echo "  ./start.sh run frontend    # 启动 Vue 开发服务器"
    echo "  ./start.sh run all         # 启动全部"
    echo "  ./start.sh stop backend     # 停止后端"
    echo "  ./start.sh stop frontend    # 停止前端"
    echo "  ./start.sh stop all        # 停止全部"
    echo ""
    echo -e "${YELLOW}开发模式:${NC}"
    echo "  ./start.sh debug           # personal（默认）+ 前端热重载"
    echo "  ./start.sh debug minimal   # minimal 精简版"
    echo "  # Team：cd ../centag-pro && ./start.sh build team"
    echo "  ./start.sh logs           # 查看日志"
    echo ""
    echo -e "${YELLOW}构建:${NC}"
    echo "  ./start.sh build all      # 构建所有"
    echo "  ./start.sh build backend   # 仅构建后端"
    echo "  ./start.sh build frontend  # 构建前端"
    echo ""
    echo -e "${YELLOW}Docker 操作:${NC}"
    echo "  ./start.sh docker up      # 仅启动 Centag 容器（默认）"
    echo "  cd deploy/stack && ./start.sh help   # 依赖栈：PG/Redis/ES/Ollama/Mem0 等"
    echo "  ./start.sh docker down    # 停止容器"
    echo "  ./start.sh docker logs    # 查看容器日志"
    echo ""

    # 显示访问地址
    print_message $CYAN "🌐 服务访问地址:"
    if curl -s http://localhost:$BACKEND_PORT/health >/dev/null 2>&1; then
        echo -e "  ${GREEN}✅${NC} 后端 API:   http://localhost:$BACKEND_PORT"
        echo -e "  ${GREEN}✅${NC} Web 界面:  http://localhost:$BACKEND_PORT"
    else
        echo -e "  ${YELLOW}⚠️${NC}  后端 API:   http://localhost:$BACKEND_PORT (未运行)"
        echo -e "  ${YELLOW}⚠️${NC}  Web 界面:  http://localhost:$BACKEND_PORT (未运行)"
    fi

    if [ -d "$PROJECT_ROOT/web" ] && lsof -ti ":5173" >/dev/null 2>&1; then
        echo -e "  ${GREEN}✅${NC} Vue Dev:    http://localhost:5173"
    fi

    echo ""
    print_message $GREEN "✨ 祝您使用愉快！"
}

# 主向导函数
run_wizard() {
    print_title
    echo ""
    echo -e "${CYAN}欢迎使用项目初始化向导！${NC}"
    echo -e "${CYAN}本向导将引导您完成项目的初始化、构建和部署流程。${NC}"
    echo ""

    # 步骤 1: 依赖检查
    wizard_step "1" "依赖检查"
    check_dependencies

    # 步骤 2: 环境配置
    wizard_env_config

    # 步骤 3: 项目构建
    wizard_build

    # 步骤 4: PostgreSQL 状态检查
    wizard_check_pg

    # 步骤 5: 运行服务
    wizard_run_mode

    # 完成
    wizard_finish
}

# ============================================
# 统一命令处理 - 按照 命令 类型 目标 格式
# ============================================

main() {
    # --version / version 显示版本信息
    if [ "${1:-}" = "--version" ] || [ "${1:-}" = "version" ]; then
        show_version
        exit 0
    fi

    # 无参数 / --help / -h / help 显示命令列表
    if [ -z "${1:-}" ] || [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ] || [ "${1:-}" = "help" ]; then
        show_short_help
        exit 0
    fi

    local cmd="${1:-}"
    shift || true

    # 如果第一个参数是 --help/-h，显示该命令的详细帮助
    if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
        show_command_help "$cmd"
        exit 0
    fi

    case "$cmd" in

        # ── 向导 ─────────────────────────────────────────────────────
        wizard|w|-w|--wizard)
            run_wizard
            ;;

        # ── 环境初始化 ─────────────────────────────────────────────────
        init)
            setup
            ;;

        env)
            local env_cmd="${1:-gen}"
            shift || true
            case "$env_cmd" in
                gen) generate_secrets "$@" ;;
                *)
                    print_error "未知 env 子命令: '$env_cmd'"
                    echo "用法: $0 env gen [--interactive] [--force]"
                    exit 1
                    ;;
            esac
            ;;

        # ── 构建 ────────────────────────────────────────────────────────
        build)
            local target="${1:-all}"
            shift || true
            local with_desktop=false
            local with_docker=false
            local with_wrap=false
            local unknown_args=()
            for arg in "$@"; do
                case "$arg" in
                    --desktop) with_desktop=true ;;
                    --docker)  with_docker=true ;;
                    --wrap)    with_wrap=true ;;
                    *)
                        unknown_args+=("$arg")
                        ;;
                esac
            done
            if [ ${#unknown_args[@]} -gt 0 ]; then
                print_error "未知 build 参数: ${unknown_args[*]}"
                echo "用法: $0 build <目标> [--desktop] [--docker] [--wrap]"
                exit 1
            fi
            target=$(normalize_type "$target")
            case "$target" in
                # 开发构建
                backend|be)
                    if $with_desktop || $with_docker; then
                        print_error "--desktop/--docker 不能用于 build be"
                        exit 1
                    fi
                    $with_wrap && { print_error "--wrap 不能用于 build be；请用: build wrap"; exit 1; }
                    build backend
                    ;;
                frontend|fe|vue)
                    if $with_desktop || $with_docker; then
                        print_error "--desktop/--docker 不能用于 build fe"
                        exit 1
                    fi
                    $with_wrap && { print_error "--wrap 不能用于 build fe；请用: build wrap"; exit 1; }
                    build webui
                    ;;
                all)
                    if $with_desktop || $with_docker; then
                        print_error "--desktop/--docker 不能用于 build all；请用: build personal [--desktop|--docker]"
                        exit 1
                    fi
                    $with_wrap && { print_error "--wrap 不能用于 build all；请用: build wrap"; exit 1; }
                    build all
                    ;;
                # 系统代理
                wrap)
                    if $with_desktop || $with_docker; then
                        print_error "--desktop/--docker 不能与 build wrap 同用"
                        exit 1
                    fi
                    build_wrap_shell
                    print_success "Ready: centag-wrap ($(go env GOOS)/$(go env GOARCH))"
                    print_info "真源命令: cd apps/wrap && GOWORK=off go build -o centag-wrap ."
                    ;;
                # 发行版构建
                personal|minimal|team)
                    if $with_desktop && $with_docker; then
                        print_error "--desktop 和 --docker 不能同时使用"
                        exit 1
                    fi
                    if $with_docker; then
                        _dist_docker_build "$target" "" ""
                    elif $with_desktop; then
                        build_with_desktop "$target"
                    else
                        build dist "$(edition_to_dist "$target")"
                    fi
                    $with_wrap && build_wrap_shell
                    ;;
                *)
                    print_error "未知构建目标: '$target'"
                    echo "支持的构建目标: all, be, fe, personal, minimal, wrap（Team → centag-pro）"
                    echo "选项: --desktop | --docker | --wrap"
                    exit 1
                    ;;
            esac
            ;;

        # ── Web UI ─────────────────────────────────────────────────────
        webui)
            local wsub="${1:-build}"
            shift || true
            case "$wsub" in
                dev) webui_dev ;;
                build) webui_build ;;
                lint) webui_lint ;;
                clean) webui_clean ;;
                *)
                    print_error "未知 webui 子命令: '$wsub'"
                    echo "用法: $0 webui <dev|build|lint|clean>"
                    exit 1
                    ;;
            esac
            ;;

        # ── 运行（前台）────────────────────────────────────────────────
        run)
            local svc="${1:-backend}"
            shift || true

            # 解析选项
            local with_desktop=false
            local with_docker=false
            local extra_args=()
            for arg in "$@"; do
                case "$arg" in
                    --desktop) with_desktop=true ;;
                    --docker)  with_docker=true ;;
                    *)         extra_args+=("$arg") ;;
                esac
            done

            case "$svc" in
                # 开发运行
                backend|be)
                    if $with_desktop || $with_docker; then
                        print_error "--desktop/--docker 不适用于 run be"
                        exit 1
                    fi
                    # 非 debug 默认守护进程（崩溃拉起 + OTA update_stop）；前台请用 debug
                    start_backend_background
                    ;;
                frontend|fe|vue)
                    if $with_desktop || $with_docker; then
                        print_error "--desktop/--docker 不适用于 run fe"
                        exit 1
                    fi
                    start_frontend_dev
                    ;;
                all)
                    if $with_desktop || $with_docker; then
                        print_error "--desktop/--docker 不适用于 run all"
                        exit 1
                    fi
                    _run_all_dev
                    ;;
                # 发行版运行
                personal|minimal)
                    if $with_docker; then
                        if $with_desktop; then
                            print_error "--desktop 和 --docker 不能同时使用"
                            exit 1
                        fi
                        _dist_docker_run "$svc" "" "" "false"
                    elif $with_desktop; then
                        run_edition "$svc" --desktop "${extra_args[@]}"
                    else
                        run_edition "$svc" "${extra_args[@]}"
                    fi
                    ;;
                # 系统代理
                wrap)
                    run_wrap "${extra_args[@]}"
                    ;;
                team)
                    print_error "team 请用 Docker/Profile 运行；托盘不支持 team"
                    echo "示例: ./start.sh run personal --docker  或  ./start.sh profile team up"
                    exit 1
                    ;;
                *)
                    print_error "未知运行目标: $svc"
                    echo "支持: be, fe, all, personal, minimal, wrap"
                    echo "选项: --desktop | --docker"
                    exit 1
                    ;;
            esac
            ;;

        # ── 守护进程 ────────────────────────────────────────────────────
        daemon)
            local daemon_sub="${1:-backend}"
            case "$daemon_sub" in
                stop)
                    daemon-stop
                    ;;
                debug)
                    daemon-debug
                    ;;
                status)
                    _daemon_status
                    ;;
                backend|be)
                    daemon
                    ;;
                *)
                    print_error "未知 daemon 子命令: '$daemon_sub'"
                    echo "用法: $0 daemon [backend|stop|debug|status]"
                    exit 1
                    ;;
            esac
            ;;

        # ── 调试（后端 debug 模式 + 前端热重载）────────────────────────
        debug)
            local edition="personal"
            local with_desktop=false
            local with_docker=false
            for arg in "$@"; do
                case "$arg" in
                    --desktop) with_desktop=true ;;
                    --docker) with_docker=true ;;
                    personal|minimal) edition="$arg" ;;
                esac
            done
            debug "$edition" "$with_desktop" "$with_docker"
            ;;

        # ── 停止 ───────────────────────────────────────────────────────
        stop)
            local svc="${1:-all}"
            shift || true

            local with_docker=false
            for arg in "$@"; do
                case "$arg" in
                    --docker) with_docker=true ;;
                    *) ;;
                esac
            done

            if $with_docker; then
                local container="centag-${svc}"
                if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${container}$"; then
                    print_info "停止 Docker 容器: ${container}"
                    docker stop "${container}"
                    print_success "容器 ${container} 已停止"
                else
                    print_warn "容器 ${container} 未运行"
                fi
            else
                case "$svc" in
                    backend|be)
                        _stop_backend_only
                        ;;
                    frontend|fe|vue)
                        kill_port 5173
                        print_success "Vue dev server stopped"
                        ;;
                    daemon)
                        daemon-stop
                        ;;
                    personal|minimal)
                        print_error "停止发行版 $svc 请使用 --docker 参数"
                        echo "用法: $0 stop $svc --docker"
                        exit 1
                        ;;
                    all|"")
                        stop
                        kill_port 5173
                        print_success "All services stopped"
                        ;;
                    *)
                        print_error "未知停止目标: $svc"
                        echo "支持: be, fe, daemon, all, personal --docker, minimal --docker"
                        exit 1
                        ;;
                esac
            fi
            ;;

        # ── 状态和日志 ─────────────────────────────────────────────────
        status)
            print_title
            echo ""

            # 后端状态
            echo -e "${CYAN}📊 后端服务:${NC}"
            if command -v lsof >/dev/null 2>&1; then
                lsof -ti ":$BACKEND_PORT" >/dev/null && print_success "✅ 后端服务运行中 (端口: $BACKEND_PORT)" || print_warn "❌ 后端服务未运行"
            fi

            # Vue 开发服务器状态
            echo ""
            echo -e "${CYAN}🎨 Vue 开发服务器:${NC}"
            if command -v lsof >/dev/null 2>&1; then
                lsof -ti ":5173" >/dev/null && print_success "✅ Vue 开发服务器运行中 (端口: 5173)" || print_warn "❌ Vue 开发服务器未运行"
            fi

            # 守护进程状态
            echo ""
            echo -e "${CYAN}🛡️  守护进程:${NC}"
            local daemon_pid_file="$BIN_DIR/storage/centag.daemon.pid"
            if [ -f "$daemon_pid_file" ]; then
                local daemon_pid=$(cat "$daemon_pid_file" 2>/dev/null || true)
                if [ -n "$daemon_pid" ] && kill -0 "$daemon_pid" 2>/dev/null; then
                    print_success "✅ 守护进程运行中 (PID: $daemon_pid)"
                else
                    print_warn "❌ 守护进程 PID 文件存在但进程未运行"
                fi
            else
                print_warn "❌ 守护进程未运行"
            fi

            echo ""
            print_info "访问地址:"
            echo "  后端 API:  http://localhost:$BACKEND_PORT"
            echo "  Web 界面:  http://localhost:$BACKEND_PORT"
            if lsof -ti ":5173" >/dev/null 2>&1; then
                echo "  Vue Dev:    http://localhost:5173"
            fi
            echo ""
            ;;

        logs)
            # 查看服务日志（需要实现 view_logs 函数或使用 docker logs）
            if command -v lsof >/dev/null 2>&1; then
                local backend_pid=$(lsof -ti ":$BACKEND_PORT" 2>/dev/null || true)
                if [ -n "$backend_pid" ]; then
                    print_info "Backend PID: $backend_pid"
                    print_info "日志文件位置: $BIN_DIR/storage/logs/"
                    ls -la "$BIN_DIR/storage/logs/" 2>/dev/null || echo "日志目录不存在"
                else
                    print_warn "后端服务未运行"
                fi
            fi
            ;;

        # ── 清理 ───────────────────────────────────────────────────────
        clean)
            clean "$@"
            ;;

        # ── Stack 中间件（加载 deploy/stack/lib）────────────────────
        stack)
            # main() 已 shift 掉顶层命令名，此处勿再 shift
            stack_cmd "$@"
            ;;

        # ── Docker ─────────────────────────────────────────────────────
         docker)
            local docker_cmd="${1:-}"
            shift || true
            case "$docker_cmd" in
                # ── 发行版 Docker 操作（新）──
                build)
                    local edition="${1:-}"
                    if [ -z "$edition" ]; then
                        print_error "请指定发行版: minimal, personal, 或 team"
                        echo "用法: $0 docker build <minimal|personal|team>"
                        echo "兼容别名: all → personal；backend|be → minimal"
                        exit 1
                    fi
                    case "$edition" in
                        minimal|personal|team)
                            _dist_docker_build "$edition" "" ""
                            ;;
                        all|backend|be|frontend|fe)
                            # 向后兼容旧目标名（勿把未知拼写吞进 all/*）
                            docker_build "$edition"
                            ;;
                        *)
                            print_error "无效的发行版名称: $edition (支持: minimal, personal, team)"
                            echo "用法: $0 docker build <minimal|personal|team>"
                            exit 1
                            ;;
                    esac
                    ;;
                run)
                    local edition="${1:-}"
                    if [ -z "$edition" ]; then
                        print_error "请指定发行版: minimal, personal, 或 team"
                        echo "用法: $0 docker run <minimal|personal|team> [port] [--reset] [--initdata <path>]"
                        exit 1
                    fi
                    shift
                    local port="20060"
                    local initdata_path=""
                    local reset_data="false"
                    while [ $# -gt 0 ]; do
                        case "$1" in
                            --initdata)
                                initdata_path="$2"
                                shift 2
                                ;;
                            --reset)
                                reset_data="true"
                                shift
                                ;;
                            *)
                                if [[ "$1" =~ ^[0-9]+$ ]]; then
                                    port="$1"
                                else
                                    print_warn "忽略未知参数: $1"
                                fi
                                shift
                                ;;
                        esac
                    done
                    _dist_docker_run "$edition" "$port" "$initdata_path" "$reset_data"
                    ;;
                # ── Docker Compose 操作（保留）──
                up)
                    docker_up "${1:-}"
                    ;;
                down)
                    docker_down
                    ;;
                logs)
                    docker_logs "${1:-}"
                    ;;
                status)
                    docker_status
                    ;;
                clean)
                    docker_clean
                    ;;
                pack)
                    docker_pack
                    ;;
                debug)
                    docker_debug "${1:-}"
                    ;;
                restart)
                    docker_restart "${1:-}"
                    ;;
                *)
                    print_error "未知 docker 子命令: '$docker_cmd'"
                    echo "用法: $0 docker <子命令> [参数]"
                    echo ""
                    echo "发行版操作:"
                    echo "  $0 docker build <minimal|personal|team>                   构建 Docker 镜像"
                    echo "  $0 docker run   <minimal|personal|team> [port] [--reset]  运行 Docker 容器"
                    echo ""
                    echo "Compose 操作:"
                    echo "  $0 docker up|down|logs|status|clean|pack|debug|restart"
                    exit 1
                    ;;
            esac
            ;;

        # ── 打包 ─────────────────────────────────────────────────────
        pack)
            print_info "提示: pack 已并入 package 命令，推荐使用: ./start.sh package ota [--upload]（pack 仍可用）"
            pack "$@"
            ;;

        # ── 部署包统一入口（桌面 / GitHub / CLI / fnOS / Docker）────────
        package)
            # ota = 服务端 OTA 更新包（原 pack；service/update 为兼容别名）
            case "${1:-}" in
                ota|service|update|hot-update)
                    shift
                    pack "$@"
                    ;;
                *)
                    local package_script="${PROJECT_ROOT}/scripts/packaging/package.sh"
                    if [ ! -f "$package_script" ]; then
                        print_error "打包入口不存在: $package_script"
                        exit 1
                    fi
                    bash "$package_script" "$@"
                    ;;
            esac
            ;;

        # ── 测试 ─────────────────────────────────────────────────────
        test)
            test
            ;;

        *)
            print_error "未知命令: $cmd"
            echo ""
            show_short_help
            exit 1
            ;;
    esac
}

main "$@"
