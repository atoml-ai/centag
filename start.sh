#!/bin/bash

# Centag - Build, Run, Debug Script (Linux/macOS)

set -euo pipefail

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
readonly BIN_DIR="${PROJECT_ROOT}/bin/server"
readonly PACKAGES_DIR="${PROJECT_ROOT}/bin/packages"
readonly STATIC_DIR="${PROJECT_ROOT}/bin/server/static"
readonly BACKEND_PORT=20060

cd "$PROJECT_ROOT" || exit 1

# Allow Go to automatically download required toolchain
export GOTOOLCHAIN=auto

# Add Go bin to PATH
if command -v go >/dev/null 2>&1; then
    export PATH="$PATH:$(go env GOPATH)/bin"
fi

# 版本号 — 优先从 git tag 获取，否则使用日期
CENTAG_VERSION=""
if command -v git >/dev/null 2>&1; then
    CENTAG_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || true)
fi
if [ -z "$CENTAG_VERSION" ]; then
    CENTAG_VERSION="v2.1-dev"
fi
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

    # 方法1: 使用 lsof (最可靠)
    if command -v lsof >/dev/null 2>&1; then
        pids=$(lsof -ti ":$port" 2>/dev/null || true)

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
    pids=$(pgrep -f "${BIN_DIR}/centag" || true)
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

# 清理后端端口，失败时中止并打印占用进程信息。
kill_backend_port_or_exit() {
    if kill_port "$BACKEND_PORT"; then
        return 0
    fi
    print_error "端口 $BACKEND_PORT 清理失败，已中止启动，避免新旧进程混跑。"
    if command -v lsof >/dev/null 2>&1; then
        local occupied
        occupied=$(lsof -nP -iTCP:"$BACKEND_PORT" -sTCP:LISTEN 2>/dev/null || true)
        if [ -n "$occupied" ]; then
            print_warn "当前占用端口的进程："
            echo "$occupied"
        fi
    fi
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
# gateway / team 二进制插件集合对齐（个人全功能 vs 团队版差别在部署默认依赖，不在插件裁剪）
_FULL_FEATURE_TAGS="protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure"

_get_dist_tags() {
    local dist_name="$1"
    case "$dist_name" in
        minimal)
            echo "minimal,protocol_openai,protocol_anthropic,backend_openai,backend_ollama,backend_anthropic"
            ;;
        gateway|team)
            echo "$_FULL_FEATURE_TAGS"
            ;;
        *)
            echo ""
            ;;
    esac
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

    print_info "编译 ${output_name} ..."
    eval go build $tags_arg -ldflags \"$ldflags_str\" -o "${PROJECT_ROOT}/bin/server/${output_name}" "$package_path"

    if [ $? -ne 0 ]; then
        print_error "编译失败: ${output_name}"
        cd "$PROJECT_ROOT"
        exit 1
    fi

    local size=$(ls -lh "${PROJECT_ROOT}/bin/server/${output_name}" | awk '{print $5}')
    print_success "${output_name} 编译完成 (${size})"
    print_info "二进制: ${PROJECT_ROOT}/bin/server/${output_name}"

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

# Map product edition → dist binary name (personal uses gateway dist).
edition_to_dist() {
    case "$1" in
        personal|gateway) echo "gateway" ;;
        minimal) echo "minimal" ;;
        team) echo "team" ;;
        *) echo "$1" ;;
    esac
}

edition_to_sidecar() {
    case "$1" in
        personal|gateway) echo "centag-gateway" ;;
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
    plat="${PROJECT_ROOT}/bin/launcher/${goos}-${goarch}/centag-launcher${ext}"
    if [ -x "$plat" ] || [ -f "$plat" ]; then
        echo "$plat"
        return 0
    fi
    local latest="${PROJECT_ROOT}/bin/launcher/centag-launcher${ext}"
    if [ -x "$latest" ] || [ -f "$latest" ]; then
        echo "$latest"
        return 0
    fi
    return 1
}

# Optional L1 launcher (apps/launcher). Used via --launcher on build/run.
# Native build for current OS only (darwin|linux|windows); CGO required.
build_launcher_shell() {
    local script="${PROJECT_ROOT}/scripts/build-launcher.sh"
    if [ ! -x "$script" ]; then
        print_error "未找到构建脚本: $script"
        exit 1
    fi
    print_info "Building desktop launcher for current host ($(go env GOOS)/$(go env GOARCH))..."
    bash "$script"
}

# ./start.sh build <personal|minimal> --launcher
build_with_launcher() {
    local edition="$1"
    case "$edition" in
        personal|gateway|minimal) ;;
        team)
            print_error "--launcher 不支持 team（团队版请用 Web/Docker）"
            exit 1
            ;;
        *)
            print_error "--launcher 仅支持 personal / minimal（gateway 为 personal 别名）"
            exit 1
            ;;
    esac

    local dist_name
    dist_name="$(edition_to_dist "$edition")"
    local label="$edition"
    if [ "$edition" = "gateway" ]; then
        label="personal"
    fi

    print_info "Building ${label} service + launcher..."
    build_distribution "$dist_name"
    build_frontend_prod
    build_launcher_shell
    print_success "Ready: $(edition_to_sidecar "$edition") + launcher ($(go env GOOS)/$(go env GOARCH))"
}

# Build Distribution (minimal/gateway/team)
build_distribution() {
    local dist_name="${1:-minimal}"

    case "$dist_name" in
        minimal|gateway|team)
            ;;
        "")
            print_error "Please specify distribution: minimal, gateway, or team"
            print_info "Usage: $0 dist <minimal|gateway|team>"
            exit 1
            ;;
        *)
            print_error "Unknown distribution: $dist_name"
            print_info "Valid distributions: minimal, gateway, team"
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

    _compile_go_binary "$dist_dir" "." "$output_name" "$go_tags" ""
}

# Build Backend
build_backend() {
    mkdir -p "$BIN_DIR"
    make build

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

# Pack - 打包更新包（参考 gov-subscribe）
pack() {
    local upload=false
    local package_script="${PROJECT_ROOT}/scripts/release/package.sh"

    # 解析参数
    while [ $# -gt 0 ]; do
        case "$1" in
            --upload)
                upload=true
                shift
                ;;
            *)
                print_error "未知参数: $1"
                echo "用法: $0 pack [--upload]"
                echo "  --upload  先构建，再打包，然后更新到容器"
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

    # 确保已构建（如果没指定 --upload，仍然检查）
    if [ ! -f "$BIN_DIR/centag" ]; then
        print_error "未找到编译后的 centag，请先执行构建"
        echo "  ./start.sh build"
        exit 1
    fi

    mkdir -p "$PACKAGES_DIR"
    local package_path
    package_path="$(
        bash "$package_script" service \
            --version "$VERSION" \
            --build-time "$BUILD_TIME" \
            --source-bin "$BIN_DIR/centag" \
            --source-static "$BIN_DIR/static" \
            --out-dir "$PACKAGES_DIR"
    )"
    local package_name
    package_name="$(basename "$package_path" .tar.gz)"

    # 获取文件大小
    local package_size=$(du -h "$package_path" | cut -f1)

    print_success "Package created successfully!"
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}📦 Package Complete!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}Package: ${package_path}${NC}"
    echo -e "${GREEN}Size: ${package_size}${NC}"
    echo -e "${GREEN}Version: v${VERSION}${NC}"
    if [ -f "${package_path}.sha256" ]; then
        echo -e "${GREEN}Checksum: ${package_path}.sha256${NC}"
    fi
    if [ -f "${package_path}.manifest.json" ]; then
        echo -e "${GREEN}Manifest: ${package_path}.manifest.json${NC}"
    fi
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

