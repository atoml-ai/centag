#!/bin/bash
# Centag 容器入口脚本

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 打印消息
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

# 默认配置
# 支持两种环境变量名：LLM_PROXY_SERVER_PORT（标准）或 SERVER_PORT（简化）
LLM_PROXY_SERVER_PORT=${LLM_PROXY_SERVER_PORT:-${SERVER_PORT:-20060}}
LLM_PROXY_SERVER_HOST=${LLM_PROXY_SERVER_HOST:-${SERVER_HOST:-0.0.0.0}}
LOG_LEVEL=${LOG_LEVEL:-info}

# 日志配置（支持 LLM_PROXY_LOG_LEVEL 和 LOG_LEVEL 两种变量名）
# 容器默认 both+console：docker logs 能看到请求日志；生产可用环境变量改回 file。
LLM_PROXY_LOG_LEVEL=${LLM_PROXY_LOG_LEVEL:-${LOG_LEVEL:-info}}
LLM_PROXY_LOG_FORMAT=${LLM_PROXY_LOG_FORMAT:-console}
LLM_PROXY_LOG_OUTPUT=${LLM_PROXY_LOG_OUTPUT:-both}
LLM_PROXY_LOG_PATH=${LLM_PROXY_LOG_PATH:-/app/logs}

# 导出给 centag 二进制使用
export LLM_PROXY_SERVER_PORT
export LLM_PROXY_SERVER_HOST
export LLM_PROXY_LOG_LEVEL
export LLM_PROXY_LOG_FORMAT
export LLM_PROXY_LOG_OUTPUT
export LLM_PROXY_LOG_PATH

# Redis 配置
REDIS_ENABLED=${REDIS_ENABLED:-false}
REDIS_ADDR=${REDIS_ADDR:-localhost:26379}
REDIS_PASSWORD=${REDIS_PASSWORD:-}
REDIS_DB=${REDIS_DB:-0}

# PostgreSQL 配置（最终是否等待 PG 由 apply_profile_db_mode 按 LLM_PROXY_DB_DRIVER 决定）
POSTGRES_ENABLED=${POSTGRES_ENABLED:-false}
POSTGRES_HOST=${POSTGRES_HOST:-centag-postgresql}
POSTGRES_PORT=${POSTGRES_PORT:-5432}
POSTGRES_USER=${POSTGRES_USER:-postgres}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-}
POSTGRES_DB=${POSTGRES_DB:-centag}

# 向量数据库配置
VECTOR_ENABLED=${VECTOR_ENABLED:-false}
VECTOR_ADDR=${VECTOR_ADDR:-localhost:28000}
VECTOR_TYPE=${VECTOR_TYPE:-chromadb}

