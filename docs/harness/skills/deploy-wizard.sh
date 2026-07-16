#!/usr/bin/env bash
# ============================================================================
# Centag Deploy Wizard - 预置部署脚本
# ============================================================================
# 本脚本封装部署向导的核心逻辑，供 Agent skill 调用
# 用法: bash deploy-wizard.sh [选项]
#
# 选项:
#   --mode <mode>       部署模式 (gateway|cached|agent-memory|docker|local)
#   --port <port>       服务端口 (默认 20060)
#   --admin-user <user> 管理员用户名 (默认 admin)
#   --admin-pass <pass> 管理员密码 (交互式输入)
#   --pg-host <host>    PostgreSQL 地址 (cached/agent-memory 模式)
#   --pg-port <port>    PostgreSQL 端口 (默认 5432)
#   --pg-user <user>    PostgreSQL 用户名 (默认 llmproxy)
#   --pg-pass <pass>    PostgreSQL 密码 (cached/agent-memory 模式)
#   --pg-db <db>        PostgreSQL 数据库名 (默认 centag)
#   --ollama-host <url> Ollama 地址
#   --no-interact       非交互模式 (使用默认值)
#   --help              显示帮助信息
# ============================================================================

set -euo pipefail

# ============================================================================
# 颜色定义
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ============================================================================
# 默认值
# ============================================================================
DEPLOY_MODE=""
PORT=20060
ADMIN_USERNAME="admin"
ADMIN_PASSWORD=""
PG_HOST="localhost"
PG_PORT=5432
PG_USER="llmproxy"
PG_PASSWORD=""
PG_DATABASE="centag"
OLLAMA_HOST="http://localhost:11434"
OLLAMA_ENABLED=false
NO_INTERACT=false