# load_profile_env — Profile 部署环境链（方案 C Phase 3）
# 顺序：deploy/stack/.env → config/profiles/<name>/.env → config/secrets/.env（后者覆盖前者）
load_profile_env() {
    local profile_dir="$1"
    local stack_env="$PROJECT_ROOT/deploy/stack/.env"
    local profile_env="$profile_dir/.env"
    local secrets_env="$PROJECT_ROOT/config/secrets/.env"

    if [ -f "$stack_env" ]; then
        print_info "加载 deploy/stack/.env（中间件基准）..."
        set -a
        # shellcheck source=/dev/null
        source "$stack_env"
        set +a
    fi
    if [ -f "$profile_env" ]; then
        print_info "加载 profile 环境: ${profile_env#"$PROJECT_ROOT"/}"
        set -a
        # shellcheck source=/dev/null
        source "$profile_env"
        set +a
    fi
    if [ -f "$secrets_env" ]; then
        print_info "加载 config/secrets/.env（应用密钥）..."
        set -a
        # shellcheck source=/dev/null
        source "$secrets_env"
        set +a
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
    kill_backend_port_or_exit || return 1
    [ ! -f "$BIN_DIR/centag" ] && build
    print_test_examples
    cd "$BIN_DIR"
    print_info "Starting backend service from: $BIN_DIR (port: $BACKEND_PORT)..."
    ./centag
    cd "$PROJECT_ROOT"
}

# 前台调试：覆盖 secrets 里常见的 LLM_PROXY_LOG_OUTPUT=file（否则 zap 只写文件，终端看不到访问日志）
# 插件 init 使用标准库 log，仍走 stderr，故此前会出现「只有 Plugin initialized、无 request 日志」
centag_export_debug_console_env() {
    export LLM_PROXY_SERVER_MODE=debug
    export LLM_PROXY_LOG_LEVEL=debug
    export LLM_PROXY_LOG_FORMAT=console
    export LLM_PROXY_LOG_OUTPUT=both
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

# Debug - 开发模式：Vite 监听重建 (输出到 bin/static) + 后端 (debug 模式 + 控制台日志)
# WSL2 环境下 5173 端口不可达，改用 vite build --watch 直接输出到 bin/static/
# 前端文件变化后 Vite 自动重建，刷新浏览器 (localhost:20060) 即可看到最新内容
#
# 选项:
#   （默认）       CENTAG_EDITION=personal —— 对齐 gateway 个人全功能
#   --minimal      精简 WebUI + centag-minimal（edition=minimal）
#   --team         全功能二进制 + CENTAG_EDITION=team
debug() {
    # ── 解析选项 ──────────────────────────────────────────────────
    local use_minimal=false
    local use_team=false
    for arg in "$@"; do
        case "$arg" in
            --minimal) use_minimal=true ;;
            --team) use_team=true ;;
            *)
                print_error "未知 debug 选项: $arg"
                echo "用法: $0 debug [--minimal|--team]"
                exit 1
                ;;
        esac
    done

    if $use_minimal && $use_team; then
        print_error "--minimal 与 --team 不能同时使用"
        exit 1
    fi

    # ── minimal 分支：精简 WebUI + centag-minimal ─────────────────
    if $use_minimal; then
        _debug_minimal
        return
    fi

    # ── 全功能分支：webui 前端；默认 personal，--team 为团队版 ─────
    local edition="personal"
    if $use_team; then
        edition="team"
    fi

    load_env

    # 自动检测数据库模式
    detect_database_mode

    # 强制 debug 模式 + 控制台输出格式，便于开发时直接查看日志
    # 开发模式下同时写文件与 stdout，避免仅 file 时启动失败在终端无输出
    centag_export_debug_console_env

    # 覆盖 secrets 中的 edition（与 gateway/team Profile 语义对齐）
    export CENTAG_EDITION="${edition}"

    # ── 清理所有残留进程（保证前台独占）──────────────────────────
    cleanup_residual_processes

    rm -f "$BIN_DIR/storage/centag.pid" 2>/dev/null || true

    kill_backend_port_or_exit || return 1

    check_go
    print_info "编译后端 (edition=${edition})..."
    build backend

    # 检查前端依赖
    check_node
    local webui_dir="${PROJECT_ROOT}/web"
    if [ ! -d "$webui_dir/node_modules" ] || [ "$webui_dir/package.json" -nt "$webui_dir/node_modules/.package-lock.json" ]; then
        print_info "安装 Web UI 依赖..."
        cd "$webui_dir" && npm install && cd "$PROJECT_ROOT"
    fi

    # 确保 bin/static 目录存在
    mkdir -p "$BIN_DIR/static"

    # 后台启动 Vite watch 模式：监听 web/src 变化，直接构建到 bin/static/
    print_info "启动前端 watch 构建（变化后刷新浏览器即可生效）..."
    cd "$webui_dir"
    npx vite build --watch --outDir "$BIN_DIR/static" --emptyOutDir false > /tmp/centag-vite.log 2>&1 &
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

    # 退出时自动清理 Vite 进程
    trap "kill $vite_pid 2>/dev/null; print_info '已停止前端 watch 进程'" EXIT INT TERM

    echo ""
    print_info "════════════════════════════════════════"
    print_info "  开发模式已启动"
    print_info "  产品版本:    ${edition}"
    print_info "  访问地址:    http://localhost:$BACKEND_PORT"
    print_info "  前端变化后:  刷新浏览器即可看到最新内容"
    print_info "  后端变化后:  下次执行 debug 会先自动编译；也可 ./start.sh build be 单独编译"
    print_info "  按 Ctrl+C 停止所有服务"
    print_info "════════════════════════════════════════"
    echo ""

    # 前台启动后端（日志直接输出到控制台）
    cd "$BIN_DIR"
    CENTAG_EDITION="${edition}" ./centag
    cd "$PROJECT_ROOT"
}