# 等待依赖服务就绪
wait_for_service() {
    local host=$1
    local port=$2
    local service_name=$3
    local max_attempts=${4:-30}
    local wait_seconds=${5:-2}

    print_info "等待 ${service_name} 就绪 (${host}:${port})..."

    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        # 根据服务类型使用不同的检测方法
        case "$service_name" in
            PostgreSQL)
                # PostgreSQL 检测：使用 pg_isready 或尝试 TCP 连接
                if command -v pg_isready >/dev/null 2>&1; then
                    if PGPASSWORD="${POSTGRES_PASSWORD:-}" pg_isready -h "$host" -p "$port" -U "${POSTGRES_USER:-postgres}" >/dev/null 2>&1; then
                        print_success "${service_name} 已就绪"
                        return 0
                    fi
                elif command -v nc >/dev/null 2>&1; then
                    if nc -z "$host" "$port" 2>/dev/null; then
                        print_success "${service_name} 已就绪"
                        return 0
                    fi
                elif command -v timeout >/dev/null 2>&1 && command -v bash >/dev/null 2>&1; then
                    if timeout 1 bash -c "cat < /dev/null > /dev/tcp/$host/$port" 2>/dev/null; then
                        print_success "${service_name} 已就绪"
                        return 0
                    fi
                fi
                ;;
            Redis)
                # Redis 检测：尝试 TCP 连接
                if command -v nc >/dev/null 2>&1; then
                    if nc -z "$host" "$port" 2>/dev/null; then
                        print_success "${service_name} 已就绪"
                        return 0
                    fi
                elif command -v timeout >/dev/null 2>&1 && command -v bash >/dev/null 2>&1; then
                    if timeout 1 bash -c "cat < /dev/null > /dev/tcp/$host/$port" 2>/dev/null; then
                        print_success "${service_name} 已就绪"
                        return 0
                    fi
                fi
                ;;
            VectorDB)
                # 向量数据库检测：HTTP 健康检查
                if command -v curl >/dev/null 2>&1; then
                    # 尝试多个健康检查端点
                    if curl -f -s "http://${host}:${port}/healthz" >/dev/null 2>&1 || \
                       curl -f -s "http://${host}:${port}/api/v2/heartbeat" >/dev/null 2>&1 || \
                       curl -f -s "http://${host}:${port}/" >/dev/null 2>&1; then
                        print_success "${service_name} 已就绪"
                        return 0
                    fi
                fi
                ;;
            *)
                # 通用检测：尝试 TCP 连接
                if command -v nc >/dev/null 2>&1; then
                    if nc -z "$host" "$port" 2>/dev/null; then
                        print_success "${service_name} 已就绪"
                        return 0
                    fi
                elif command -v curl >/dev/null 2>&1; then
                    if curl -f -s "http://${host}:${port}" >/dev/null 2>&1; then
                        print_success "${service_name} 已就绪"
                        return 0
                    fi
                fi
                ;;
        esac

        attempt=$((attempt + 1))
        if [ $attempt -lt $max_attempts ]; then
            sleep $wait_seconds
        fi
    done

    print_warn "${service_name} 未在 ${max_attempts} 次尝试后响应，继续启动..."
    return 1
}

# 按 Profile 数据库驱动对齐 entrypoint 行为（与 personal/cached/agent-memory 设计一致）
# - sqlite（personal）    → 不等待任何 stack 中间件
# - postgresql（cached）  → 等待 stack PostgreSQL
# - auto（agent-memory）  → 等待 PostgreSQL（主机名由 compose / PG_HOST 注入）
apply_profile_db_mode() {
    local driver="${LLM_PROXY_DB_DRIVER:-sqlite}"

    case "$driver" in
        postgresql|auto)
            POSTGRES_ENABLED=true
            POSTGRES_HOST="${PG_HOST:-${POSTGRES_HOST:-centag-postgresql}}"
            ;;
        sqlite|*)
            POSTGRES_ENABLED=false
            ;;
    esac
}

