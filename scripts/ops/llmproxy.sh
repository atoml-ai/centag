#!/bin/bash
# Centag 管理脚本
# 用法：llmproxy <command> [args...]

set -euo pipefail

# 配置
LLM_PROXY_URL="http://192.168.1.50:20060"
LLM_PROXY_ADMIN_KEY="llmproxy_062d55a83bdf78b6cd2d353f2d0a8bb2a5c008483b8af136265081ed6ef169f9"
LLM_PROXY_CONTAINER="centag"
DOCKER_CMD="/home/node/bin/docker"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[OK]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# API 请求辅助函数
api_get() { curl -s -H "Authorization: Bearer ${LLM_PROXY_ADMIN_KEY}" "$1"; }
api_put() { curl -s -X PUT -H "Authorization: Bearer ${LLM_PROXY_ADMIN_KEY}" -H "Content-Type: application/json" "$1" -d "$2"; }
api_post() { curl -s -X POST -H "Authorization: Bearer ${LLM_PROXY_ADMIN_KEY}" -H "Content-Type: application/json" "$1" -d "$2"; }

cmd_status() {
    print_info "检查 Centag 服务状态..."
    if $DOCKER_CMD ps --format '{{.Names}}' | grep -q "^${LLM_PROXY_CONTAINER}$"; then
        print_success "容器 ${LLM_PROXY_CONTAINER} 运行中"
    else
        print_error "容器 ${LLM_PROXY_CONTAINER} 未运行"; return 1
    fi
    local health=$(curl -s "${LLM_PROXY_URL}/health")
    if echo "$health" | grep -q '"status":"ok"'; then
        print_success "服务健康检查通过"
    else
        print_error "服务健康检查失败"; return 1
    fi
    echo ""
    print_info "服务地址：${LLM_PROXY_URL}"
    print_info "Web 界面：${LLM_PROXY_URL}/"
}