# ./start.sh run <personal|minimal> [--launcher]
# Without --launcher: foreground sidecar. With --launcher: menu/tray + browser shell.
run_edition() {
    local edition="$1"
    shift || true
    local with_launcher=false
    local extra_args=()
    for arg in "$@"; do
        case "$arg" in
            --launcher) with_launcher=true ;;
            --) ;;
            *)
                extra_args+=("$arg")
                ;;
        esac
    done

    case "$edition" in
        personal|gateway|minimal) ;;
        team)
            if $with_launcher; then
                print_error "--launcher 不支持 team"
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
    if [ "$edition" = "gateway" ]; then
        run_edition="personal"
    fi

    local sidecar="${BIN_DIR}/${sidecar_name}"
    if [ ! -x "$sidecar" ] && [ ! -f "$sidecar" ]; then
        print_info "未找到 ${sidecar_name}，先构建 ${dist_name}..."
        build_distribution "$dist_name"
    fi

    load_env

    if ! $with_launcher; then
        print_info "启动 ${run_edition} 服务: ${sidecar}"
        cd "$BIN_DIR"
        CENTAG_EDITION="${run_edition}" exec "./${sidecar_name}"
    fi

    if [ ! -d "${BIN_DIR}/static" ] || [ ! -f "${BIN_DIR}/static/index.html" ]; then
        print_info "构建前端静态资源 → ${BIN_DIR}/static ..."
        build_frontend_prod
    fi

    local launcher_bin
    if ! launcher_bin="$(resolve_launcher_bin)"; then
        print_info "未找到当前系统启动器二进制，先构建..."
        build_launcher_shell
        launcher_bin="$(resolve_launcher_bin)" || {
            print_error "启动器二进制构建后仍未找到"
            exit 1
        }
    fi

    if [ -z "${LLM_PROXY_ADMIN_PASSWORD:-}" ]; then
        print_warn "未检测到 LLM_PROXY_ADMIN_PASSWORD；首轮 seed 将使用内置默认口令"
    else
        print_info "已加载管理员口令环境变量（来自 config/secrets/.env）"
    fi

    print_info "启动桌面启动器 edition=${run_edition} platform=$(go env GOOS)/$(go env GOARCH)"
    print_info "  launcher: ${launcher_bin}"
    print_info "  sidecar: ${sidecar}"
    print_info "  data: 用户数据目录（与 bin/server 开发库分离；见 apps/launcher/README）"
    # macOS bash 3.2 + set -u: empty "${arr[@]}" is "unbound variable"
    if [ ${#extra_args[@]} -gt 0 ]; then
        exec "$launcher_bin" -edition="$run_edition" -bin="$sidecar" "${extra_args[@]}"
    else
        exec "$launcher_bin" -edition="$run_edition" -bin="$sidecar"
    fi
}

# ── Minimal 调试：精简 WebUI（vite build）+ centag-minimal 后端 ─────────────
_debug_minimal() {
    load_env
    detect_database_mode
    centag_export_debug_console_env
    export CENTAG_EDITION=minimal
    export INITDATA_PATH="${PROJECT_ROOT}/config/profiles/minimal/initdata"

    cleanup_residual_processes
    rm -f "$BIN_DIR/storage/centag.pid" 2>/dev/null || true
    kill_backend_port_or_exit || return 1

    check_go
    print_info "编译 minimal 发行版后端..."
    build_distribution "minimal"

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
    trap "kill $vite_pid 2>/dev/null; print_info '已停止前端 watch'" EXIT INT TERM

    echo ""
    print_info "════════════════════════════════════════"
    print_info "  minimal 精简模式已启动"
    print_info "  前端:        WebUI (edition=minimal)"
    print_info "  访问地址:    http://localhost:$BACKEND_PORT/static/"
    print_info "  首次进入:    设置管理密码后登录"
    print_info "  页面:        概览 / 后端 / 策略 / 设置"
    print_info "  按 Ctrl+C 停止"
    print_info "════════════════════════════════════════"
    echo ""

    cd "$BIN_DIR"
    CENTAG_EDITION=minimal ./centag-minimal
    cd "$PROJECT_ROOT"
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
    kill_backend_port_or_exit || return 1
    [ ! -f "$BIN_DIR/centag" ] && build backend >/dev/null 2>&1
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

# Clean
clean() {
    echo "[INFO] Cleaning build artifacts..."
    rm -rf "$BIN_DIR"
    echo "[SUCCESS] Clean completed"
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
    local remaining_pids=$(ps aux | grep -E "bin/server/centag|/centag " | grep -v grep | grep -v "start.sh" | awk '{print $2}' || true)
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

    local process_check=$(ps aux | grep -E "bin/server/centag|/centag " | grep -v grep | grep -v "start.sh" || true)
    if [ -n "$process_check" ]; then
        print_error "Warning: Some centag processes may still be running:"
        echo "$process_check"
    else
        print_success "All services stopped successfully"
    fi
}

# Force Stop - 强制停止所有相关进程
force-stop() {
    print_warn "Force stopping all centag related processes..."

    # 1. 强制杀死所有 centag 二进制进程
    local all_pids=$(ps aux | grep -E "bin/server/centag|/centag " | grep -v grep | grep -v "start.sh" | awk '{print $2}' || true)
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
    local remaining=$(ps aux | grep -E "bin/server/centag|/centag " | grep -v grep | grep -v "start.sh" || true)
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
    npm run build

    if [ $? -eq 0 ]; then
        print_success "Web UI 构建完成!"
        print_info "构建产物位置: ../bin/server/static"
        cd "$PROJECT_ROOT"

        # 静态文件已直接构建到 bin/server/static，无需同步
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
    local static_dir="${PROJECT_ROOT}/bin/server/static"
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
    kill_backend_port_or_exit || return 1
    [ ! -f "$BIN_DIR/centag" ] && build backend >/dev/null 2>&1
    print_test_examples
    cd "$BIN_DIR"
    print_info "Starting backend from: $BIN_DIR (port: $BACKEND_PORT)..."
    ./centag
    cd "$PROJECT_ROOT"
}

# 启动后端服务（后台/守护进程）
start_backend_background() {
    load_env
    kill_backend_port_or_exit || return 1
    [ ! -f "$BIN_DIR/centag" ] && build backend >/dev/null 2>&1
    print_test_examples
    print_info "Starting daemon from: $BIN_DIR..."
    "${PROJECT_ROOT}/scripts/tools/daemon.sh" "$BIN_DIR"
}

# 启动前端开发服务器
start_frontend_dev() {
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
#   dist_name   — minimal|gateway|team
#   tag         — 镜像标签（可选，默认 centag-<dist_name>:latest）
#   initdata_path — 自定义 initdata.zip 路径（可选）
_dist_docker_build() {
    local dist_name="${1:-minimal}"
    local tag="${2:-}"
    local initdata_path="${3:-}"

    if [[ ! "$dist_name" =~ ^(minimal|gateway|team)$ ]]; then
        print_error "无效的发行版名称: $dist_name (支持: minimal, gateway, team)"
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
            mkdir -p pipeline-templates/common pipeline-templates/gateway

            # 复制流水线模板（按目录结构）
            for f in "$PROJECT_ROOT"/config/initdata/pipeline-templates/common/*.yaml; do
                [ -f "$f" ] && cp "$f" pipeline-templates/common/
            done
            for f in "$PROJECT_ROOT"/config/initdata/pipeline-templates/gateway/*.yaml; do
                [ -f "$f" ] && cp "$f" pipeline-templates/gateway/
            done

            # 生成精简版 initial-backends.yaml（仅 OpenAI，无 key）
            cat > initial-backends.yaml << 'INITDATA_EOF'
version: "2.0"
description: Default initdata - OpenAI backend (configure API key in .env)
backends:
  - id: openai
    name: OpenAI
    type: openai
    base_url: https://api.openai.com/v1
    api_key: "${OPENAI_API_KEY}"
    enabled: true
    timeout: 120
    max_retries: 3
    auto_fetch_models: true
    description: OpenAI GPT-4/GPT-3.5 API
    supported_models:
      - requested_model: gpt-4o
        actual_model: gpt-4o
        is_exact: true
        compatibility_score: 1.0
      - requested_model: gpt-4o-mini
        actual_model: gpt-4o-mini
        is_exact: true
        compatibility_score: 1.0
    capabilities:
      max_context_tokens: 128000
      features:
        - chat
        - completion
      supports_tools: true
    weight: 100
    priority: 10
INITDATA_EOF

            zip -r initdata.zip .
        )
        initdata_archive_flag=true
        print_info "默认 initdata 已生成"
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
# 参数: dist_name port initdata_path
_dist_docker_run() {
    local dist_name="${1:-minimal}"
    local port="${2:-20060}"
    local initdata_path="${3:-}"

    if [[ ! "$dist_name" =~ ^(minimal|gateway|team)$ ]]; then
        print_error "无效的发行版名称: $dist_name (支持: minimal, gateway, team)"
        exit 1
    fi

    local tag="centag-${dist_name}:latest"
    # 检查镜像是否存在，不存在则先构建
    if ! docker image inspect "$tag" >/dev/null 2>&1; then
        print_info "镜像 ${tag} 不存在，先执行构建..."
        _dist_docker_build "$dist_name" "" "$initdata_path"
    fi

    load_env

    # team 版本使用 docker-compose 启动所有服务
    if [ "$dist_name" = "team" ]; then
        print_info "启动 team 服务（含 PostgreSQL + Redis + Qdrant）..."
        cd "${PROJECT_ROOT}/config/profiles/team"
        docker compose up -d
        print_success "team 服务已启动"
        print_info "  - Centag: http://localhost:${port}"
        print_info "  - PostgreSQL: localhost:5432"
        print_info "  - Redis: localhost:6379"
        print_info "  - Qdrant: http://localhost:6333"
        cd "${PROJECT_ROOT}"
    else
        print_info "启动容器: ${tag} (端口 ${port})..."
        exec docker run --rm -it \
            --env-file "${PROJECT_ROOT}/config/secrets/.env" \
            -e CENTAG_EDITION="${dist_name}" \
            -p "${port}:20060" \
            -v "${PROJECT_ROOT}/storage:/app/storage" \
            -v "${PROJECT_ROOT}/logs:/app/logs" \
            "$tag"
    fi
}

# ── Docker 构建（已弃用，代理到 docker build）──────────────────────
# 用法: docker_build [be|fe|all]
#   be/backend  - 代理到 dist docker-build minimal
#   fe/frontend - 已弃用
#   all         - 代理到 dist docker-build gateway（默认）
docker_build() {
    check_docker
    local target="${1:-all}"
    target=$(normalize_type "$target")

    case "$target" in
        backend)
            print_warn "'./start.sh docker build backend' 已弃用，请使用: ./start.sh docker build minimal"
            _dist_docker_build "minimal" "" ""
            ;;
        frontend)
            print_error "前端独立镜像已弃用，请使用全栈镜像: ./start.sh docker build gateway"
            exit 1
            ;;
        all|*)
            print_info "代理到: ./start.sh docker build gateway"
            _dist_docker_build "gateway" "" ""
            ;;
    esac
}

# Docker 运行容器 (单容器模式)
docker_run() {
    check_docker

    # 确保镜像存在
    if ! docker image inspect centag:latest >/dev/null 2>&1; then
        print_warn "镜像不存在，正在构建..."
        docker_build
    fi

    chmod +x "${PROJECT_ROOT}/scripts/docker/docker-run.sh"
    "${PROJECT_ROOT}/scripts/docker/docker-run.sh"
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

# ============================================
# Profile 子命令：场景化一键部署
# ============================================

profile_list() {
    print_title
    echo ""
    echo -e "${CYAN}可用 Deployment Profiles:${NC}"
    echo ""

    local profiles_dir="$PROJECT_ROOT/config/profiles"
    local found=0

    for dir in "$profiles_dir"/*/; do
        local name
        name=$(basename "$dir")
        if [ "$name" = "_shared" ] || [ "$name" = "README.md" ]; then
            continue
        fi
        if [ ! -f "$dir/docker-compose.yaml" ]; then
            continue
        fi

        found=$((found + 1))
        local readme="$dir/README.md"
        local desc=""
        if [ -f "$readme" ]; then
            desc=$(grep -m1 '^[^#].*' "$readme" 2>/dev/null | sed 's/^[[:space:]]*//' | head -c 60)
        fi

        echo -e "  ${GREEN}• $name${NC}"
        if [ -n "$desc" ]; then
            echo "    $desc"
        fi
    done

    if [ "$found" -eq 0 ]; then
        print_warn "未找到可用的 Profile"
        echo "请检查 $profiles_dir 目录"
        exit 1
    fi

    echo ""
    echo -e "${YELLOW}用法示例:${NC}"
    echo "  ./start.sh profile <name> up      # 启动指定 Profile"
    echo "  ./start.sh profile <name> down    # 停止指定 Profile"
    echo "  ./start.sh profile <name> logs    # 查看日志"
    echo "  ./start.sh profile <name> status  # 查看状态"
    echo ""
}

profile_validate() {
    local name="$1"
    local profiles_dir="$PROJECT_ROOT/config/profiles"

    if [ -z "$name" ]; then
        print_error "请指定 Profile 名称"
        echo "用法: $0 profile <name> <up|down|logs|status>"
        echo ""
        profile_list
        exit 1
    fi

    if [ "$name" = "_shared" ] || [ ! -d "$profiles_dir/$name" ] || [ ! -f "$profiles_dir/$name/docker-compose.yaml" ]; then
        print_error "未知 Profile: '$name'"
        echo ""
        profile_list
        exit 1
    fi
}

profile_compose_cmd() {
    local compose_cmd="docker-compose"
    if ! command -v docker-compose >/dev/null 2>&1; then
        compose_cmd="docker compose"
    fi
    if ! $compose_cmd version >/dev/null 2>&1; then
        print_error "docker-compose 未安装"
        exit 1
    fi
    echo "$compose_cmd"
}

profile_load_stack_helpers() {
    local profile_dir="$1"
    PROFILE_PROJECT_ROOT="$PROJECT_ROOT"
    load_profile_env "$profile_dir"
    # shellcheck source=config/profiles/_shared/profile-stack.sh
    source "${PROJECT_ROOT}/config/profiles/_shared/profile-stack.sh"
}

profile_up() {
    local name="$1"
    shift || true
    profile_validate "$name"

    check_docker

    local profiles_dir="$PROJECT_ROOT/config/profiles"
    local profile_dir="$profiles_dir/$name"
    PROFILE_PROJECT_ROOT="$PROJECT_ROOT"

    if [ ! -f "$profile_dir/.env" ]; then
        if [ -f "$profile_dir/.env.example" ]; then
            print_warn "未找到 $name/.env，正在从 .env.example 复制..."
            cp "$profile_dir/.env.example" "$profile_dir/.env"
            print_info "已生成 $name/.env，请按需修改（如 API Key）后重新运行"
            echo ""
        fi
    fi

    profile_load_stack_helpers "$profile_dir"
    profile_ensure_stack_deps "$name" "$profile_dir"

    if ! docker image inspect centag:latest >/dev/null 2>&1; then
        print_warn "镜像 centag:latest 不存在，Docker Compose 将自动构建..."
    fi

    local compose_cmd
    compose_cmd=$(profile_compose_cmd)

    print_info "启动 Profile: ${GREEN}$name${NC}"
    echo ""

    cd "$profile_dir"
    profile_invoke_compose "$name" "$profile_dir" "$compose_cmd" up -d "$@"
    local exit_code=$?
    cd "$PROJECT_ROOT"

    if [ $exit_code -ne 0 ]; then
        print_error "Profile $name 启动失败"
        exit $exit_code
    fi

    print_success "Profile $name 已启动"
    echo ""
    print_info "访问地址:"
    echo "  - Centag: http://localhost:${LLM_PROXY_SERVER_PORT:-20060}"
    echo ""
    print_info "常用命令:"
    echo "  ./start.sh profile $name logs     # 查看日志"
    echo "  ./start.sh profile $name status   # 查看状态"
    echo "  ./start.sh profile $name down     # 停止服务"
    echo ""
}

profile_down() {
    local name="$1"
    shift || true
    profile_validate "$name"

    check_docker

    local profiles_dir="$PROJECT_ROOT/config/profiles"
    local profile_dir="$profiles_dir/$name"

    profile_load_stack_helpers "$profile_dir"

    local compose_cmd
    compose_cmd=$(profile_compose_cmd)

    print_info "停止 Profile: ${GREEN}$name${NC}"
    cd "$profile_dir"
    profile_invoke_compose "$name" "$profile_dir" "$compose_cmd" down --remove-orphans "$@"
    cd "$PROJECT_ROOT"

    print_success "Profile $name 已停止"
}

profile_logs() {
    local name="$1"
    shift || true
    profile_validate "$name"

    check_docker

    local profiles_dir="$PROJECT_ROOT/config/profiles"
    local profile_dir="$profiles_dir/$name"

    profile_load_stack_helpers "$profile_dir"

    local compose_cmd
    compose_cmd=$(profile_compose_cmd)

    cd "$profile_dir"
    print_info "容器状态（日志跟随中，Ctrl+C 退出）:"
    profile_invoke_compose "$name" "$profile_dir" "$compose_cmd" ps 2>/dev/null || true
    echo ""
    if [ -n "${1:-}" ]; then
        profile_invoke_compose "$name" "$profile_dir" "$compose_cmd" logs --tail=100 -f "$@"
    else
        profile_invoke_compose "$name" "$profile_dir" "$compose_cmd" logs --tail=100 -f
    fi
    cd "$PROJECT_ROOT"
}

profile_status() {
    local name="$1"
    profile_validate "$name"

    check_docker

    local profiles_dir="$PROJECT_ROOT/config/profiles"
    local profile_dir="$profiles_dir/$name"

    profile_load_stack_helpers "$profile_dir"

    local compose_cmd
    compose_cmd=$(profile_compose_cmd)

    print_info "Profile ${GREEN}$name${NC} 容器状态:"
    echo ""
    cd "$profile_dir"
    profile_invoke_compose "$name" "$profile_dir" "$compose_cmd" ps
    cd "$PROJECT_ROOT"
}

# profile_reset: 彻底重置 Profile（删除数据卷 + 重新启动），实现真正的一键运行
profile_reset() {
    local name="$1"
    shift || true
    profile_validate "$name"

    check_docker

    local profiles_dir="$PROJECT_ROOT/config/profiles"
    local profile_dir="$profiles_dir/$name"
    local compose_cmd
    compose_cmd=$(profile_compose_cmd)

    print_warn "即将彻底重置 Profile: ${GREEN}$name${NC}（所有数据卷将被删除）"
    read -r -p "确认继续? [y/N] " confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        print_info "已取消"
        exit 0
    fi

    profile_load_stack_helpers "$profile_dir"
    profile_ensure_stack_deps "$name" "$profile_dir"

    # 1. 停止并删除所有卷
    print_info "停止并删除数据卷..."
    cd "$profile_dir"
    profile_invoke_compose "$name" "$profile_dir" "$compose_cmd" down -v --remove-orphans

    # 2. 拉取 Ollama 模型（按 profile 模式）
    case "$name" in
        agent-memory)
            if profile_uses_stack_network "$name" "$profile_dir"; then
                print_info "通过 stack Ollama 拉取模型（首次需几分钟）..."
                profile_ensure_stack_deps "$name" "$profile_dir"
                profile_pull_stack_ollama_models llama3.1 qwen2.5 bge-m3
            else
                print_info "启动内嵌 Ollama 并拉取模型（embedded 模式）..."
                profile_invoke_compose "$name" "$profile_dir" "$compose_cmd" up -d ollama
                local ollama_container="centag-agent-memory-ollama"
                print_info "拉取 llama3.1..."
                docker exec "$ollama_container" ollama pull llama3.1 2>/dev/null || print_warn "llama3.1 拉取可能失败，请检查网络"
                print_info "拉取 qwen2.5..."
                docker exec "$ollama_container" ollama pull qwen2.5 2>/dev/null || print_warn "qwen2.5 拉取可能失败，请检查网络"
                print_info "拉取 bge-m3（Mem0 embedding 使用）..."
                docker exec "$ollama_container" ollama pull bge-m3 2>/dev/null || print_warn "bge-m3 拉取可能失败，请检查网络"
            fi
            print_info "模型拉取完成"
            ;;
        gateway)
            if profile_uses_stack_network "$name" "$profile_dir"; then
                print_info "通过 stack Ollama 拉取模型（首次需几分钟）..."
                profile_ensure_stack_deps "$name" "$profile_dir"
                profile_pull_stack_ollama_models llama3.1
            fi
            ;;
        cached)
            if profile_uses_stack_network "$name" "$profile_dir"; then
                print_info "通过 stack Ollama 拉取 embedding 模型..."
                profile_ensure_stack_deps "$name" "$profile_dir"
                profile_pull_stack_ollama_models bge-m3
            fi
            ;;
    esac

    # 3. 启动全部服务（reset 时强制重建应用镜像，确保插件代码变更生效）
    print_info "启动 Profile: ${GREEN}$name${NC}"
    profile_invoke_compose "$name" "$profile_dir" "$compose_cmd" up -d --build
    cd "$PROJECT_ROOT"

    print_success "Profile $name 已重置并启动"
    echo ""
    print_info "访问地址: http://localhost:${LLM_PROXY_SERVER_PORT:-20060}"
    print_info "查看日志: ./start.sh profile $name logs"
    print_info "查看状态: ./start.sh profile $name status"
    echo ""
    print_info "提示: 首次启动后请检查日志确认 '首轮启动' 和 initdata 加载成功"
}

