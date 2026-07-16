#!/bin/bash
# Centag Secrets Loader
# 此脚本用于从 secrets 目录加载认证信息并设置环境变量

set -euo pipefail

# 颜色定义
RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
NC=$'\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
if [ -f "$PROJECT_ROOT/config/secrets/.env" ]; then
    SECRETS_FILE="$PROJECT_ROOT/config/secrets/.env"
elif [ -f "$PROJECT_ROOT/config/secrets/.env.middleware" ]; then
    SECRETS_FILE="$PROJECT_ROOT/config/secrets/.env.middleware"
else
    SECRETS_FILE="$PROJECT_ROOT/config/secrets/.env"
fi

# 显示消息
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

# 检查 secrets 文件是否存在
check_secrets_file() {
    if [ ! -f "$SECRETS_FILE" ]; then
        print_warn "Secrets 文件不存在: $SECRETS_FILE"
        echo ""
        print_info "请运行以下命令之一:"
        echo "  1. 自动生成: ./start.sh init-secrets"
        echo "  2. 手动生成: ./start.sh generate-secrets"
        echo "  3. 手动创建 config/secrets/.env（参考 generate-secrets 生成的结构）"
        echo ""
        return 1
    fi
    return 0
}

# 验证 secrets 文件内容
validate_secrets() {
    local missing_vars=0

    # 检查必需的环境变量
    if ! grep -q "ELASTICSEARCH_PASSWORD=" "$SECRETS_FILE"; then
        print_warn "缺少 ELASTICSEARCH_PASSWORD"
        missing_vars=$((missing_vars + 1))
    fi

    if ! grep -q "REDIS_PASSWORD=" "$SECRETS_FILE"; then
        print_warn "缺少 REDIS_PASSWORD"
        missing_vars=$((missing_vars + 1))
    fi

    if [ $missing_vars -gt 0 ]; then
        print_warn "Secrets 文件缺少 $missing_vars 个必需的变量"
        return 1
    fi

    return 0
}

# 加载 secrets 文件
load_secrets() {
    print_info "加载中间件认证配置..."

    # 检查文件是否存在
    if ! check_secrets_file; then
        return 1
    fi

    # 验证文件内容
    if ! validate_secrets; then
        print_warn "Secrets 文件验证失败,但继续加载..."
    fi

    # 加载环境变量
    export $(grep -v '^#' "$SECRETS_FILE" | xargs)

    # 显示已加载的环境变量（隐藏敏感值）
    print_success "认证配置加载成功!"
    echo ""
    print_info "已加载的认证信息:"
    [ -n "${ELASTICSEARCH_USERNAME:-}" ] && echo "  ✓ Elasticsearch: $ELASTICSEARCH_USERNAME"
    [ -n "${ELASTICSEARCH_PASSWORD:-}" ] && echo "  ✓ Elasticsearch 密码: ********"
    [ -n "${ELASTICSEARCH_API_KEY:-}" ] && echo "  ✓ Elasticsearch API Key: ${ELASTICSEARCH_API_KEY:0:16}..."
    [ -n "${REDIS_PASSWORD:-}" ] && echo "  ✓ Redis 密码: ********"
    [ -n "${CHROMADB_TOKEN:-}" ] && echo "  ✓ ChromaDB Token: ${CHROMADB_TOKEN:0:16}..."
    [ -n "${CHROMADB_USERNAME:-}" ] && echo "  ✓ ChromaDB 用户: $CHROMADB_USERNAME"
    [ -n "${OLLAMA_API_KEY:-}" ] && echo "  ✓ Ollama API Key: ${OLLAMA_API_KEY:0:16}..."
    [ -n "${OPENAI_API_KEY:-}" ] && echo "  ✓ OpenAI API Key: ${OPENAI_API_KEY:0:16}..."
    echo ""

    # 显示文件信息
    if [ -f "$SECRETS_FILE" ]; then
        local file_stat=$(stat -c "%y" "$SECRETS_FILE" 2>/dev/null || stat -f "%Sm" "$SECRETS_FILE" 2>/dev/null || echo "未知")
        print_info "配置文件: $SECRETS_FILE"
        print_info "更新时间: $file_stat"
        print_info "文件权限: $(ls -l "$SECRETS_FILE" | awk '{print $1}')"
        echo ""
    fi

    return 0
}

# 显示使用说明
show_usage() {
    cat << EOF
${GREEN}使用方法:${NC}

方式一: 在当前 shell 中加载 (推荐)
  ${YELLOW}source scripts/load-secrets.sh${NC}

方式二: 导出环境变量
  ${YELLOW}export \$(cat config/secrets/.env | grep -v '^#' | xargs)${NC}

方式三: 使用 start.sh 命令
  ${YELLOW}./start.sh docker up${NC}         # 自动加载并启动
  ${YELLOW}./start.sh init-secrets${NC}       # 初始化认证配置

${GREEN}相关命令:${NC}
  ${YELLOW}./start.sh generate-secrets${NC}            # 生成新的认证信息
  ${YELLOW}./start.sh generate-secrets --same-password${NC}    # 使用相同密码
  ${YELLOW}./start.sh generate-secrets --unique-passwords${NC} # 使用不同密码
  ${YELLOW}cat config/secrets/.env${NC}                         # 查看配置文件

${GREEN}安全提示:${NC}
  - config/secrets/.env / .env.middleware 已在 .gitignore 中
  - 请勿将此文件提交到版本控制系统
  - 生产环境建议使用专业的密钥管理系统

EOF
}

# 主函数
main() {
    # 解析参数
    local show_help=false
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -h|--help)
                show_help=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done

    # 显示帮助信息
    if [ "$show_help" = "true" ]; then
        show_usage
        return 0
    fi

    print_info "========================================"
    print_info "Centag - Secrets Loader"
    print_info "========================================"
    echo ""

    # 加载 secrets
    if load_secrets; then
        print_success "========================================"
        print_success "认证信息已加载到当前 shell 环境"
        print_success "========================================"
        echo ""
        print_info "现在可以启动服务:"
        print_info "  ./start.sh docker up"
        print_info ""
        print_info "查看完整帮助:"
        print_info "  $0 --help"
    else
        print_error "========================================"
        print_error "认证信息加载失败"
        print_error "========================================"
        echo ""
        return 1
    fi
}

# 执行主函数
main "$@"