# ============================================================================
# 辅助函数
# ============================================================================
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_step() {
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

read_input() {
    local prompt="$1"
    local default="${2:-}"
    local result
    
    if [ -n "$default" ]; then
        read -rp "$(echo -e "${BLUE}$prompt${NC} (默认 $default): ")" result
        echo "${result:-$default}"
    else
        read -rp "$(echo -e "${BLUE}$prompt${NC}: ")" result
        echo "$result"
    fi
}

read_password() {
    local prompt="$1"
    local result
    
    read -rsp "$(echo -e "${BLUE}$prompt${NC}: ")" result
    echo
    echo "$result"
}

# ============================================================================
# 帮助信息
# ============================================================================
show_help() {
    cat << EOF
Centag Deploy Wizard - 部署向导脚本

用法: bash deploy-wizard.sh [选项]

选项:
  --mode <mode>       部署模式
                      gateway      - 最小网关，SQLite，2分钟启动
                      cached       - 缓存加速，PostgreSQL
                      agent-memory - Agent全栈，Mem0+Ollama
                      docker       - 单容器快速部署
                      local        - 本地开发调试

  --port <port>       服务端口 (默认 20060)
  --admin-user <user> 管理员用户名 (默认 admin)
  --admin-pass <pass> 管理员密码

  --pg-host <host>    PostgreSQL 地址 (仅 cached/agent-memory)
  --pg-port <port>    PostgreSQL 端口 (默认 5432)
  --pg-user <user>    PostgreSQL 用户名 (默认 llmproxy)
  --pg-pass <pass>    PostgreSQL 密码 (仅 cached/agent-memory)
  --pg-db <db>        PostgreSQL 数据库名 (默认 centag)

  --ollama-host <url> Ollama 地址 (默认 http://localhost:11434)
  --no-interact       非交互模式 (使用默认值)
  --help              显示此帮助信息

示例:
  # 交互式部署 gateway 模式
  bash deploy-wizard.sh --mode gateway

  # 非交互式部署 cached 模式
  bash deploy-wizard.sh --mode cached --admin-pass mypassword --pg-pass pgpassword --no-interact

  # 指定端口部署
  bash deploy-wizard.sh --mode gateway --port 8080
EOF
}

# ============================================================================
# 环境检测
# ============================================================================
detect_environment() {
    log_step "Step 1: 环境检测"

    # 检测项目根目录
    if [ ! -f start.sh ]; then
        log_error "请在 Centag 项目根目录运行此命令"
        log_info "提示: cd /path/to/centag"
        exit 1
    fi

    # 检测操作系统
    OS=$(uname -s)
    log_info "操作系统: $OS"

    # 检测 Docker
    if command -v docker &> /dev/null; then
        DOCKER_VERSION=$(docker --version | awk '{print $3}' | tr -d ',')
        log_success "Docker: $DOCKER_VERSION"
        DOCKER_AVAILABLE=true
    else
        log_warning "Docker: 不可用"
        DOCKER_AVAILABLE=false
    fi

    # 检测 Docker Compose
    if command -v docker &> /dev/null && docker compose version &> /dev/null; then
        COMPOSE_VERSION=$(docker compose version | awk '{print $4}')
        log_success "Docker Compose: $COMPOSE_VERSION"
        COMPOSE_AVAILABLE=true
    else
        log_warning "Docker Compose: 不可用"
        COMPOSE_AVAILABLE=false
    fi

    # 检测 Go
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | awk '{print $3}' | tr -d 'go')
        log_success "Go: $GO_VERSION"
        GO_AVAILABLE=true
    else
        log_warning "Go: 不可用"
        GO_AVAILABLE=false
    fi

    # 检测端口占用
    if command -v lsof &> /dev/null; then
        if lsof -i :$PORT &> /dev/null; then
            log_warning "端口 $PORT: 已被占用"
            PORT_OCCUPIED=true
        else
            log_success "端口 $PORT: 空闲"
            PORT_OCCUPIED=false
        fi
    fi

    # 检测磁盘空间
    DISK_SPACE=$(df -h . | awk 'NR==2 {print $4}')
    log_info "磁盘空间: $DISK_SPACE 可用"
}

# ============================================================================
# 模式选择
# ============================================================================
select_mode() {
    if [ -n "$DEPLOY_MODE" ]; then
        log_info "部署模式: $DEPLOY_MODE (通过参数指定)"
        return
    fi

    log_step "Step 2: 选择部署模式"

    echo -e "${BLUE}请选择部署模式：${NC}"
    echo ""
    
    local options=()
    
    if [ "$DOCKER_AVAILABLE" = true ] && [ "$COMPOSE_AVAILABLE" = true ]; then
        options+=("gateway" "cached" "agent-memory" "docker")
        echo "  1. gateway      - 最小网关，SQLite，2分钟启动"
        echo "  2. cached       - 缓存加速，PostgreSQL，降本80%+"
        echo "  3. agent-memory - Agent全栈，Mem0+Ollama，需要2GB内存"
        echo "  4. docker       - 单容器快速部署，SQLite"
    fi
    
    if [ "$GO_AVAILABLE" = true ]; then
        options+=("local")
        local local_num=$((${#options[@]}))
        echo "  $local_num. local        - 本地开发调试，需要Go环境"
    fi

    if [ ${#options[@]} -eq 0 ]; then
        log_error "没有可用的部署模式"
        log_info "请安装 Docker 或 Go 后重试"
        exit 1
    fi

    echo ""
    local choice=$(read_input "请输入选项编号 (1-${#options[@]})" "1")
    
    # 验证输入
    if [[ "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -ge 1 ] && [ "$choice" -le ${#options[@]} ]; then
        DEPLOY_MODE=${options[$((choice-1))]}
    else
        log_error "无效选项"
        exit 1
    fi

    log_success "已选择部署模式: $DEPLOY_MODE"
}

# ============================================================================
# 端口配置
# ============================================================================
configure_port() {
    if [ "$PORT" != "20060" ]; then
        log_info "服务端口: $PORT (通过参数指定)"
        return
    fi

    log_step "Step 3: 端口配置"

    PORT=$(read_input "请输入服务端口" "20060")

    # 检查端口占用
    if command -v lsof &> /dev/null; then
        if lsof -i :$PORT &> /dev/null; then
            log_warning "端口 $PORT 已被占用"
            local stop_existing=$(read_input "是否停止占用该端口的服务？(y/n)" "y")
            if [ "$stop_existing" = "y" ]; then
                log_info "正在停止占用端口 $PORT 的服务..."
                # 尝试停止 Centag 服务
                if [ -f start.sh ]; then
                    ./start.sh stop 2>/dev/null || true
                fi
            fi
        fi
    fi

    log_success "服务端口: $PORT"
}

# ============================================================================
# 管理员配置
# ============================================================================
configure_admin() {
    log_step "Step 4: 管理员配置"

    ADMIN_USERNAME=$(read_input "管理员用户名" "$ADMIN_USERNAME")

    while true; do
        ADMIN_PASSWORD=$(read_password "管理员密码")
        
        if [ -z "$ADMIN_PASSWORD" ]; then
            log_error "密码不能为空"
            continue
        fi

        if [ ${#ADMIN_PASSWORD} -lt 8 ]; then
            log_warning "密码长度建议至少 8 位"
        fi

        local confirm_password=$(read_password "确认密码")
        
        if [ "$ADMIN_PASSWORD" = "$confirm_password" ]; then
            break
        else
            log_error "两次输入的密码不一致，请重新输入"
        fi
    done

    log_success "管理员配置完成"
}

# ============================================================================
# 数据库配置
# ============================================================================
configure_database() {
    if [ "$DEPLOY_MODE" != "cached" ] && [ "$DEPLOY_MODE" != "agent-memory" ] && [ "$DEPLOY_MODE" != "team" ]; then
        return
    fi

    log_step "Step 5: 数据库配置"

    PG_HOST=$(read_input "PostgreSQL 地址" "$PG_HOST")
    PG_PORT=$(read_input "PostgreSQL 端口" "$PG_PORT")
    PG_USER=$(read_input "PostgreSQL 用户名" "$PG_USER")
    PG_DATABASE=$(read_input "PostgreSQL 数据库名" "$PG_DATABASE")

    while true; do
        PG_PASSWORD=$(read_password "PostgreSQL 密码")
        
        if [ -z "$PG_PASSWORD" ]; then
            log_error "密码不能为空"
            continue
        fi
        break
    done

    log_success "数据库配置完成"
}

# ============================================================================
# 中间件配置
# ============================================================================
configure_middleware() {
    if [ "$DEPLOY_MODE" = "docker" ] || [ "$DEPLOY_MODE" = "local" ]; then
        return
    fi

    log_step "Step 6: 中间件配置"

    if [ "$DEPLOY_MODE" = "gateway" ]; then
        local enable_ollama=$(read_input "是否启用 Ollama 本地模型？(y/n)" "n")
        if [ "$enable_ollama" = "y" ]; then
            OLLAMA_ENABLED=true
            OLLAMA_HOST=$(read_input "Ollama 地址" "$OLLAMA_HOST")
        fi
    elif [ "$DEPLOY_MODE" = "cached" ]; then
        local enable_ollama=$(read_input "是否启用 Ollama 本地模型？(y/n)" "y")
        if [ "$enable_ollama" = "y" ]; then
            OLLAMA_ENABLED=true
            OLLAMA_HOST=$(read_input "Ollama 地址" "$OLLAMA_HOST")
        fi
    elif [ "$DEPLOY_MODE" = "agent-memory" ]; then
        OLLAMA_HOST=$(read_input "Ollama 地址" "$OLLAMA_HOST")
        OLLAMA_ENABLED=true
    fi

    log_success "中间件配置完成"
}

# ============================================================================
# 生成配置文件
# ============================================================================
generate_config() {
    log_step "Step 7: 生成配置文件"

    # 创建配置目录
    mkdir -p config/secrets

    # 备份现有配置
    if [ -f config/secrets/.env ]; then
        local backup_file="config/secrets/.env.backup.$(date +%s)"
        cp config/secrets/.env "$backup_file"
        log_info "已备份现有配置到 $backup_file"
    fi

    # 生成 .env 文件
    cat > config/secrets/.env << EOF
# Centag 配置 - 由部署向导生成
# 生成时间: $(date)

# 服务器配置
LLM_PROXY_SERVER_PORT=$PORT
LLM_PROXY_SERVER_HOST=0.0.0.0

# 管理员配置
LLM_PROXY_ADMIN_USERNAME=$ADMIN_USERNAME
LLM_PROXY_ADMIN_PASSWORD=$ADMIN_PASSWORD
LLM_PROXY_ADMIN_API_KEY=llmproxy_$(openssl rand -hex 16 2>/dev/null || head -c 32 /dev/urandom | base64 | tr -d '\+=' | head -c 32)

# 数据库配置
EOF

    # 根据模式添加数据库配置
    case "$DEPLOY_MODE" in
        gateway|docker|local)
            cat >> config/secrets/.env << EOF
LLM_PROXY_DB_DRIVER=sqlite
SQLITE_PATH=./storage/centag.db
EOF
            ;;
        cached|agent-memory|team)
            cat >> config/secrets/.env << EOF
LLM_PROXY_DB_DRIVER=postgresql
PG_HOST=$PG_HOST
PG_PORT=$PG_PORT
PG_USER=$PG_USER
PG_PASSWORD=$PG_PASSWORD
PG_DATABASE=$PG_DATABASE
PG_SSL_MODE=disable
EOF
            ;;
    esac

    # 添加中间件配置
    if [ "$OLLAMA_ENABLED" = true ]; then
        cat >> config/secrets/.env << EOF

# 中间件配置
OLLAMA_ENABLED=true
OLLAMA_HOST=$OLLAMA_HOST
EOF
    fi

    log_success "配置文件已生成: config/secrets/.env"
}

# ============================================================================
# 执行部署
# ============================================================================
execute_deploy() {
    log_step "Step 8: 执行部署"

    local deploy_cmd=""
    
    case "$DEPLOY_MODE" in
        gateway)
            deploy_cmd="./start.sh profile gateway up -d"
            ;;
        cached)
            deploy_cmd="./start.sh profile cached up -d"
            ;;
        agent-memory)
            deploy_cmd="./start.sh profile agent-memory up -d"
            ;;
        docker)
            deploy_cmd="./start.sh docker up -d"
            ;;
        local)
            deploy_cmd="./start.sh run be"
            ;;
    esac

    log_info "执行命令: $deploy_cmd"
    
    # 执行部署
    if eval "$deploy_cmd"; then
        log_success "部署命令执行成功"
    else
        log_error "部署命令执行失败"
        log_info "请检查日志: ./start.sh profile $DEPLOY_MODE logs"
        exit 1
    fi

    # 等待服务启动
    if [ "$DEPLOY_MODE" != "local" ]; then
        log_info "等待服务启动..."
        local max_wait=120
        local waited=0
        
        while [ $waited -lt $max_wait ]; do
            if curl -fsS "http://localhost:$PORT/health" > /dev/null 2>&1; then
                log_success "服务已就绪"
                break
            fi
            sleep 1
            waited=$((waited + 1))
            if [ $((waited % 10)) -eq 0 ]; then
                log_info "等待中... ($waited/$max_wait)"
            fi
        done

        if [ $waited -eq $max_wait ]; then
            log_warning "服务启动超时，请检查日志"
        fi
    fi
}

# ============================================================================
# 验证部署
# ============================================================================
verify_deploy() {
    log_step "Step 9: 验证部署"

    # 健康检查
    if curl -fsS "http://localhost:$PORT/health" > /dev/null 2>&1; then
        log_success "健康检查通过"
    else
        log_warning "健康检查失败，服务可能仍在启动中"
    fi

    # 显示访问信息
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✅ 部署完成！${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${BLUE}📍 访问地址：${NC}"
    echo -e "  - WebUI: http://localhost:$PORT"
    echo -e "  - API: http://localhost:$PORT/v1/chat/completions"
    echo -e "  - 健康检查: http://localhost:$PORT/health"
    echo ""
    echo -e "${BLUE}🔑 管理员账号：${NC}"
    echo -e "  - 用户名: $ADMIN_USERNAME"
    echo -e "  - 密码: <已配置>"
    echo ""
    echo -e "${BLUE}🚀 下一步操作：${NC}"
    echo -e "  1. 打开 WebUI 完成初始配置"
    echo -e "  2. 添加 LLM 后端供应商"
    echo -e "  3. 运行 \`./start.sh wizard\` 进行功能验证"
    echo -e "  4. 查看 \`docs/guide/\` 了解更多配置选项"
    echo ""
    echo -e "${BLUE}❓ 遇到问题？${NC}"
    echo -e "  - 查看日志: \`./start.sh profile $DEPLOY_MODE logs\`"
    echo -e "  - 停止服务: \`./start.sh profile $DEPLOY_MODE down\`"
    echo -e "  - 查看故障排查: \`docs/harness/skills/centag-deploy.md\`"
    echo ""
}

# ============================================================================
# 主流程
# ============================================================================
main() {
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --mode)
                DEPLOY_MODE="$2"
                shift 2
                ;;
            --port)
                PORT="$2"
                shift 2
                ;;
            --admin-user)
                ADMIN_USERNAME="$2"
                shift 2
                ;;
            --admin-pass)
                ADMIN_PASSWORD="$2"
                shift 2
                ;;
            --pg-host)
                PG_HOST="$2"
                shift 2
                ;;
            --pg-port)
                PG_PORT="$2"
                shift 2
                ;;
            --pg-user)
                PG_USER="$2"
                shift 2
                ;;
            --pg-pass)
                PG_PASSWORD="$2"
                shift 2
                ;;
            --pg-db)
                PG_DATABASE="$2"
                shift 2
                ;;
            --ollama-host)
                OLLAMA_HOST="$2"
                shift 2
                ;;
            --no-interact)
                NO_INTERACT=true
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done

    # 非交互模式验证
    if [ "$NO_INTERACT" = true ]; then
        if [ -z "$DEPLOY_MODE" ]; then
            log_error "非交互模式必须指定 --mode"
            exit 1
        fi
        
        # 验证必填参数
        if [ "$DEPLOY_MODE" = "cached" ] || [ "$DEPLOY_MODE" = "agent-memory" ]; then
            if [ -z "$ADMIN_PASSWORD" ]; then
                log_error "非交互模式必须指定 --admin-pass"
                exit 1
            fi
            if [ -z "$PG_PASSWORD" ]; then
                log_error "非交互模式必须指定 --pg-pass"
                exit 1
            fi
        elif [ "$DEPLOY_MODE" != "local" ]; then
            if [ -z "$ADMIN_PASSWORD" ]; then
                log_error "非交互模式必须指定 --admin-pass"
                exit 1
            fi
        fi
    fi

    # 显示欢迎信息
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  🚀 Centag Deploy Wizard${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    # 执行向导流程
    detect_environment
    select_mode
    configure_port
    configure_admin
    configure_database
    configure_middleware
    generate_config
    execute_deploy
    verify_deploy
}

# 执行主流程
main "$@"