# Docker Compose 启动（本仓库 compose 仅含 centag；中间件见 deploy/stack）
docker_up() {
    if [ -n "${1:-}" ]; then
        print_warn "已忽略多余参数「$1」：./start.sh docker up 不再接受 profile，中间件请使用 deploy/stack。"
    fi
    check_docker

    if [ ! -f "$PROJECT_ROOT/config/secrets/.env" ]; then
        print_warn "未找到 config/secrets/.env，正在自动生成认证配置..."
        "${PROJECT_ROOT}/scripts/ops/generate-secrets.sh" --same-password
    fi

    load_env

    if ! docker image inspect centag:latest >/dev/null 2>&1; then
        print_warn "主服务镜像 centag:latest 不存在，正在构建..."
        docker_build
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

    print_info "Docker Compose 说明:"
    echo "  本仓库 deploy/docker/docker-compose.yaml 仅包含 centag 应用容器。"
    echo "  PostgreSQL、Redis、Elasticsearch、Mem0 等请使用: ./start.sh stack …"
    echo ""
    print_info "示例:"
    echo "  ./start.sh docker up              # 启动 Centag 容器（默认）"
    echo "  ./start.sh stack start base       # 启动基础中间件"
    echo ""

    cd docker

    docker_compose_invoke "$compose_cmd" up -d
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
    if ! docker image inspect centag:latest >/dev/null 2>&1; then
        print_warn "主服务镜像 centag:latest 不存在，正在构建..."
        docker_build
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
        eval "GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags '$_FULL_FEATURE_TAGS' -o \"$BIN_DIR/centag\" ./cmd/centag/main.go"
    else
        print_info "正在编译为 linux/amd64 架构..."
        mkdir -p "$BIN_DIR"
        eval "CGO_ENABLED=0 go build -tags '$_FULL_FEATURE_TAGS' -o \"$BIN_DIR/centag\" ./cmd/centag/main.go"
    fi

    if [ $? -eq 0 ]; then
        # 验证编译结果（macOS 和 Linux 的 file 命令输出格式不同）
        local compiled_arch=$(file -b "$BIN_DIR/centag" 2>/dev/null || echo "")
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
    echo "  1. 本地修改代码 -> go build -o bin/server/centag"
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
            if [ -f "$BIN_DIR/centag" ]; then
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
    docker rmi centag:latest 2>/dev/null || true

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
    if ! docker image inspect centag:latest >/dev/null 2>&1; then
        print_warn "镜像不存在，正在构建..."
        docker_build
    fi

    # 创建打包目录
    mkdir -p "$package_dir"

    # 导出主服务镜像
    print_info "导出主服务镜像..."
    if docker save -o "${package_dir}/centag-image.tar" "centag:latest"; then
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
    cp scripts/docker/docker-run.sh "${package_dir}/docker-run.sh"
    chmod +x "${package_dir}/docker-run.sh"
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
echo ""
echo "或使用单容器模式:"
echo "  ./docker-run.sh"
LOADEOF
    chmod +x "${package_dir}/load-images.sh"

    # 创建 README
    cat > "${package_dir}/README.md" << 'READMEEOF'
# Centag Docker 部署包

## 目录结构

- `centag-image.tar` - 主服务镜像
- `docker-compose.yaml` - Docker Compose 配置文件（仅 centag 服务）
- `docker-run.sh` - 单容器启动脚本
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
    echo -e "  ${GREEN}run${NC}      <be|fe|personal|minimal> [--launcher]  运行服务"
    echo -e "  ${GREEN}daemon${NC}                           后台守护进程模式（自动重启）"
    echo -e "  ${GREEN}debug${NC} [--minimal|--team]         开发模式（默认 personal）+ 前端热重载"
    echo -e "  ${GREEN}stop${NC}     <be|fe>               停止服务"
    echo -e "  ${GREEN}status${NC}                           查看服务状态"
    echo -e "  ${GREEN}logs${NC}                             查看服务日志"
    echo ""

    # ── 构建 ──
    echo -e "  ${CYAN}── 构建 ──────────────────────────────────────────────${NC}"
    echo -e "  ${GREEN}build${NC}    <all|be|fe>             构建项目（开发用）"
    echo -e "  ${GREEN}build${NC}    <personal|minimal|team> [--launcher]  构建发行版"
    echo -e "  ${GREEN}docker${NC}   build <minimal|gateway|team>   构建 Docker 镜像"
    echo -e "  ${GREEN}docker${NC}   run   <minimal|gateway|team>   运行 Docker 容器"
    echo -e "  ${GREEN}clean${NC}                            清理构建产物"
    echo -e "  ${GREEN}pack${NC}     [--upload]              打包服务端更新包"
    echo -e "  ${GREEN}test${NC}                             运行单元测试"
    echo ""

    # ── 环境 ──
    echo -e "  ${CYAN}── 环境配置 ──────────────────────────────────────────${NC}"
    echo -e "  ${GREEN}init${NC}                             初始化开发环境"
    echo -e "  ${GREEN}env${NC}      gen [--force]           生成密钥配置文件"
    echo ""

    # ── 场景化部署 ──
    echo -e "  ${CYAN}── 场景化部署 (Profile) ─────────────────────────────${NC}"
    echo -e "  ${GREEN}profile${NC}  list                       列出可用部署模式"
    echo -e "  ${GREEN}profile${NC}  <name> up|down|reset      启动/停止/重置场景"
    echo -e "  ${GREEN}profile${NC}  <name> logs|status        查看日志/状态"
    echo ""

    # ── Stack & Docker ──
    echo -e "  ${CYAN}── Stack 中间件 & Docker ────────────────────────────${NC}"
    echo -e "  ${GREEN}stack${NC}    <start|stop|status|...>   中间件编排 (PG/Redis/ES/...)"
    echo -e "  ${GREEN}docker${NC}   <up|down|build|...>       Docker Compose 管理"
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
        dist)          _help_dist ;;
        run)           _help_run ;;
        daemon)        _help_daemon ;;
        debug)         _help_debug ;;
        stop)          _help_stop ;;
        status)        _help_status ;;
        logs)          _help_logs ;;
        clean)         _help_clean ;;
        profile)       _help_profile ;;
        stack)         _help_stack ;;
        docker)        _help_docker ;;
        webui)         _help_webui ;;
        pack)          _help_pack ;;
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
    echo -e "  ./start.sh build <目标> [--launcher]"
    echo ""
    echo -e "${CYAN}开发构建:${NC}"
    echo -e "  ${GREEN}all${NC}             构建全部（后端 + 生产版前端） 【默认】"
    echo -e "  ${GREEN}be${NC} | backend     仅构建后端服务"
    echo -e "  ${GREEN}fe${NC} | frontend   构建 Vue 前端"
    echo ""
    echo -e "${CYAN}发行版构建:${NC}"
    echo -e "  ${GREEN}personal${NC}  个人全功能（= gateway 发行包，默认 SQLite）"
    echo -e "  ${GREEN}minimal${NC}   轻量单机（文件配置，无 DB）"
    echo -e "  ${GREEN}team${NC}      团队版（中间件外置：PG/向量等）"
    echo -e "  ${GREEN}gateway${NC}   personal 的别名（兼容旧命令）"
    echo ""
    echo -e "${CYAN}辅助选项:${NC}"
    echo -e "  ${GREEN}--launcher${NC}    额外构建当前系统的桌面启动器（仅 personal/minimal）"
    echo -e "             自动识别 darwin / linux / windows（GOOS/GOARCH）"
    echo -e "             ${YELLOW}team 不支持 --launcher${NC}"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh build personal           # 普通个人版服务"
    echo -e "  ./start.sh build personal --launcher    # 个人版 + 桌面启动器"
    echo -e "  ./start.sh build minimal --launcher"
    echo -e "  ./start.sh build be"
    echo ""
    echo -e "${YELLOW}提示:${NC} Docker 镜像请使用: ./start.sh docker build <minimal|gateway|team>"
}

