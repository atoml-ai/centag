#!/bin/bash
# 生成 Centag 主服务所需 config/secrets/.env（不含 Redis/ES/Chroma/Ollama 等栈侧变量；见 deploy/stack）

set -euo pipefail

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
NC=$'\033[0m'

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
readonly SECRETS_DIR="$PROJECT_ROOT/config/secrets"
readonly SECRETS_FILE="$SECRETS_DIR/.env"

readonly DEFAULT_PG_HOST="${DEFAULT_PG_HOST:-pg.atoml.net}"
readonly DEFAULT_PG_PORT="${DEFAULT_PG_PORT:-5432}"
readonly DEFAULT_EXTERNAL_URL="${DEFAULT_EXTERNAL_URL:-localhost:20060}"

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
print_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

generate_password() {
    local password_length="${1:-32}"
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 48 | tr -d "=+/" | cut -c1-"${password_length}"
    elif command -v head >/dev/null 2>&1; then
        LC_ALL=C tr -dc 'A-Za-z0-9!@#$%^&*' < /dev/urandom | head -c "${password_length}"
    else
        local password=""
        local chars='A-Za-z0-9!@#$%^&*'
        for _ in $(seq 1 "${password_length}"); do
            password="${password}${chars:RANDOM%${#chars}:1}"
        done
        echo "$password"
    fi
}

generate_api_key() {
    local prefix="${1:-sk}"
    local key_length="${2:-48}"
    if command -v openssl >/dev/null 2>&1; then
        echo "${prefix}_$(openssl rand -hex "${key_length}")"
    else
        local key=""
        local chars='0123456789abcdef'
        for _ in $(seq 1 "${key_length}"); do
            key="${key}${chars:RANDOM%${#chars}:1}"
        done
        echo "${prefix}_${key}"
    fi
}

setup_secrets_dir() {
    if [ ! -d "$SECRETS_DIR" ]; then
        mkdir -p "$SECRETS_DIR"
        chmod 700 "$SECRETS_DIR"
        print_success "Created secrets directory: $SECRETS_DIR"
    fi
}