cmd_backends() {
    print_info "获取后端列表..."
    echo ""
    api_get "${LLM_PROXY_URL}/api/v1/backends" | python3 -c "
import sys, json
data = json.load(sys.stdin)
backends = data.get('data', [])
print(f\"{'ID':<20} {'Name':<25} {'Type':<10} {'Status':<8} {'Priority':<10}\")
print('-' * 80)
for b in backends:
    status = '✅' if b.get('enabled') else '❌'
    print(f\"{b.get('id'):<20} {b.get('name'):<25} {b.get('type'):<10} {status:<8} {b.get('priority'):<10}\")
print()
"
}

cmd_models() {
    print_info "获取可用模型..."
    echo ""
    api_get "${LLM_PROXY_URL}/v1/models" | python3 -c "
import sys, json
data = json.load(sys.stdin)
models = data.get('data', [])
print(f\"{'Model ID':<40} {'Owned By':<20}\")
print('-' * 65)
for m in models:
    print(f\"{m.get('id'):<40} {m.get('owned_by'):<20}\")
print()
"
}

cmd_config() {
    print_info "获取完整配置..."
    api_get "${LLM_PROXY_URL}/api/v1/config" | python3 -m json.tool
}

cmd_backend_action() {
    local backend_id="$1" action="$2"
    [ -z "$backend_id" ] || [ -z "$action" ] && { print_error "用法：llmproxy backend <id> <enable|disable>"; return 1; }
    local enabled="true"; [ "$action" = "disable" ] && enabled="false"
    [[ "$action" != "enable" && "$action" != "disable" ]] && { print_error "操作必须是 enable 或 disable"; return 1; }
    print_info "${action^} 后端：$backend_id"
    local result=$(api_put "${LLM_PROXY_URL}/api/v1/backends/${backend_id}" "{\"enabled\":${enabled}}")
    echo "$result" | grep -q '"success":true' && print_success "后端 ${backend_id} 已${action}" || { print_error "操作失败：$result"; return 1; }
}

cmd_backend_add() {
    local backend_id="$1" name="$2" base_url="$3" api_key="$4" backend_type="${5:-openai}"
    [ -z "$backend_id" ] || [ -z "$name" ] || [ -z "$base_url" ] || [ -z "$api_key" ] && {
        print_error "用法：llmproxy backend add <id> <name> <base_url> <api_key> [type]"
        print_error "示例：llmproxy backend add bailian 阿里云 Bailian https://dashscope.aliyuncs.com/<YOUR_API_KEY_HERE>xxx openai"
        return 1
    }
    print_info "添加后端：$backend_id ($name)"
    local payload=$(cat <<EOF
{
    "id": "${backend_id}",
    "name": "${name}",
    "type": "${backend_type}",
    "base_url": "${base_url}",
    "api_key": "${api_key}",
    "enabled": true,
    "weight": 50,
    "timeout": 60,
    "max_retries": 3,
    "description": "阿里云 Bailian (DashScope) API",
    "supported_models": [
        {"requested_model": "glm-4-flash", "actual_model": "glm-4-flash", "is_exact": true},
        {"requested_model": "qwen3-max-2026-01-23", "actual_model": "qwen3-max-2026-01-23", "is_exact": true},
        {"requested_model": "qwen3-coder-next", "actual_model": "qwen3-coder-next", "is_exact": true},
        {"requested_model": "qwen3-coder-plus", "actual_model": "qwen3-coder-plus", "is_exact": true},
        {"requested_model": "MiniMax-M2.5", "actual_model": "MiniMax-M2.5", "is_exact": true},
        {"requested_model": "glm-5", "actual_model": "glm-5", "is_exact": true},
        {"requested_model": "glm-4.7", "actual_model": "glm-4.7", "is_exact": true},
        {"requested_model": "kimi-k2.5", "actual_model": "kimi-k2.5", "is_exact": true},
        {"requested_model": "gpt-4", "actual_model": "qwen3-max-2026-01-23", "compatibility_score": 0.95},
        {"requested_model": "gpt-3.5-turbo", "actual_model": "glm-4-flash", "compatibility_score": 0.9}
    ],
    "capabilities": {
        "max_context_tokens": 128000,
        "features": ["streaming", "json_mode", "function_calling"],
        "supports_tools": true
    },
    "priority": 3
}
EOF
)
    local result=$(api_post "${LLM_PROXY_URL}/api/v1/backends" "$payload")
    echo "$result" | grep -q '"success":true' && print_success "后端 ${backend_id} 已添加" || { print_error "添加失败：$result"; return 1; }
}

cmd_restart() {
    print_info "重启 Centag 容器..."
    if $DOCKER_CMD restart ${LLM_PROXY_CONTAINER} >/dev/null 2>&1; then
        print_success "容器已重启"; sleep 3; cmd_status
    else
        print_error "重启失败"; return 1
    fi
}

cmd_logs() {
    local lines="${1:-50}"
    print_info "查看最近 ${lines} 行日志..."
    $DOCKER_CMD logs --tail ${lines} ${LLM_PROXY_CONTAINER} 2>&1
}

cmd_help() {
    echo "Centag 管理工具"
    echo -e "\n用法：llmproxy <command> [args...]\n"
    echo "命令:"
    echo "  status              检查服务健康状态"
    echo "  backends            列出所有后端配置"
    echo "  models              列出可用模型"
    echo "  config              获取完整配置"
    echo "  backend <id> <action>  启用/禁用后端 (enable|disable)"
    echo "  backend add <id> <name> <url> <key> [type]  添加新后端"
    echo "  restart             重启 centag 容器"
    echo "  logs [lines]        查看最近日志 (默认 50 行)"
    echo "  help                显示此帮助信息"
    echo -e "\n示例:"
    echo "  llmproxy status"
    echo "  llmproxy backends"
    echo "  llmproxy backend ollama-local disable"
    echo "  llmproxy backend add bailian '阿里云 Bailian' https://dashscope.aliyuncs.com <YOUR_API_KEY_HERE>xxx openai"
}

main() {
    local cmd="${1:-help}"; shift || true
    case "$cmd" in
        status) cmd_status ;;
        backends) cmd_backends ;;
        models) cmd_models ;;
        config) cmd_config ;;
        backend)
            local subcmd="${1:-}"
            if [ "$subcmd" = "add" ]; then
                shift
                cmd_backend_add "$@"
            else
                cmd_backend_action "$@"
            fi
            ;;
        restart) cmd_restart ;;
        logs) cmd_logs "$@" ;;
        help|--help|-h) cmd_help ;;
        *) print_error "未知命令：$cmd"; exit 1 ;;
    esac
}

main "$@"