_help_dist() {
    echo -e "${GREEN}命令: dist${NC} ${YELLOW}(已弃用)${NC}"
    echo -e "       ${YELLOW}请迁移到 build / docker 命令${NC}"
    echo ""
    echo -e "${CYAN}迁移映射:${NC}"
    echo -e "  ./start.sh dist build <ed>         →  ${GREEN}./start.sh build <ed>${NC}"
    echo -e "  ./start.sh dist run <ed>           →  ${GREEN}./start.sh docker run <ed>${NC}"
    echo -e "  ./start.sh dist docker-build <ed>  →  ${GREEN}./start.sh docker build <ed>${NC}"
    echo -e "  ./start.sh dist docker-run <ed>    →  ${GREEN}./start.sh docker run <ed>${NC}"
}

_help_run() {
    echo -e "${GREEN}命令: run${NC}"
    echo -e "       ${YELLOW}启动服务（前台运行）${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh run <服务> [--launcher]"
    echo ""
    echo -e "${CYAN}服务:${NC}"
    echo -e "  ${GREEN}be${NC} | backend        启动后端服务 (端口 20060)"
    echo -e "  ${GREEN}fe${NC} | frontend      启动 Vue 开发服务器 (端口 5173)"
    echo -e "  ${GREEN}personal${NC} | gateway  个人版发行包（前台）"
    echo -e "  ${GREEN}minimal${NC}             minimal 发行包（前台）"
    echo -e "  ${GREEN}all${NC}                全部（需两个终端分别启动 be/fe）"
    echo ""
    echo -e "${CYAN}辅助选项:${NC}"
    echo -e "  ${GREEN}--launcher${NC}    以桌面启动器启动（菜单/托盘 + 浏览器；仅 personal/minimal）"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh run be"
    echo -e "  ./start.sh run personal            # 普通个人版服务"
    echo -e "  ./start.sh run personal --launcher     # 启动器方式"
    echo -e "  ./start.sh run minimal --launcher"
    echo ""
    echo -e "${YELLOW}注意:${NC} 开发模式需两个终端: 终端1 run be, 终端2 run fe"
}

