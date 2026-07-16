# shellcheck shell=bash
# Profile 与 stack 中间件的协作（方案 D）
#
# 三种 Profile 差异（设计真源见各 manifest.conf + docker-compose.yaml）：
#
# | Profile       | 应用 DB     | STACK_DEPS              | 容器内 stack .env | stack-network |
# |---------------|------------|-------------------------|-------------------|---------------|
# | gateway       | SQLite     | 无（默认）/ ollama（可选） | 不注入（仅 .env + secrets） | 仅 Ollama 时 |
# | cached        | PostgreSQL | postgresql ollama       | 注入（要 PG 密码） | 始终 |
# | agent-memory  | auto/PG    | postgresql qdrant ollama mem0 | 注入（Mem0 等） | centag 服务 |
#
# load_profile_env（start.sh）对三种 Profile 都会加载 stack/.env —— 仅供宿主机侧 stack ensure；
# gateway 容器 compose  deliberately 不含 deploy/stack/.env，避免 PG 变量污染 SQLite 模式。
#
# embedded — compose 内嵌中间件（agent-memory 可用 docker-compose.embedded.yaml）
# modular  — profile up 前 stack ensure，应用层连 deploy/stack-network

profile_stack_mode() {
    local name="${1:-}"
    local profile_dir="${2:-}"
    if [ -f "${profile_dir}/manifest.conf" ]; then
        # shellcheck source=/dev/null
        source "${profile_dir}/manifest.conf"
        echo "${STACK_MODE:-embedded}"
        return 0
    fi
    echo "embedded"
}

profile_stack_deps() {
    local profile_dir="${2:-}"
    if [ -f "${profile_dir}/manifest.conf" ]; then
        # shellcheck source=/dev/null
        source "${profile_dir}/manifest.conf"
        echo "${STACK_DEPS:-}"
        return 0
    fi
    echo ""
}

# 按 OLLAMA_ENABLED 等运行时变量过滤 manifest 中的 STACK_DEPS
profile_resolve_stack_deps() {
    local name="${1:-}"
    local profile_dir="${2:-}"
    local deps item resolved=""

    deps=$(profile_stack_deps "$name" "$profile_dir")

    for item in $deps; do
        case "$item" in
            ollama)
                case "${OLLAMA_ENABLED:-true}" in
                    false|0|no|NO|FALSE) continue ;;
                    *) resolved="$resolved ollama" ;;
                esac
                ;;
            *)
                resolved="$resolved $item"
                ;;
        esac
    done

    # shellcheck disable=SC2001
    echo "$resolved" | sed 's/^ //'
}

profile_uses_stack_network() {
    local name="$1"
    local profile_dir="$2"
    local mode deps

    mode=$(profile_stack_mode "$name" "$profile_dir")
    [ "$mode" = "modular" ] || return 1

    deps=$(profile_resolve_stack_deps "$name" "$profile_dir")
    [ -n "$deps" ]
}

# 返回 compose -f 参数（空格分隔，供 eval 或数组使用）
profile_compose_file_args() {
    local name="$1"
    local profile_dir="$2"
    local mode args

    mode=$(profile_stack_mode "$name" "$profile_dir")

    if [ "$mode" = "embedded" ] && [ -f "${profile_dir}/docker-compose.embedded.yaml" ]; then
        echo "-f docker-compose.embedded.yaml"
        return 0
    fi

    args="-f docker-compose.yaml"
    if profile_uses_stack_network "$name" "$profile_dir"; then
        args="$args -f docker-compose.stack-network.yaml"
    fi
    echo "$args"
}

# 在 profile_dir 下执行 compose（自动附加 stack-network overlay）
profile_invoke_compose() {
    local name="$1"
    local profile_dir="$2"
    local compose_cmd="$3"
    shift 3
    local file_args
    file_args=$(profile_compose_file_args "$name" "$profile_dir")
    # shellcheck disable=SC2086
    $compose_cmd $file_args "$@"
}

profile_stack_ollama_container() {
    echo "centag-ollama"
}

# modular 模式：在 stack 的 Ollama 容器中拉取模型
profile_pull_stack_ollama_models() {
    local container
    container=$(profile_stack_ollama_container)
    local model

    for model in "$@"; do
        print_info "拉取 ${model}..."
        docker exec "$container" ollama pull "$model" 2>/dev/null \
            || print_warn "${model} 拉取可能失败，请检查网络"
    done
}

# modular 模式：通过 stack lib 确保依赖
profile_ensure_stack_deps() {
    local name="$1"
    local profile_dir="$2"
    local mode deps

    mode=$(profile_stack_mode "$name" "$profile_dir")
    if [ "$mode" != "modular" ]; then
        return 0
    fi

    deps=$(profile_resolve_stack_deps "$name" "$profile_dir")
    if [ -z "$deps" ]; then
        print_info "Profile ${name}（modular）：无 stack 依赖，仅启动应用容器"
        return 0
    fi

    if [ ! -f "${PROFILE_PROJECT_ROOT:-}/deploy/stack/lib/stack.sh" ]; then
        print_error "未找到 deploy/stack/lib/stack.sh，无法确保中间件依赖"
        return 1
    fi

    print_info "Profile ${name}（modular）：确保 stack 依赖: ${deps}"
    # shellcheck disable=SC2086
    env STACK_ROOT="${PROFILE_PROJECT_ROOT}/deploy/stack" \
        STACK_INVOKER="./start.sh stack" \
        STACK_QUIET_CD=1 \
        bash -c 'source "${STACK_ROOT}/lib/stack.sh" && stack_ensure_services "$@"' bash $deps
}