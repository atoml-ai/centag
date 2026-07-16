# shellcheck shell=bash
# 对外暴露的中间件 ensure API（供 centag profile / stack ensure 复用）

# 检查单个 middleware 服务是否在运行
stack_middleware_running() {
    local svc="$1"
    local running
    running=$(docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack ps -q "$svc" 2>/dev/null)
    [ -n "$running" ] && docker ps -q --filter "id=$running" --filter "status=running" | grep -q .
}

# 确保 middleware 列表中的服务已启动并就绪
# 用法: stack_ensure_middleware postgresql qdrant ollama
stack_ensure_middleware() {
    local svc deps_needed=()

    if [ $# -eq 0 ]; then
        print_error "stack_ensure_middleware: 至少指定一个服务"
        return 1
    fi

    ensure_env_file
    load_env
    ensure_middleware_network || return 1

    for svc in "$@"; do
        case "$svc" in
            postgresql|redis|elasticsearch|qdrant|neo4j|ollama)
                if ! stack_middleware_running "$svc"; then
                    deps_needed+=("$svc")
                fi
                ;;
            *)
                print_error "stack_ensure_middleware: 不支持的服务 '$svc'"
                return 1
                ;;
        esac
    done

    if [ ${#deps_needed[@]} -eq 0 ]; then
        print_success "中间件已运行: $*"
        return 0
    fi

    print_info "启动中间件: ${deps_needed[*]}"
    docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack up --remove-orphans -d "${deps_needed[@]}"

    for svc in "${deps_needed[@]}"; do
        wait_for_dependency "$svc" 120 || return 1
    done

    print_success "中间件已就绪: $*"
}

# 确保 stack 服务（中间件 + mem0 + pi-sandbox）
# 用法: stack_ensure_services mem0 | stack_ensure_services postgresql ollama
stack_ensure_services() {
    local svc

    if [ $# -eq 0 ]; then
        print_error "stack_ensure_services: 至少指定一个服务"
        return 1
    fi

    for svc in "$@"; do
        case "$svc" in
            mem0|m0)
                start_mem0
                ;;
            pi-sandbox|pi)
                start_pi_sandbox
                ;;
            base)
                ensure_env_file
                load_env
                docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack --profile base up --remove-orphans -d
                ;;
            postgresql|postgres|pg|redis|rd|elasticsearch|es|qdrant|qd|neo4j|n4j|ollama|ol)
                svc=$(resolve_service "$svc")
                stack_ensure_middleware "$svc"
                ;;
            *)
                print_error "stack_ensure_services: 未知服务 '$svc'"
                return 1
                ;;
        esac
    done
}