_help_daemon() {
    echo -e "${GREEN}命令: daemon${NC}"
    echo -e "       ${YELLOW}后台守护进程模式（自动重启）${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh daemon [子命令]"
    echo ""
    echo -e "${CYAN}子命令:${NC}"
    echo -e "  ${GREEN}(无)${NC}               启动后端守护进程"
    echo -e "  ${GREEN}stop${NC}              停止守护进程"
    echo -e "  ${GREEN}debug${NC}             守护进程调试模式"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh daemon            # 启动守护进程"
    echo -e "  ./start.sh daemon stop       # 停止守护进程"
}

_help_debug() {
    echo -e "${GREEN}命令: debug${NC}"
    echo -e "       ${YELLOW}开发调试模式${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh debug              # 默认 personal（对齐 gateway）"
    echo -e "  ./start.sh debug --minimal    # minimal 精简版"
    echo -e "  ./start.sh debug --team       # team 团队版"
    echo ""
    echo -e "${CYAN}选项:${NC}"
    echo -e "  ${GREEN}（默认）${NC}     CENTAG_EDITION=personal + 全功能二进制（SQLite 个人网关）"
    echo -e "  ${GREEN}--minimal${NC}   精简 WebUI + centag-minimal（edition=minimal）"
    echo -e "  ${GREEN}--team${NC}      全功能二进制 + CENTAG_EDITION=team（多租户/计费面）"
    echo ""
    echo -e "${CYAN}说明:${NC}"
    echo -e "  默认与 gateway Profile 一致（personal）；--team / --minimal 互斥。"
    echo -e "  均支持：后端 debug 日志 + 前端文件变更自动同步 + 一键 Ctrl+C 停止。"
}