generate_secrets_file() {
    local use_same_password="${1:-true}"
    local admin_username="admin"
    local admin_password
    local admin_api_key
    local api_key_storage_secret

    admin_password="centag123"
    print_info "管理员使用固定口令: centag123"

    admin_api_key=$(generate_api_key "llmproxy" 32)
    if command -v openssl >/dev/null 2>&1; then
        api_key_storage_secret=$(openssl rand -hex 32)
    else
        api_key_storage_secret=$(generate_password 48)
    fi

    cat > "$SECRETS_FILE" << EOF
# =============================================================================
# Centag — config/secrets/.env（由 scripts/ops/generate-secrets.sh 生成）
# =============================================================================
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S')
# 勿提交到 Git。
#
# 使用说明：
#   - 开发调试：PG/Mem0 等中间件配置从 deploy/stack/.env 自动同步
#   - 正式环境：从云服务环境变量获取（如 K8s Secret、Vault 等）
# =============================================================================

# -----------------------------------------------------------------------------
# 服务监听
# -----------------------------------------------------------------------------
LLM_PROXY_SERVER_PORT=20060
LLM_PROXY_SERVER_HOST=0.0.0.0
LLM_PROXY_SERVER_MODE=release
LLM_PROXY_EXTERNAL_URL=${DEFAULT_EXTERNAL_URL}

# -----------------------------------------------------------------------------
# 元数据库（SQLite 为默认，可选 PostgreSQL）
# -----------------------------------------------------------------------------
LLM_PROXY_DB_DRIVER=sqlite
LLM_PROXY_DB_PATH=./storage/centag.db
# PG_HOST=          # 从 deploy/stack/.env 同步
# PG_PORT=5432
# PG_USER=postgres
# PG_PASSWORD=      # 从 deploy/stack/.env 同步
# PG_DATABASE=centag
# PG_SSL_MODE=disable

# -----------------------------------------------------------------------------
# Web 管理员（必填，API Key 用于认证）
# -----------------------------------------------------------------------------
LLM_PROXY_ADMIN_USERNAME=${admin_username}
LLM_PROXY_ADMIN_PASSWORD=${admin_password}
LLM_PROXY_DEFAULT_ADMIN_API_KEY_NAME=Default
LLM_PROXY_ADMIN_API_KEY=${admin_api_key}
LLM_PROXY_API_KEY_STORAGE_SECRET=${api_key_storage_secret}

# -----------------------------------------------------------------------------
# 代理默认（空库 seed；可按需修改）
# -----------------------------------------------------------------------------
LLM_PROXY_DEFAULT_MODE=transparent-proxy
LLM_PROXY_DEFAULT_BACKEND_ID=
LLM_PROXY_DEFAULT_MODEL=
LLM_PROXY_DEFAULT_EMBEDDING_MODEL=bge-m3:latest

# -----------------------------------------------------------------------------
# 日志
# -----------------------------------------------------------------------------
LLM_PROXY_LOG_LEVEL=info
LLM_PROXY_LOG_FORMAT=json
LLM_PROXY_LOG_OUTPUT=file
LLM_PROXY_LOG_PATH=./logs
LLM_PROXY_LOG_FILENAME=centag.log
LLM_PROXY_LOG_MAX_SIZE=0
LLM_PROXY_LOG_MAX_BACKUPS=0
LLM_PROXY_LOG_MAX_AGE=0
LLM_PROXY_LOG_COMPRESS=true

# -----------------------------------------------------------------------------
# 可选中间件（通过 deploy/stack/.env 或环境变量配置）
# -----------------------------------------------------------------------------
# Mem0（可选，需要时取消注释并配置）
# MEM0_ENABLED=false
# MEM0_PORT=20061
# MEM0_ADMIN_API_KEY=
# MEM0_API_URL=http://localhost:20061

# Pi Sandbox（可选，需要时取消注释并配置）
# PI_SANDBOX_ENABLED=false
# PI_SANDBOX_PORT=20062

# Ollama（可选）
# OLLAMA_HOST=http://127.0.0.1:21434

# Redis（可选）
# REDIS_ENABLED=false

# Elasticsearch（可选）
# ELASTICSEARCH_ENABLED=false
EOF

    chmod 600 "$SECRETS_FILE"
    print_success "已写入 $SECRETS_FILE"
}

show_secrets_summary() {
    print_info "========================================"
    print_info "摘要（勿泄露）"
    print_info "========================================"
    if [ -f "$SECRETS_FILE" ]; then
        print_info "Web UI Admin:"
        grep "^LLM_PROXY_ADMIN_USERNAME=" "$SECRETS_FILE" 2>/dev/null | cut -d'=' -f2- | sed 's/^/  Username: /'
        grep "^LLM_PROXY_ADMIN_PASSWORD=" "$SECRETS_FILE" 2>/dev/null | cut -d'=' -f2- | sed 's/^/  Password: /'
        print_info "PostgreSQL (PG_*):"
        grep "^PG_HOST=" "$SECRETS_FILE" 2>/dev/null | sed 's/^/  /'
        grep "^PG_DATABASE=" "$SECRETS_FILE" 2>/dev/null | sed 's/^/  /'
        print_info "========================================"
    fi
}

export_secrets() {
    if [ -f "$SECRETS_FILE" ]; then
        # shellcheck disable=SC2046
        export $(grep -v '^#' "$SECRETS_FILE" | grep -v '^$' | xargs) 2>/dev/null || true
        print_success "已导出环境变量（当前 shell）"
    else
        print_error "未找到 $SECRETS_FILE"
        return 1
    fi
}

main() {
    local use_same_password="true"

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --unique-passwords) use_same_password="false" ;;
            --same-password) use_same_password="true" ;;
        esac
        shift
    done

    print_info "========================================"
    print_info "Centag secrets 生成"
    print_info "========================================"

    setup_secrets_dir
    generate_secrets_file "$use_same_password"
    show_secrets_summary || true
    export_secrets || true

    print_success "完成。Compose 仅读取 config/secrets/.env，不再维护 deploy/docker/.env。"
    print_info "下一步: 在 deploy/stack 起依赖后 ./start.sh docker up"
}

main "$@"
exit 0
