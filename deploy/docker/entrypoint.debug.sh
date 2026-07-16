#!/bin/bash
# Proxy Claw Debug 容器入口脚本
# 与 entrypoint.sh 相同，但优先使用 /app/bin/centag（本地编译）

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
print_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Debug 模式：host 网络模式下，添加 hosts 映射访问宿主机服务
if [ -n "${DEBUG_HOST_MODE:-}" ]; then
    print_info "检测到 host 网络模式，配置宿主机服务访问..."

    # 修复 hosts 文件中可能存在的格式错误（如：ff02::2 ip6-allrouters127.0.0.1 pg.atoml.net）
    # Docker 挂载 /etc/hosts，sed 无法直接修改，需要使用临时文件方式
    if grep -q "ip6-allrouters127\.0\.0\.1" /etc/hosts 2>/dev/null; then
        print_info "修复 hosts 文件格式..."
        local_hosts_tmp="/tmp/hosts.fixed"
        awk '{gsub(/ip6-allrouters127\.0\.0\.1/, "ip6-allrouters\n127.0.0.1"); print}' /etc/hosts > "$local_hosts_tmp"
        cat "$local_hosts_tmp" > /etc/hosts 2>/dev/null || print_warn "无法写入 /etc/hosts，可能需要特权模式"
        rm -f "$local_hosts_tmp"
    fi

    # 将 pg.atoml.net 解析到 127.0.0.1（宿主机）
    if ! grep -q "^127\.0\.0\.1.*pg\.atoml\.net" /etc/hosts 2>/dev/null; then
        # 确保文件末尾有换行符再追加
        if [ -s /etc/hosts ] && [ "$(tail -c 1 /etc/hosts | wc -l)" -eq 0 ]; then
            echo "" >> /etc/hosts
        fi
        echo "127.0.0.1 pg.atoml.net" >> /etc/hosts
        print_info "已添加 pg.atoml.net -> 127.0.0.1"
    fi
    # 将 ol.atoml.net 解析到 127.0.0.1（宿主机）
    if ! grep -q "^127\.0\.0\.1.*ol\.atoml\.net" /etc/hosts 2>/dev/null; then
        echo "127.0.0.1 ol.atoml.net" >> /etc/hosts
        print_info "已添加 ol.atoml.net -> 127.0.0.1"
    fi
fi

LLM_PROXY_SERVER_PORT=${LLM_PROXY_SERVER_PORT:-${SERVER_PORT:-20060}}
LLM_PROXY_SERVER_HOST=${LLM_PROXY_SERVER_HOST:-${SERVER_HOST:-0.0.0.0}}
# Debug 模式默认使用 debug 日志级别
LOG_LEVEL=${LOG_LEVEL:-debug}

export LLM_PROXY_SERVER_PORT
export LLM_PROXY_SERVER_HOST
export LLM_PROXY_LOG_LEVEL=${LLM_PROXY_LOG_LEVEL:-debug}

REDIS_ENABLED=${REDIS_ENABLED:-false}
REDIS_ADDR=${REDIS_ADDR:-localhost:26379}
REDIS_PASSWORD=${REDIS_PASSWORD:-}
REDIS_DB=${REDIS_DB:-0}

POSTGRES_ENABLED=${POSTGRES_ENABLED:-true}
# 支持 PG_* 和 POSTGRES_* 环境变量
POSTGRES_HOST=${PG_HOST:-${POSTGRES_HOST:-postgresql}}
POSTGRES_PORT=${PG_PORT:-${POSTGRES_PORT:-5432}}
POSTGRES_USER=${PG_USER:-${POSTGRES_USER:-postgres}}
POSTGRES_PASSWORD=${PG_PASSWORD:-${POSTGRES_PASSWORD:-}}
POSTGRES_DB=${PG_DATABASE:-${POSTGRES_DB:-centag}}

export POSTGRES_HOST POSTGRES_PORT POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB

VECTOR_ENABLED=${VECTOR_ENABLED:-false}
VECTOR_ADDR=${VECTOR_ADDR:-localhost:28000}
VECTOR_TYPE=${VECTOR_TYPE:-chromadb}

wait_for_service() {
    local host=$1
    local port=$2
    local service_name=$3
    local max_attempts=${4:-30}
    local wait_seconds=${5:-2}

    print_info "等待 ${service_name} 就绪 (${host}:${port})..."
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        # 使用 bash /dev/tcp 检测端口连通性
        if timeout 1 bash -c "cat < /dev/null > /dev/tcp/${host}/${port}" 2>/dev/null; then
            print_success "${service_name} 已就绪"
            return 0
        fi

        attempt=$((attempt + 1))
        print_info "  尝试 ${attempt}/${max_attempts}..."
        if [ $attempt -lt $max_attempts ]; then
            sleep $wait_seconds
        fi
    done

    print_warn "${service_name} 未在 ${max_attempts} 次尝试后响应，继续启动..."
    return 1
}

main() {
    print_info "======================================"
    print_info "   Proxy Claw Debug 容器启动"
    print_info "======================================"

    print_info "配置信息:"
    echo "  - 服务端口: ${LLM_PROXY_SERVER_PORT}"
    echo "  - 服务地址: ${LLM_PROXY_SERVER_HOST}"
    echo "  - 日志级别: ${LOG_LEVEL}"
    echo "  - PostgreSQL: ${POSTGRES_ENABLED} (${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB})"
    echo "  - Redis: ${REDIS_ENABLED} (${REDIS_ADDR})"
    echo "  - 向量数据库: ${VECTOR_ENABLED} (${VECTOR_TYPE} @ ${VECTOR_ADDR})"
    echo ""

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

    # 优先使用挂载进来的本地编译二进制
    if [ -x /app/bin/centag ]; then
        CENTAG_BIN="/app/bin/centag"
        print_info "使用本地编译二进制: $CENTAG_BIN"
        print_warn "提示: 修改本地 bin/centag 后，执行 ./start.sh docker-restart 即可生效"
    else
        CENTAG_BIN="/app/centag"
        print_error "未找到本地编译二进制: /app/bin/centag"
        print_info "请先在本地编译: go build -o bin/centag ./cmd/centag/main.go"
        print_info "使用容器内置二进制: $CENTAG_BIN"
    fi

    print_info "======================================"

    if [ "$(id -u)" -eq 0 ]; then
        mkdir -p /app/logs /app/storage /app/plugins
        chown -R llmproxy:llmproxy /app/logs /app/storage /app/plugins
        exec gosu llmproxy "$CENTAG_BIN"
    fi
    exec "$CENTAG_BIN"
}

trap 'print_info "接收到终止信号，正在关闭..."; exit 0' SIGTERM SIGINT

main "$@"