_help_stop() {
    echo -e "${GREEN}命令: stop${NC}"
    echo -e "       ${YELLOW}停止运行中的服务${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh stop <目标>"
    echo ""
    echo -e "${CYAN}目标:${NC}"
    echo -e "  ${GREEN}be${NC} | backend     停止后端服务"
    echo -e "  ${GREEN}fe${NC} | frontend   停止 Vue 开发服务器"
    echo -e "  ${GREEN}all${NC}             停止所有服务 【默认】"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh stop              # 停止所有服务"
    echo -e "  ./start.sh stop be           # 仅停止后端"
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
    echo -e "    tail -f bin/server/logs/centag.log"
    echo -e "    tail -f bin/server/storage/logs/*.log"
}

_help_clean() {
    echo -e "${GREEN}命令: clean${NC}"
    echo -e "       ${YELLOW}清理构建产物${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh clean"
    echo ""
    echo -e "${CYAN}说明:${NC}"
    echo -e "  清理 bin/ 目录下的编译产物。清理后需要重新 build。"
}

_help_profile() {
    echo -e "${GREEN}命令: profile${NC}"
    echo -e "       ${YELLOW}场景化一键部署${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh profile <名称> <子命令> [参数]"
    echo ""
    echo -e "${CYAN}子命令:${NC}"
    echo -e "  ${GREEN}list${NC}                      列出所有可用部署模式"
    echo -e "  ${GREEN}<name> up${NC}   [--build]     启动指定场景"
    echo -e "  ${GREEN}<name> down${NC} [--volumes]   停止指定场景"
    echo -e "  ${GREEN}<name> reset${NC}             彻底重置（删卷 + 拉模型 + 启动）"
    echo -e "  ${GREEN}<name> logs${NC} [service]    查看日志"
    echo -e "  ${GREEN}<name> status${NC}            查看容器状态"
    echo ""
    echo -e "${CYAN}可用场景:${NC}"
    echo -e "  ${GREEN}gateway${NC}      个人全功能（默认 SQLite 单容器；可外接中间件）"
    echo -e "  ${GREEN}cached${NC}       缓存加速（PG + pgvector，精确+语义缓存）"
    echo -e "  ${GREEN}agent-memory${NC}  智能体记忆（Mem0 + Sandbox + PG + Qdrant + Ollama）"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh profile list"
    echo -e "  ./start.sh profile gateway up"
    echo -e "  ./start.sh profile agent-memory up --build"
    echo -e "  ./start.sh profile gateway logs"
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
    echo -e "       ${YELLOW}Docker 镜像构建 / Compose 管理${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh docker <子命令> [参数]"
    echo ""
    echo -e "${CYAN}发行版操作:${NC}"
    echo -e "  ${GREEN}build${NC} <minimal|gateway|team>              构建 Docker 镜像"
    echo -e "  ${GREEN}run${NC}   <minimal|gateway|team> [port]       运行 Docker 容器"
    echo ""
    echo -e "${CYAN}Compose 操作:${NC}"
    echo -e "  ${GREEN}up${NC}                   启动 Centag 容器"
    echo -e "  ${GREEN}down${NC}                 停止并清理容器"
    echo -e "  ${GREEN}logs${NC} [service]       查看容器日志"
    echo -e "  ${GREEN}status${NC}               查看容器状态"
    echo -e "  ${GREEN}restart${NC} [service]    重启容器"
    echo -e "  ${GREEN}debug${NC}                启动 Debug 模式（挂载本地 bin）"
    echo -e "  ${GREEN}clean${NC}                清理所有容器/镜像/数据卷"
    echo -e "  ${GREEN}pack${NC}                 打包镜像为 tar.gz"
    echo ""
    echo -e "${YELLOW}注意:${NC} docker compose 仅编排 centag 容器；中间件请用 stack 命令"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh docker build minimal   # 构建 minimal 镜像"
    echo -e "  ./start.sh docker run gateway     # 运行 gateway 容器"
    echo -e "  ./start.sh docker up              # Compose 启动"
    echo -e "  ./start.sh docker logs            # 查看日志"
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
    echo -e "${GREEN}命令: pack${NC}"
    echo -e "       ${YELLOW}打包服务端更新包${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo -e "  ./start.sh pack [--upload]"
    echo ""
    echo -e "${CYAN}选项:${NC}"
    echo -e "  ${GREEN}--upload${NC}   打包并上传热更新（需设置认证 Token）"
    echo ""
    echo -e "${CYAN}认证优先级:${NC}"
    echo -e "  CENTAG_UPDATE_TOKEN > LLM_PROXY_DEFAULT_ADMIN_API_KEY"
    echo -e "  > LLM_PROXY_ADMIN_API_KEY > CENTAG_API_KEY"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  ./start.sh pack"
    echo -e "  CENTAG_UPDATE_TOKEN=xxx ./start.sh pack --upload"
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
                        kill_backend_port_or_exit || continue
                        [ ! -f "$BIN_DIR/centag" ] && build backend >/dev/null 2>&1
                        cd "$BIN_DIR"
                        nohup ./centag > logs/centag.log 2>&1 &
                        cd "$PROJECT_ROOT"
                        sleep 2
                        if lsof -ti ":$BACKEND_PORT" >/dev/null 2>&1; then
                            print_success "✅ 服务已启动 (后台运行)"
                            print_info "日志文件: bin/logs/centag.log"
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
                        kill_backend_port_or_exit || continue
                        [ ! -f "$BIN_DIR/centag" ] && build backend >/dev/null 2>&1
                        cd "$BIN_DIR"
                        nohup ./centag > logs/centag.log 2>&1 &
                        cd "$PROJECT_ROOT"
                        sleep 2
                        if lsof -ti ":$BACKEND_PORT" >/dev/null 2>&1; then
                            print_success "✅ 后端服务已启动 (后台运行)"
                            print_info "日志文件: bin/logs/centag.log"
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
                            kill_backend_port_or_exit || continue
                            [ ! -f "$BIN_DIR/centag" ] && build backend >/dev/null 2>&1
                            print_test_examples
                            cd "$BIN_DIR"
                            ./centag
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
                        kill_backend_port_or_exit || continue
                        [ ! -f "$BIN_DIR/centag" ] && build backend >/dev/null 2>&1
                        print_test_examples
                        cd "$BIN_DIR"
                        print_info "按 Ctrl+C 停止服务"
                        ./centag
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
                        kill_backend_port_or_exit || continue
                        [ ! -f "$BIN_DIR/centag" ] && build backend >/dev/null 2>&1
                        print_test_examples
                        cd "$BIN_DIR"
                        print_info "按 Ctrl+C 停止服务"
                        ./centag
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
                            kill_backend_port_or_exit || continue
                            [ ! -f "$BIN_DIR/centag" ] && build backend >/dev/null 2>&1
                            print_test_examples
                            cd "$BIN_DIR"
                            print_info "按 Ctrl+C 停止服务"
                            ./centag
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
            print_info "默认密码: JEAofRz0WteQOsWI"
            print_info "或运行 ./start.sh env gen 生成新凭据"
            echo ""
        fi
    else
        print_warn "未找到密钥配置文件"
        print_info "默认用户名: admin"
        print_info "默认密码: JEAofRz0WteQOsWI"
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
    echo "  ./start.sh debug           # personal（默认，对齐 gateway）+ 前端热重载"
    echo "  ./start.sh debug --minimal # minimal 精简版"
    echo "  ./start.sh debug --team    # team 团队版"
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

    # 如果参数中包含 --help/-h，显示该命令的详细帮助
    for arg in "$@"; do
        if [ "$arg" = "--help" ] || [ "$arg" = "-h" ]; then
            show_command_help "$cmd"
            exit 0
        fi
    done

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
            local with_launcher=false
            local unknown_args=()
            for arg in "$@"; do
                case "$arg" in
                    --launcher) with_launcher=true ;;
                    *)
                        unknown_args+=("$arg")
                        ;;
                esac
            done
            if [ ${#unknown_args[@]} -gt 0 ]; then
                print_error "未知 build 参数: ${unknown_args[*]}"
                echo "用法: $0 build <目标> [--launcher]"
                exit 1
            fi
            target=$(normalize_type "$target")
            case "$target" in
                backend|be)
                    if $with_launcher; then
                        print_error "--launcher 不能用于 build be"
                        exit 1
                    fi
                    build backend
                    ;;
                frontend|fe|vue)
                    if $with_launcher; then
                        print_error "--launcher 不能用于 build fe"
                        exit 1
                    fi
                    build webui
                    ;;
                all)
                    if $with_launcher; then
                        print_error "--launcher 不能用于 build all；请用: build personal --launcher"
                        exit 1
                    fi
                    build all
                    ;;
                personal|gateway|minimal|team)
                    if $with_launcher; then
                        build_with_launcher "$target"
                    else
                        build dist "$(edition_to_dist "$target")"
                    fi
                    ;;
                *)
                    print_error "未知构建目标: '$target'"
                    echo "支持的构建目标: all, be, fe, personal, minimal, team, gateway"
                    echo "启动器: ./start.sh build personal --launcher  或  ./start.sh build minimal --launcher"
                    exit 1
                    ;;
            esac
            ;;

        # ── 发行版（已弃用，代理到 build / docker）───────────────────────
        dist)
            local subcmd="${1:-}"
            if [ -n "$subcmd" ]; then shift; fi
            case "$subcmd" in
                build)
                    local dist_name="${1:-minimal}"
                    print_warn "已弃用: './start.sh dist build' → 请用 './start.sh build $dist_name'"
                    build dist "$dist_name"
                    ;;
                run)
                    local dist_name="${1:-minimal}"
                    print_warn "已弃用: './start.sh dist run' → 请用 './start.sh docker run $dist_name'"
                    local bin_path="$PROJECT_ROOT/bin/server/centag-${dist_name}"
                    if [ ! -f "$bin_path" ]; then
                        print_error "未找到发行版二进制: $bin_path"
                        print_info "请先执行: $0 build $dist_name"
                        exit 1
                    fi
                    load_env
                    print_info "启动 Centag ${dist_name}..."
                    exec "$bin_path"
                    ;;
                docker-build)
                    local dist_name="${1:-minimal}"
                    print_warn "已弃用: './start.sh dist docker-build' → 请用 './start.sh docker build $dist_name'"
                    _dist_docker_build "$dist_name" "" ""
                    ;;
                docker-run)
                    local dist_name="${1:-minimal}"
                    print_warn "已弃用: './start.sh dist docker-run' → 请用 './start.sh docker run $dist_name'"
                    _dist_docker_run "$dist_name" "" ""
                    ;;
                *)
                    if [ -n "$subcmd" ]; then
                        print_warn "已弃用: './start.sh dist $subcmd' → 请用 './start.sh build $subcmd'"
                        build dist "$subcmd"
                    else
                        print_error "请指定子命令或发行版名称"
                        print_info "用法: $0 dist <build|run|docker-build|docker-run> <minimal|gateway|team>"
                        print_info "推荐: $0 build <minimal|gateway|team>  或  $0 docker <build|run> <minimal|gateway|team>"
                        exit 1
                    fi
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

        # ── 运行（后台）────────────────────────────────────────────────
        run)
            local svc="${1:-backend}"
            shift || true
            svc=$(normalize_type "$svc")
            case "$svc" in
                backend|be)
                    start_backend_foreground
                    ;;
                frontend|fe|vue)
                    start_frontend_dev
                    ;;
                personal|gateway|minimal)
                    run_edition "$svc" "$@"
                    ;;
                team)
                    print_error "team 请用 Docker/Profile 运行；托盘不支持 team"
                    echo "示例: ./start.sh docker run team  或  ./start.sh profile team up"
                    exit 1
                    ;;
                all)
                    print_warn "⚠️  'all' 模式说明："
                    echo ""
                    show_all_mode_info
                    ;;
                *)
                    print_error "未知运行目标: $svc"
                    echo "支持的运行目标: be, fe, personal, minimal"
                    echo "启动器: ./start.sh run personal --launcher  或  ./start.sh run minimal --launcher"
                    echo ""
                    show_all_mode_info
                    exit 1
                    ;;
            esac
            ;;

        # ── 守护进程 ────────────────────────────────────────────────────
        daemon)
            local daemon_sub="${1:-}"
            daemon_sub=$(normalize_type "$daemon_sub")
            case "$daemon_sub" in
                stop)
                    daemon-stop
                    ;;
                debug)
                    daemon-debug
                    ;;
                backend|be|"")
                    daemon
                    ;;
                *)
                    print_error "未知 daemon 子命令: '$daemon_sub'"
                    echo "用法: $0 daemon [be|stop|debug]"
                    exit 1
                    ;;
            esac
            ;;

        # ── 调试（后端 debug 模式 + 前端热重载）────────────────────────
        debug)
            debug "$@"
            ;;

        # ── 停止 ───────────────────────────────────────────────────────
        stop)
            local svc="${1:-all}"
            svc=$(normalize_type "$svc")
            case "$svc" in
                backend|be)
                    stop
                    ;;
                frontend|fe|vue)
                    kill_port 5173
                    print_success "Vue dev server stopped"
                    ;;
                all)
                    stop
                    kill_port 5173
                    print_success "All services stopped"
                    ;;
                *)
                    print_error "未知停止目标: $svc"
                    exit 1
                    ;;
            esac
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
            clean
            ;;

        # ── Stack 中间件（加载 deploy/stack/lib）────────────────────
        stack)
            # main() 已 shift 掉顶层命令名，此处勿再 shift
            stack_cmd "$@"
            ;;

        # ── Deployment Profiles（场景化一键部署）────────────────────────
        profile)
            local profile_name="${1:-}"
            local profile_cmd="${2:-}"

            if [ "$profile_name" = "list" ] || [ -z "$profile_name" ]; then
                profile_list
                exit 0
            fi

            shift || true
            profile_cmd="${1:-}"
            shift || true

            case "$profile_cmd" in
                up)
                    profile_up "$profile_name" "$@"
                    ;;
                down)
                    profile_down "$profile_name" "$@"
                    ;;
                reset)
                    profile_reset "$profile_name" "$@"
                    ;;
                logs)
                    profile_logs "$profile_name" "$@"
                    ;;
                status)
                    profile_status "$profile_name"
                    ;;
                *)
                    print_error "未知 profile 子命令: '$profile_cmd'"
                    echo "用法: $0 profile <name> <up|down|reset|logs|status>"
                    echo ""
                    profile_list
                    exit 1
                    ;;
            esac
            ;;

        # ── Docker ─────────────────────────────────────────────────────
         docker)
            local docker_cmd="${1:-}"
            shift || true
            case "$docker_cmd" in
                # ── 发行版 Docker 操作（新）──
                build)
                    local edition="${1:-}"
                    case "$edition" in
                        minimal|gateway|team)
                            _dist_docker_build "$edition" "" ""
                            ;;
                        *)
                            # 向后兼容: docker build all/be/fe
                            docker_build "$edition"
                            ;;
                    esac
                    ;;
                run)
                    local edition="${1:-minimal}"
                    local port="${2:-20060}"
                    local initdata_path=""
                    # 检查 --initdata 参数
                    shift 2 || true
                    while [ $# -gt 0 ]; do
                        case "$1" in
                            --initdata) initdata_path="$2"; shift 2 ;;
                            *) shift ;;
                        esac
                    done
                    _dist_docker_run "$edition" "$port" "$initdata_path"
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
                    echo "  $0 docker build <minimal|gateway|team>              构建 Docker 镜像"
                    echo "  $0 docker run   <minimal|gateway|team> [port]       运行 Docker 容器"
                    echo ""
                    echo "Compose 操作:"
                    echo "  $0 docker up|down|logs|status|clean|pack|debug|restart"
                    exit 1
                    ;;
            esac
            ;;

        # ── 打包 ─────────────────────────────────────────────────────
        pack)
            pack "$@"
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