# 主函数
main() {
    apply_profile_db_mode

    print_info "======================================"
    print_info "   Centag 容器启动"
    print_info "======================================"
    
    # 显示配置
    print_info "配置信息:"
    echo "  - 服务端口: ${LLM_PROXY_SERVER_PORT}"
    echo "  - 服务地址: ${LLM_PROXY_SERVER_HOST}"
    echo "  - 数据库驱动: ${LLM_PROXY_DB_DRIVER:-sqlite}"
    echo "  - 日志级别: ${LLM_PROXY_LOG_LEVEL} (${LLM_PROXY_LOG_FORMAT}, ${LLM_PROXY_LOG_OUTPUT}, path=${LLM_PROXY_LOG_PATH})"
    echo "  - PostgreSQL 等待: ${POSTGRES_ENABLED} (${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB})"
    echo "  - Redis: ${REDIS_ENABLED} (${REDIS_ADDR})"
    echo "  - 向量数据库: ${VECTOR_ENABLED} (${VECTOR_TYPE} @ ${VECTOR_ADDR})"
    echo ""

    # 等待依赖服务（失败时只打印警告，不中断启动）
    if [ "$POSTGRES_ENABLED" = "true" ]; then
        wait_for_service "$POSTGRES_HOST" "${POSTGRES_PORT}" "PostgreSQL" || true
    fi

    if [ "$REDIS_ENABLED" = "true" ]; then
        local redis_host=$(echo "$REDIS_ADDR" | cut -d: -f1)
        local redis_port=$(echo "$REDIS_ADDR" | cut -d: -f2)
        wait_for_service "$redis_host" "${redis_port:-6379}" "Redis" || true
    fi

    if [ "$VECTOR_ENABLED" = "true" ]; then
        local vector_host=$(echo "$VECTOR_ADDR" | cut -d: -f1)
        local vector_port=$(echo "$VECTOR_ADDR" | cut -d: -f2)
        wait_for_service "$vector_host" "${vector_port:-8000}" "VectorDB" || true
    fi
    
    # 首次启动：若尚无 SQLite 库，则从镜像内 initdata 拷贝默认库（与仓库 config/initdata/data/centag.db 同源）
    local data_dir="./data"
    local seed_db="./config/initdata/data/centag.db"
    if false; then  # Disabled: force PostgreSQL
        mkdir -p "$data_dir"
        cp "$seed_db" "${data_dir}/centag.db"
        print_success "已初始化 ${data_dir}/centag.db（默认配置，首次仅执行一次）"
    fi

    # 确保持久化目录存在（宿主机 bind-mount 时也需可写）
    # storage: SQLite + CENTAG_DATA_DIR；bin/static 可挂卷以持久化 OTA
    mkdir -p /app/logs /app/storage /app/storage/memory-store /app/plugins /app/bin/certs /app/bin/certs/domains /app/static

    # 卷为空时从镜像种子恢复（不覆盖已有 OTA 结果）
    seed_runtime_paths

    # 提示：相对 SQLITE_PATH 会落到 /app/bin/storage，脱离常见挂载点
    if [ -n "${SQLITE_PATH:-}" ]; then
        echo "  - SQLITE_PATH: ${SQLITE_PATH}"
    fi
    if [ -n "${CENTAG_DATA_DIR:-}" ]; then
        echo "  - CENTAG_DATA_DIR: ${CENTAG_DATA_DIR}"
    fi
    echo "  - DAEMON_MODE: ${DAEMON_MODE:-true}"

    # 设置权限
    if [ "$(id -u)" -eq 0 ]; then
        chown -R llmproxy:llmproxy /app/logs /app/storage /app/plugins /app/bin /app/static 2>/dev/null || true
    fi

    # 非 debug 默认守护进程（OTA update_stop + 崩溃拉起）；DAEMON_MODE=false 可直启
    if [ "${DAEMON_MODE:-true}" = "true" ]; then
        print_info "使用守护进程模式启动..."
        print_info "======================================"
        
        if [ -f "./daemon.sh" ]; then
            chmod +x ./daemon.sh
            if [ "$(id -u)" -eq 0 ]; then
                exec gosu llmproxy bash ./daemon.sh .
            else
                exec bash ./daemon.sh .
            fi
        else
            print_error "守护进程脚本不存在: ./daemon.sh"
            print_info "使用直接启动模式..."
            if [ "$(id -u)" -eq 0 ]; then
                exec gosu llmproxy /app/bin/centag
            else
                exec /app/bin/centag
            fi
        fi
    else
        print_info "启动 Centag 服务（直启，DAEMON_MODE=false）..."
        print_info "======================================"

        if [ "$(id -u)" -eq 0 ]; then
            exec gosu llmproxy /app/bin/centag
        else
            exec /app/bin/centag
        fi
    fi
}

# 将镜像内 /opt/centag/seed 拷到挂载卷（仅当目标缺少关键文件时）
seed_runtime_paths() {
    local seed_bin="/opt/centag/seed/bin/centag"
    local seed_static="/opt/centag/seed/static"

    if [ -f "$seed_bin" ] && [ ! -f /app/bin/centag ]; then
        print_info "种子初始化: /app/bin/centag"
        mkdir -p /app/bin
        cp -a "$seed_bin" /app/bin/centag
        chmod +x /app/bin/centag
    fi

    if [ -d "$seed_static" ] && [ ! -f /app/static/index.html ]; then
        print_info "种子初始化: /app/static"
        mkdir -p /app/static
        cp -a "$seed_static"/. /app/static/
    fi
}

# 信号处理
trap 'print_info "接收到终止信号，正在关闭..."; exit 0' SIGTERM SIGINT

# 执行主函数
main "$@"
