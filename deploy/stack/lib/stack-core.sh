generate_password() {
    local length="${1:-32}"
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 "$length" | tr -d '=/+' | head -c "$length"
    else
        tr -dc 'A-Za-z0-9' </dev/urandom | head -c "$length"
    fi
}

load_env() {
    if [ -f "${STACK_PROJECT_ROOT}/.env" ]; then
        set -a
        # shellcheck source=/dev/null
        source "${STACK_PROJECT_ROOT}/.env"
        set +a
        print_success "环境变量已加载: .env"
    elif [ -f "${STACK_PROJECT_ROOT}/.env.example" ]; then
        print_warn "未找到 .env，使用 .env.example"
        set -a
        # shellcheck source=/dev/null
        source "${STACK_PROJECT_ROOT}/.env.example"
        set +a
    else
        print_warn "未找到环境变量文件"
    fi
}

# =============================================================================
# .env 读写与服务配置向导
# =============================================================================

is_placeholder_val() {
    local val="${1:-}"
    case "$val" in
        "" | __RANDOM_PASSWORD__ | '__RANDOM_PASSWORD__') return 0 ;;
        *) return 1 ;;
    esac
}

update_env_key() {
    local key="$1"
    local value="$2"
    local tmp_env escaped

    [ -f "${ENV_FILE}" ] || { print_error "未找到 ${ENV_FILE}"; exit 1; }

    tmp_env=$(mktemp)
    escaped=$(printf '%s' "$value" | sed 's/[&|]/\\&/g')

    if grep -q "^[[:space:]]*${key}=" "${ENV_FILE}" 2>/dev/null; then
        sed "s|^[[:space:]]*${key}=.*|${key}=${escaped}|" "${ENV_FILE}" > "$tmp_env"
    else
        cp "${ENV_FILE}" "$tmp_env"
        printf '%s=%s\n' "$key" "$value" >> "$tmp_env"
    fi
    mv "$tmp_env" "${ENV_FILE}"
}

wizard_read_secret() {
    local prompt="$1"
    local input=""

    echo -ne "${CYAN}${prompt}: ${NC}" >&2
    if [ -t 0 ]; then
        read -rs input
        echo "" >&2
    else
        read -rs input < /dev/tty 2>/dev/null || input=""
        echo "" >&2
    fi
    echo "$input"
}

require_interactive_config() {
    if [ -t 0 ] && [ -t 1 ]; then
        return 0
    fi
    print_error "缺少必要配置且当前为非交互环境，请编辑 ${ENV_FILE} 后重试"
    return 1
}

# Pi：检查是否已配置任一 LLM 密钥
has_pi_llm_credentials() {
    local var val
    local pi_key_vars=(
        ANTHROPIC_API_KEY ANTHROPIC_OAUTH_TOKEN OPENAI_API_KEY
        GEMINI_API_KEY GOOGLE_API_KEY DEEPSEEK_API_KEY KIMI_API_KEY
        GROQ_API_KEY OPENROUTER_API_KEY MISTRAL_API_KEY
        AZURE_OPENAI_API_KEY AI_GATEWAY_API_KEY XAI_API_KEY
    )
    load_env
    for var in "${pi_key_vars[@]}"; do
        val="${!var:-}"
        if ! is_placeholder_val "$val"; then
            return 0
        fi
    done
    return 1
}

wizard_configure_pi_llm() {
    wizard_step 1 "Pi Sandbox — LLM 配置"

    echo "Pi Sandbox 需要至少一个 LLM 供应商 API Key 才能调用模型。"
    echo ""
    echo "  ${GREEN}[1]${NC} OpenAI          → OPENAI_API_KEY"
    echo "  ${GREEN}[2]${NC} Kimi            → KIMI_API_KEY"
    echo "  ${GREEN}[3]${NC} Anthropic       → ANTHROPIC_API_KEY"
    echo "  ${GREEN}[4]${NC} DeepSeek        → DEEPSEEK_API_KEY"
    echo "  ${GREEN}[5]${NC} Google Gemini   → GEMINI_API_KEY"
    echo "  ${GREEN}[6]${NC} OpenRouter      → OPENROUTER_API_KEY"
    echo "  ${GREEN}[7]${NC} 稍后手动编辑 .env 并退出"
    echo ""

    local choice key_name api_key
    choice=$(wizard_read "请选择 LLM 供应商" "1")

    case "$choice" in
        1) key_name="OPENAI_API_KEY" ;;
        2) key_name="KIMI_API_KEY" ;;
        3) key_name="ANTHROPIC_API_KEY" ;;
        4) key_name="DEEPSEEK_API_KEY" ;;
        5) key_name="GEMINI_API_KEY" ;;
        6) key_name="OPENROUTER_API_KEY" ;;
        7)
            print_info "请编辑 ${ENV_FILE} 填入 API Key 后执行: ${STACK_INVOKER} start pi-sandbox"
            exit 0
            ;;
        *)
            print_error "无效选项: $choice"
            return 1
            ;;
    esac

    api_key=$(wizard_read_secret "请输入 ${key_name}（输入不回显）")
    if is_placeholder_val "$api_key"; then
        print_error "${key_name} 不能为空"
        return 1
    fi

    update_env_key "$key_name" "$api_key"
    load_env
    print_success "已保存 ${key_name} 到 .env"
}

ensure_pi_config() {
    ensure_env_file
    load_env

    if has_pi_llm_credentials; then
        return 0
    fi

    print_warn "未检测到 Pi Sandbox 的 LLM API Key"
    require_interactive_config || exit 1
    wizard_configure_pi_llm

    if ! has_pi_llm_credentials; then
        print_error "Pi Sandbox 仍缺少 LLM 配置，无法启动"
        exit 1
    fi
}

# Mem0：自动生成仍为占位符的栈内密钥
ensure_mem0_stack_secrets() {
    local password needs_update=false
    load_env

    if is_placeholder_val "${POSTGRES_PASSWORD:-}" \
        || is_placeholder_val "${MEM0_JWT_SECRET:-}" \
        || is_placeholder_val "${MEM0_ADMIN_API_KEY:-}" \
        || is_placeholder_val "${NEO4J_PASSWORD:-}"; then
        needs_update=true
    fi

    if ! $needs_update; then
        return 0
    fi

    password=$(generate_password 32)
    print_info "检测到 Mem0/中间件密钥未初始化，正在自动生成..."

    is_placeholder_val "${POSTGRES_PASSWORD:-}" && update_env_key "POSTGRES_PASSWORD" "$password"
    is_placeholder_val "${PG_PASSWORD:-}" && update_env_key "PG_PASSWORD" "$password"
    is_placeholder_val "${NEO4J_PASSWORD:-}" && update_env_key "NEO4J_PASSWORD" "$password"
    is_placeholder_val "${ELASTICSEARCH_PASSWORD:-}" && update_env_key "ELASTICSEARCH_PASSWORD" "$password"
    is_placeholder_val "${MEM0_JWT_SECRET:-}" && update_env_key "MEM0_JWT_SECRET" "$password"
    is_placeholder_val "${MEM0_ADMIN_API_KEY:-}" && update_env_key "MEM0_ADMIN_API_KEY" "mem0_${password}"
    is_placeholder_val "${MEM0_QDRANT_API_KEY:-}" && update_env_key "MEM0_QDRANT_API_KEY" "qdrant_${password}"

    load_env
    print_success "Mem0 认证与数据库密钥已写入 .env"
}

has_mem0_llm_credentials() {
    local llm_provider="${LLM_PROVIDER:-openai}"
    local embedder_provider="${EMBEDDER_PROVIDER:-ollama}"

    load_env

    case "$llm_provider" in
        ollama|Ollama|OLLAMA)
            # 本地 Ollama，由 start_mem0 自动拉起 middleware
            ;;
        openai|OpenAI|OPENAI)
            if is_placeholder_val "${MEM0_OPENAI_API_KEY:-}" \
                && is_placeholder_val "${OPENAI_API_KEY:-}"; then
                return 1
            fi
            ;;
        *)
            print_warn "未知 LLM_PROVIDER=${llm_provider}，按 openai 路径校验 API Key"
            if is_placeholder_val "${MEM0_OPENAI_API_KEY:-}" \
                && is_placeholder_val "${OPENAI_API_KEY:-}"; then
                return 1
            fi
            ;;
    esac

    case "$embedder_provider" in
        ollama|Ollama|OLLAMA)
            ;;
        openai|OpenAI|OPENAI)
            if is_placeholder_val "${MEM0_OPENAI_API_KEY:-}" \
                && is_placeholder_val "${OPENAI_API_KEY:-}"; then
                return 1
            fi
            ;;
    esac

    return 0
}

wizard_configure_mem0_llm() {
    wizard_step 1 "Mem0 — LLM / Embedder 配置"

    echo "Mem0 需要 LLM 与嵌入模型。可选择本地 Ollama（免费）或 OpenAI 兼容 API。"
    echo ""
    echo "  ${GREEN}[1]${NC} 本地 Ollama（推荐开发环境）"
    echo "      LLM + Embedder 均走 Ollama，将自动启动 centag-ollama 容器"
    echo ""
    echo "  ${GREEN}[2]${NC} OpenAI 兼容 API（BigModel / OpenAI / 其他）"
    echo "      使用 MEM0_OPENAI_API_KEY + MEM0_OPENAI_BASE_URL"
    echo ""
    echo "  ${GREEN}[3]${NC} 混合：OpenAI 兼容 LLM + Ollama 嵌入（省 API 费用）"
    echo ""
    echo "  ${GREEN}[4]${NC} 稍后手动编辑 .env 并退出"
    echo ""

    local choice api_key base_url model
    choice=$(wizard_read "请选择 Mem0 模型方案" "1")

    case "$choice" in
        1)
            update_env_key "LLM_PROVIDER" "ollama"
            update_env_key "EMBEDDER_PROVIDER" "ollama"
            update_env_key "OLLAMA_BASE_URL" "http://centag-ollama:11434"
            ;;
        2)
            api_key=$(wizard_read_secret "请输入 MEM0_OPENAI_API_KEY（或 OpenAI/BigModel Key）")
            if is_placeholder_val "$api_key"; then
                print_error "API Key 不能为空"
                return 1
            fi
            base_url=$(wizard_read "API Base URL" "https://open.bigmodel.cn/api/paas/v4")
            model=$(wizard_read "LLM 模型名" "glm-4-flash")
            update_env_key "LLM_PROVIDER" "openai"
            update_env_key "EMBEDDER_PROVIDER" "openai"
            update_env_key "MEM0_OPENAI_API_KEY" "$api_key"
            update_env_key "OPENAI_API_KEY" "$api_key"
            update_env_key "MEM0_OPENAI_BASE_URL" "$base_url"
            update_env_key "MEM0_OPENAI_MODEL" "$model"
            update_env_key "MEM0_EMBEDDING_DIMS" "1536"
            ;;
        3)
            api_key=$(wizard_read_secret "请输入 MEM0_OPENAI_API_KEY（LLM 用）")
            if is_placeholder_val "$api_key"; then
                print_error "API Key 不能为空"
                return 1
            fi
            base_url=$(wizard_read "API Base URL" "https://open.bigmodel.cn/api/paas/v4")
            model=$(wizard_read "LLM 模型名" "glm-4-flash")
            update_env_key "LLM_PROVIDER" "openai"
            update_env_key "EMBEDDER_PROVIDER" "ollama"
            update_env_key "MEM0_OPENAI_API_KEY" "$api_key"
            update_env_key "OPENAI_API_KEY" "$api_key"
            update_env_key "MEM0_OPENAI_BASE_URL" "$base_url"
            update_env_key "MEM0_OPENAI_MODEL" "$model"
            update_env_key "OLLAMA_BASE_URL" "http://centag-ollama:11434"
            update_env_key "MEM0_EMBEDDING_DIMS" "1024"
            ;;
        4)
            print_info "请编辑 ${ENV_FILE} 配置 LLM_PROVIDER / MEM0_OPENAI_API_KEY 等后执行: ${STACK_INVOKER} start mem0"
            exit 0
            ;;
        *)
            print_error "无效选项: $choice"
            return 1
            ;;
    esac

    load_env
    print_success "Mem0 LLM 配置已保存到 .env"
}

ensure_mem0_config() {
    ensure_env_file
    ensure_mem0_stack_secrets
    load_env

    if has_mem0_llm_credentials; then
        print_success "Mem0 LLM 配置检查通过（LLM_PROVIDER=${LLM_PROVIDER:-openai}, EMBEDDER_PROVIDER=${EMBEDDER_PROVIDER:-ollama}）"
        return 0
    fi

    print_warn "未检测到 Mem0 所需的 LLM API 配置"
    require_interactive_config || exit 1
    wizard_configure_mem0_llm

    if ! has_mem0_llm_credentials; then
        print_error "Mem0 仍缺少 LLM 配置，无法启动"
        exit 1
    fi
}

# =============================================================================
# 共享网络（仅 Mem0 等需要访问 middleware 的服务使用）
# =============================================================================

# 共享网络必须由 middleware compose 创建（勿 docker network create，否则会触发 compose 警告）
ensure_middleware_network() {
    local network_name="centag-stack-network"

    if ! docker network inspect "$network_name" >/dev/null 2>&1; then
        return 0
    fi

    local compose_project
    compose_project=$(docker network inspect "$network_name" --format '{{index .Labels "com.docker.compose.project"}}' 2>/dev/null || echo "")

    # 旧版 start.sh 用 docker network create 手动创建的网络没有 compose 项目标签
    if [ -n "$compose_project" ]; then
        return 0
    fi

    local attached
    attached=$(docker network inspect "$network_name" --format '{{len .Containers}}' 2>/dev/null || echo "0")

    if [ "${attached:-0}" -gt 0 ]; then
        print_warn "检测到旧版手动创建的 ${network_name}（无 Compose 标签），且仍有 ${attached} 个容器连接"
        print_warn "请先执行: ${STACK_INVOKER} stop all"
        print_warn "然后执行: docker network rm ${network_name}"
        print_warn "再重新: ${STACK_INVOKER} start mem0"
        return 1
    fi

    print_info "移除旧版手动创建的 ${network_name}，将由 middleware compose 重新创建..."
    docker network rm "$network_name" >/dev/null
}

service_compose_file() {
    local svc="$1"
    case "$svc" in
        mem0) echo "${SERVICES_DIR}/mem0/docker-compose.yml" ;;
        pi-sandbox) echo "${SERVICES_DIR}/pi-sandbox/docker-compose.yml" ;;
        *) echo "${MIDDLEWARE_DIR}/docker-compose.yml" ;;
    esac
}

# =============================================================================
# 依赖健康检查
# =============================================================================

wait_for_dependency() {
    local svc="$1"
    local max_wait="${2:-120}"
    local elapsed=0

    print_info "等待 $svc 就绪（最多 ${max_wait}s）..."
    while [ $elapsed -lt $max_wait ]; do
        local status
        status=$(docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack ps --format json "$svc" 2>/dev/null | grep -o '"Health":"[^"]*"' | cut -d'"' -f4 || echo "")
        if [ "$status" = "healthy" ]; then
            print_success "$svc 已就绪 (${elapsed}s)"
            return 0
        fi
        # 兼容旧版 docker compose 无 json 格式
        local raw_status
        raw_status=$(docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack ps "$svc" 2>/dev/null | grep -o "(healthy)" || true)
        if [ -n "$raw_status" ]; then
            print_success "$svc 已就绪 (${elapsed}s)"
            return 0
        fi
        sleep 3
        elapsed=$((elapsed + 3))
    done
    print_error "$svc 等待超时 (${max_wait}s)，请检查日志: ${STACK_INVOKER} logs $svc"
    return 1
}

check_dependencies() {
    local deps_ok=true
    command -v docker >/dev/null 2>&1 || { print_error "Docker 未安装"; deps_ok=false; }
    docker info >/dev/null 2>&1 || { print_error "Docker 守护进程未运行"; deps_ok=false; }
    docker compose --env-file "${ENV_FILE}" version >/dev/null 2>&1 || docker-compose version >/dev/null 2>&1 || { print_error "Docker Compose 未安装"; deps_ok=false; }
    $deps_ok || exit 1
}

# =============================================================================
# 向导模式
# =============================================================================

wizard_read() {
    local prompt="$1"
    local default="${2:-}"
    local input=""

    if [ -n "$default" ]; then
        echo -ne "${CYAN}${prompt} [${default}]: ${NC}" >&2
    else
        echo -ne "${CYAN}${prompt}: ${NC}" >&2
    fi

    if [ -t 0 ]; then
        read -r input
    else
        read -r input < /dev/tty 2>/dev/null || input=""
    fi

    echo "${input:-$default}"
}

wizard_confirm() {
    local prompt="$1"
    local default="${2:-n}"
    local input

    if [ "$default" = "y" ]; then
        echo -ne "${YELLOW}${prompt} [Y/n]: ${NC}" >&2
    else
        echo -ne "${YELLOW}${prompt} [y/N]: ${NC}" >&2
    fi

    read -r input
    input="${input:-$default}"
    [[ "$input" =~ ^[Yy]$ ]]
}

wizard_step() {
    local step_num="$1"
    local title="$2"
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  步骤 ${step_num}: ${title}${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

ensure_env_file() {
    if [ ! -f "${STACK_PROJECT_ROOT}/.env" ]; then
        print_warn "未找到 .env 文件"
        if [ -t 0 ] && [ -t 1 ]; then
            # 交互式环境：询问用户
            echo ""
            echo -e "启动服务需要配置文件，是否现在生成? ${CYAN}[Y/n]${NC}"
            echo -ne "> "
            read -r answer
            if [[ "$answer" =~ ^[Nn] ]]; then
                print_info "已取消"
                exit 0
            else
                init_env_file
            fi
        else
            # 非交互式环境：自动生成
            print_info "非交互式环境，自动生成 .env..."
            init_env_file
        fi
    fi
}

init_env_file() {
    if [ ! -f "${STACK_PROJECT_ROOT}/.env" ]; then
        if [ ! -f "${STACK_PROJECT_ROOT}/.env.example" ]; then
            print_error "未找到 .env.example 文件"
            exit 1
        fi

        print_info "从 .env.example 创建 .env..."
        cp "${STACK_PROJECT_ROOT}/.env.example" "${STACK_PROJECT_ROOT}/.env"

        # 检测已有数据卷残留：若 PostgreSQL 数据卷存在但 .env 被重新生成，
        # 会导致新密码与旧数据卷中存储的密码不一致，引发认证失败。
        if docker volume inspect centag-stack_postgresql_data >/dev/null 2>&1; then
            print_warn "检测到 PostgreSQL 数据卷已存在，但 .env 将被重新生成"
            print_warn "这会导致新密码与数据卷中旧密码不一致！"
            print_warn "建议先执行: ${STACK_INVOKER} clean all"
        fi

        local password
        password=$(generate_password 32)
        print_info "自动生成统一密钥..."

        # 写入临时文件再 mv：兼容 macOS BSD sed（其 sed -i 需备份后缀参数，易误解析脚本）
        local tmp_env
        tmp_env=$(mktemp)

        # 支持通过环境变量 MEM0_BIGMODEL_API_KEY 自动注入 BigModel 密钥
        local mem0_api_key=""
        if [ -n "${MEM0_BIGMODEL_API_KEY:-}" ]; then
            mem0_api_key="${MEM0_BIGMODEL_API_KEY}"
        fi

        sed \
            -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${password}|" \
            -e "s|^PG_PASSWORD=.*|PG_PASSWORD=${password}|" \
            -e "s|^REDIS_PASSWORD=.*|REDIS_PASSWORD=|" \
            -e "s|^ELASTICSEARCH_PASSWORD=.*|ELASTICSEARCH_PASSWORD=${password}|" \
            -e "s|^NEO4J_PASSWORD=.*|NEO4J_PASSWORD=${password}|" \
            -e "s|^MEM0_ADMIN_API_KEY=.*|MEM0_ADMIN_API_KEY=mem0_${password}|" \
            -e "s|^MEM0_JWT_SECRET=.*|MEM0_JWT_SECRET=${password}|" \
            -e "s|^MEM0_QDRANT_API_KEY=.*|MEM0_QDRANT_API_KEY=qdrant_${password}|" \
            -e "s|^MEM0_OPENAI_API_KEY=.*|MEM0_OPENAI_API_KEY=${mem0_api_key}|" \
            -e "s|^MEM0_OPENAI_BASE_URL=.*|MEM0_OPENAI_BASE_URL=https://open.bigmodel.cn/api/paas/v4|" \
            -e "s|^MEM0_OPENAI_MODEL=.*|MEM0_OPENAI_MODEL=glm-4-flash|" \
            -e "s|^LLM_PROVIDER=.*|LLM_PROVIDER=openai|" \
            -e "s|^MEM0_POSTGRES_PASSWORD=.*|MEM0_POSTGRES_PASSWORD=${password}|" \
            "${STACK_PROJECT_ROOT}/.env" > "$tmp_env"
        mv "$tmp_env" "${STACK_PROJECT_ROOT}/.env"

        print_success "已创建 .env 并生成统一密钥"
    else
        print_success "找到 .env 文件"
    fi

    load_env
}

run_wizard() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║         Centag Stack 交互式启动向导                       ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    wizard_step 1 "环境变量配置"
    ensure_env_file

    echo ""
    print_info "当前配置的服务端口:"
    echo "  PostgreSQL:  ${POSTGRES_PORT:-5432}"
    echo "  Redis:       ${REDIS_PORT:-26379}"
    echo "  Qdrant:      ${QDRANT_PORT:-6333}"
    echo "  Neo4j:       ${NEO4J_HTTP_PORT:-7474}"
    echo "  Mem0:        ${MEM0_PORT:-20061}"
    echo "  Pi Sandbox:  ${PI_SANDBOX_PORT:-20062}"
    echo ""
    print_info "统一密钥已自动生成（可通过修改 .env 更换）"

    wizard_step 2 "选择服务组合"

    echo "请选择要启动的服务组合（输入数字）:"
    echo ""
    echo "  ${GREEN}[1]${NC} 基础中间件（推荐）"
    echo "      包含: PostgreSQL, Redis, Elasticsearch, Qdrant, Neo4j, Ollama"
    echo ""
    echo "  ${GREEN}[2]${NC} Mem0 完整栈"
    echo "      包含: PostgreSQL + Qdrant + Neo4j + Mem0 Server"
    echo ""
    echo "  ${GREEN}[3]${NC} 全部服务"
    echo "      包含: 基础中间件 + Mem0 + Pi Sandbox"
    echo ""
    echo "  ${GREEN}[4]${NC} 仅 Pi Sandbox（独立）"
    echo "      Pi Agent HTTP 网关，无需中间件（需在 .env 配置 LLM API Key）"
    echo ""
    echo "  ${GREEN}[5]${NC} 仅 PostgreSQL"
    echo "      仅启动 PostgreSQL 数据库"
    echo ""
    echo "  ${GREEN}[6]${NC} 自定义选择"
    echo "      手动输入服务名（支持简写: pg, rd, es, qd, n4j, ol, m0, pi）"
    echo ""

    local choice
    choice=$(wizard_read "请输入选项" "1")

    local services_to_start=()

    case "$choice" in
        1)
            services_to_start+=("base")
            ;;
        2)
            services_to_start+=("mem0")
            ;;
        3)
            services_to_start+=("base")
            services_to_start+=("mem0")
            services_to_start+=("pi-sandbox")
            ;;
        4)
            services_to_start+=("pi-sandbox")
            ;;
        5)
            services_to_start+=("postgres")
            ;;
        6)
            run_custom_selection
            return
            ;;
        *)
            print_error "无效选项: $choice"
            exit 1
            ;;
    esac

    wizard_step 3 "确认并启动"

    ensure_env_file

    echo "将要启动的服务:"
    for svc in "${services_to_start[@]}"; do
        echo "  - ${GREEN}$svc${NC}"
    done
    echo ""

    if wizard_confirm "确认启动?" "y"; then
        for svc in "${services_to_start[@]}"; do
            print_info "启动服务: $svc"
            case "$svc" in
                base)
                    docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack --profile base up --remove-orphans -d
                    ;;
                mem0)
                    start_mem0
                    ;;
                pi-sandbox)
                    start_pi_sandbox
                    ;;
                *)
                    docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack up --remove-orphans -d "$svc"
                    ;;
            esac
        done

        echo ""
        print_success "服务启动完成!"
        echo ""
        print_info "查看状态: ${STACK_INVOKER} status"
        print_info "查看日志: ${STACK_INVOKER} logs <服务名>"
        echo ""
        print_info "提示: 编辑 .env 文件可修改配置和密钥"
    else
        print_info "已取消启动"
    fi
}

run_custom_selection() {
    echo ""
    echo -e "${CYAN}自定义服务选择${NC}"
    echo ""
    echo "可用服务: postgres/pg, redis/rd, elasticsearch/es, qdrant/qd, neo4j/n4j, ollama/ol, mem0/m0, pi-sandbox/pi"
    echo ""
    echo "请输入要启动的服务名（空格分隔，留空结束）:"
    echo -ne "${CYAN}> ${NC}"
    read -r input

    if [ -z "$input" ]; then
        print_warn "未选择任何服务"
        return
    fi

    local selected=()
    for svc in $input; do
        local resolved
        resolved=$(resolve_service "$svc")
        if [[ "$resolved" != "$svc" ]] || [[ "$svc" =~ ^(postgres|redis|elasticsearch|qdrant|neo4j|ollama|mem0|pi-sandbox)$ ]]; then
            selected+=("$resolved")
        else
            print_warn "未知服务: $svc"
        fi
    done

    if [ ${#selected[@]} -eq 0 ]; then
        print_warn "未选择任何有效服务"
        return
    fi

    echo ""
    print_info "将启动: ${selected[*]}"

    ensure_env_file

    for svc in "${selected[@]}"; do
        case "$svc" in
            mem0)
                start_mem0
                ;;
            pi-sandbox)
                start_pi_sandbox
                ;;
            *)
                docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack up --remove-orphans -d "$svc"
                ;;
    esac
    done

    print_success "服务启动完成!"
}

# =============================================================================
# 服务管理命令
# =============================================================================

start_mem0() {
    local compose_file="${SERVICES_DIR}/mem0/docker-compose.yml"

    if [ ! -f "$compose_file" ]; then
        print_error "未找到: $compose_file"
        exit 1
    fi

    ensure_mem0_config
    ensure_middleware_network || exit 1

    print_info "检查 Mem0 依赖的中间件（由 start.sh 自动拉起，并创建 centag-stack-network）..."
    local deps_needed=()

    for svc in postgresql qdrant neo4j ollama; do
        local running=$(docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack ps -q "$svc" 2>/dev/null)
        if [ -z "$running" ] || ! docker ps -q --filter "id=$running" --filter "status=running" | grep -q .; then
            deps_needed+=("$svc")
        fi
    done

    if [ ${#deps_needed[@]} -gt 0 ]; then
        print_info "启动 Mem0 依赖: ${deps_needed[*]}"
        docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack up --remove-orphans -d "${deps_needed[@]}"
        for svc in "${deps_needed[@]}"; do
            wait_for_dependency "$svc" 120 || exit 1
        done
    else
        print_success "依赖服务已运行"
    fi

    print_info "启动 Mem0 服务..."
    # 清理卡在 Created 状态的同名容器，避免并行启动时 compose 报 name conflict
    local stale_mem0
    stale_mem0=$(docker ps -a --filter "name=^/centag-mem0$" --filter "status=created" -q 2>/dev/null || true)
    if [ -n "$stale_mem0" ]; then
        print_warn "发现未启动的 centag-mem0 容器，正在清理后重试..."
        docker rm -f centag-mem0 2>/dev/null || true
    fi
    docker compose --env-file "${ENV_FILE}" -f "$compose_file" up --remove-orphans -d --build
    print_success "Mem0 服务已启动"
}

start_pi_sandbox() {
    local compose_file="${SERVICES_DIR}/pi-sandbox/docker-compose.yml"

    if [ ! -f "$compose_file" ]; then
        print_error "未找到: $compose_file"
        exit 1
    fi

    ensure_pi_config

    print_info "启动 Pi Sandbox（独立服务，无需 middleware 网络）..."
    docker compose --env-file "${ENV_FILE}" -f "$compose_file" up --remove-orphans -d --build
    print_success "Pi Sandbox 已启动（根路径与 UI: http://localhost:${PI_SANDBOX_PORT:-20062}/ 与 …/ui/，健康检查 …/api/health）"
}

stop_service() {
    local service
    service=$(resolve_service "${1:-}")

    case "$service" in
        base)
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack --profile base down 2>/dev/null || true
            ;;
        mem0)
            docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/mem0/docker-compose.yml" down 2>/dev/null || true
            ;;
        pi-sandbox)
            docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/pi-sandbox/docker-compose.yml" down 2>/dev/null || true
            ;;
        all)
            print_info "停止所有服务..."
            # 停止 docker compose 网络（使用各自的项目名称）
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack down --remove-orphans 2>/dev/null || true
            docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/mem0/docker-compose.yml" down --remove-orphans 2>/dev/null || true
            docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/pi-sandbox/docker-compose.yml" down --remove-orphans 2>/dev/null || true
            print_success "所有服务已停止"
            ;;
        postgresql|redis|elasticsearch|qdrant|neo4j|ollama)
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack stop "$service" 2>/dev/null || true
            print_success "服务已停止: $service"
            ;;
        *)
            print_error "未知服务: $service"
            echo "可用: base, mem0, pi-sandbox, all, postgres, redis, elasticsearch, qdrant, neo4j, ollama"
            exit 1
            ;;
    esac
}

clean_service() {
    local service
    service=$(resolve_service "${1:-}")

    case "$service" in
        mem0)
            print_warn "即将清理 Mem0: 停止容器、删除容器/网络/数据卷、删除镜像..."
            docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/mem0/docker-compose.yml" down -v --remove-orphans --rmi all 2>/dev/null || true
            # 额外清理可能残留的命名镜像
            docker image rm centag-mem0:latest 2>/dev/null || true
            # 清理 dangling 镜像
            docker image prune -f --filter "label=centag=mem0" 2>/dev/null || true
            print_success "Mem0 容器、数据卷和镜像已清理"
            ;;
        pi-sandbox)
            print_warn "即将清理 Pi Sandbox: 停止容器、删除容器/网络/数据卷、删除镜像..."
            docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/pi-sandbox/docker-compose.yml" down -v --remove-orphans --rmi all 2>/dev/null || true
            print_success "Pi Sandbox 容器、数据卷和镜像已清理"
            ;;
        base)
            print_warn "即将清理基础中间件: 停止容器、删除容器/网络/数据卷..."
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack --profile base down -v --remove-orphans 2>/dev/null || true
            print_success "基础中间件容器与数据卷已清理（镜像保留，ollama 不受影响）"
            ;;
        all)
            print_warn "即将清理所有服务: 停止容器、删除容器/网络/数据卷、删除服务镜像..."
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack down -v --remove-orphans 2>/dev/null || true
            docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/mem0/docker-compose.yml" down -v --remove-orphans --rmi all 2>/dev/null || true
            docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/pi-sandbox/docker-compose.yml" down -v --remove-orphans --rmi all 2>/dev/null || true
            docker image rm centag-mem0:latest 2>/dev/null || true
            print_success "所有服务容器、数据卷和镜像已清理"
            ;;
        postgresql|redis|elasticsearch|qdrant|neo4j|ollama)
            print_warn "即将清理 $service: 停止并删除容器..."
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack rm -fs "$service" 2>/dev/null || true
            print_success "服务已清理: $service"
            ;;
        *)
            print_error "未知服务: $service"
            echo "可用: base, mem0, pi-sandbox, all, postgres, redis, elasticsearch, qdrant, neo4j, ollama"
            exit 1
            ;;
    esac
}

show_status() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}              Centag Stack 状态                        ${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo ""

print_info "基础中间件:"
    docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack ps 2>/dev/null || echo "  未运行"
    echo ""
    print_info "Mem0 服务:"
    docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/mem0/docker-compose.yml" ps 2>/dev/null || echo "  未运行"
    echo ""
    print_info "Pi Sandbox:"
    docker compose --env-file "${ENV_FILE}" -f "${SERVICES_DIR}/pi-sandbox/docker-compose.yml" ps 2>/dev/null || echo "  未运行"
    echo ""
}

show_health() {
    print_info "=== 健康检查 ==="
    local failed=0

    load_env
    check_service_health "PostgreSQL" "${POSTGRES_PORT:-5432}" || failed=$((failed + 1))
    check_service_health "Redis" "${REDIS_PORT:-26379}" || failed=$((failed + 1))
    check_service_health "Qdrant" "${QDRANT_PORT:-6333}" || failed=$((failed + 1))
    check_service_health "Neo4j" "${NEO4J_HTTP_PORT:-7474}" || failed=$((failed + 1))
    check_service_health "Mem0" "${MEM0_PORT:-20061}" || failed=$((failed + 1))
    check_service_health "Pi Sandbox" "${PI_SANDBOX_PORT:-20062}" || failed=$((failed + 1))

    if [ $failed -eq 0 ]; then
        print_success "所有服务健康"
    else
        print_warn "$failed 个服务不可用"
    fi
}

check_service_health() {
    local name="$1"
    local port="$2"
    if bash -c "echo >/dev/tcp/localhost/$port" 2>/dev/null; then
        print_success "$name ($port) - 可用"
        return 0
    else
        print_warn "$name ($port) - 不可用"
        return 1
    fi
}

# =============================================================================
# 帮助信息
# =============================================================================

show_help() {
    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║                  Centag Stack 管理脚本                        ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC} ${GREEN}${STACK_INVOKER}${NC} ${YELLOW}<命令>${NC} [服务]"
    echo ""
    echo -e "${CYAN}━━━ 启动向导 ━━━${NC}"
    echo "  ${GREEN}w${NC}, ${GREEN}wizard${NC}              交互式启动向导（推荐首次使用）"
    echo ""
    echo -e "${CYAN}━━━ 启动服务 ━━━${NC}"
    echo "  ${GREEN}start${NC} [服务]         启动服务"
    echo "  ${GREEN}start base${NC}            启动所有基础中间件"
    echo "  ${GREEN}start mem0${NC}            启动 Mem0（含依赖）"
    echo "  ${GREEN}start pi-sandbox${NC}     启动 Pi Sandbox（Pi Agent HTTP 网关）"
    echo "  ${GREEN}start all${NC}             启动全部服务"
    echo ""
    echo -e "${CYAN}━━━ 单个服务（支持简写）━━━━${NC}"
    echo "  ${GREEN}start pg${NC}    启动 PostgreSQL"
    echo "  ${GREEN}start rd${NC}    启动 Redis"
    echo "  ${GREEN}start es${NC}    启动 Elasticsearch"
    echo "  ${GREEN}start qd${NC}    启动 Qdrant"
    echo "  ${GREEN}start n4j${NC}   启动 Neo4j"
    echo "  ${GREEN}start ol${NC}    启动 Ollama"
    echo "  ${GREEN}start m0${NC}    启动 Mem0"
    echo "  ${GREEN}start pi${NC}    启动 Pi Sandbox"
    echo ""
    echo "  完整名称也可用: postgres, redis, elasticsearch, qdrant, neo4j, ollama, mem0, pi-sandbox"
    echo ""
    echo -e "${CYAN}━━━ 停止服务 ━━━${NC}"
    echo "  ${GREEN}stop${NC} [服务]           停止服务（保留容器和镜像）"
    echo "  ${GREEN}stop all${NC}             停止所有服务"
    echo ""
    echo -e "${CYAN}━━━ 清理服务 ━━━${NC}"
    echo "  ${GREEN}clean${NC} [服务]          清理服务（删除容器、网络和镜像）"
    echo "  ${GREEN}clean mem0${NC}           清理 Mem0（含镜像重建）"
    echo "  ${GREEN}clean all${NC}            清理所有服务"
    echo ""
    echo -e "${CYAN}━━━ 查看状态 ━━━${NC}"
    echo "  ${GREEN}list${NC}, ${GREEN}ls${NC}              服务包一览与文档链接"
    echo "  ${GREEN}status${NC}                查看所有服务状态"
    echo "  ${GREEN}health${NC}                健康检查"
    echo ""
    echo -e "${CYAN}━━━ 日志查看 ━━━${NC}"
    echo "  ${GREEN}logs${NC} <服务>           查看服务日志（跟随）"
    echo "  例: ${YELLOW}logs mem0${NC}       查看 Mem0 日志"
    echo "  例: ${YELLOW}logs postgres${NC}   查看 PostgreSQL 日志"
    echo ""
    echo -e "${CYAN}━━━ 依赖确保（Profile / 模块化部署）━━━${NC}"
    echo "  ${GREEN}ensure${NC} <服务...>        确保中间件/服务已运行（不重复启动）"
    echo "  例: ${YELLOW}${STACK_INVOKER} ensure postgresql ollama${NC}"
    echo "  例: ${YELLOW}${STACK_INVOKER} ensure mem0${NC}"
    echo ""
    echo -e "${CYAN}━━━ 其他命令 ━━━${NC}"
    echo "  ${GREEN}restart${NC} [服务]        重启服务"
    echo "  ${GREEN}help${NC}, ${GREEN}--help${NC}, ${GREEN}-h${NC}    显示此帮助信息"
    echo ""

    echo -e "${CYAN}━━━ 可用服务列表 ━━━${NC}"
    echo ""
    echo -e "  ${YELLOW}基础中间件:${NC}"
    echo "    postgres       PostgreSQL + pgvector (端口: 5432)"
    echo "    redis          Redis 缓存 (宿主机端口: 26379)"
    echo "    elasticsearch  Elasticsearch 向量存储 (宿主机端口: 29200)"
    echo "    qdrant         Qdrant 向量数据库 (端口: 6333)"
    echo "    neo4j          Neo4j 图数据库 (端口: 7474/7687)"
    echo "    ollama         Ollama 本地模型 (宿主机端口: 21434)"
    echo ""
    echo -e "  ${YELLOW}第三方服务:${NC}"
    echo "    mem0           Mem0 记忆服务 (端口: 20061)"
    echo "                   依赖: postgresql, qdrant, neo4j"
    echo "    pi-sandbox     Pi Agent 沙盒 HTTP 网关 (端口: 20062，独立部署)"
    echo "                   需在 .env 配置至少一个 LLM API Key；详见 docs/pi-sandbox-guide.md"
    echo ""
    echo -e "${CYAN}━━━ 快速开始 ━━━${NC}"
    echo "  ${GREEN}1.${NC} 交互式向导: ${YELLOW}${STACK_INVOKER} w${NC}"
    echo "  ${GREEN}2.${NC} 选择服务组合并启动"
    echo "  ${GREEN}3.${NC} 查看状态: ${YELLOW}${STACK_INVOKER} status${NC}"
    echo ""

    echo -e "${CYAN}━━━ 示例 ━━━${NC}"
    echo "  ${STACK_INVOKER} w                    # 交互式向导启动（推荐）"
    echo "  ${STACK_INVOKER} start base           # 启动所有基础中间件"
    echo "  ${STACK_INVOKER} start mem0           # 启动 Mem0（含依赖）"
    echo "  ${STACK_INVOKER} start pi-sandbox     # 启动 Pi Sandbox"
    echo "  ${STACK_INVOKER} start all            # 启动全部服务"
    echo "  ${STACK_INVOKER} stop all             # 停止所有服务"
    echo "  ${STACK_INVOKER} logs mem0            # 查看 Mem0 日志"
    echo "  ${STACK_INVOKER} status               # 查看状态"
    echo ""
}

show_list() {
    echo ""
    echo -e "${BLUE}Centag Stack 服务包（各自独立，由 start.sh 统一编排）${NC}"
    echo ""
    echo -e "${CYAN}middleware/${NC} — 基础中间件（PostgreSQL、Redis、Qdrant 等）"
    echo "  启动: ${STACK_INVOKER} start base | ${STACK_INVOKER} start pg"
    echo "  文档: docs/middleware-guide.md"
    echo ""
    echo -e "${CYAN}services/mem0/${NC} — Mem0 记忆服务（需 middleware 中的 PG/Qdrant/Neo4j/Ollama）"
    echo "  启动: ${STACK_INVOKER} start mem0"
    echo "  文档: docs/mem0-guide.md"
    echo ""
    echo -e "${CYAN}services/pi-sandbox/${NC} — Pi Agent HTTP 网关（无 stack 内依赖）"
    echo "  启动: ${STACK_INVOKER} start pi-sandbox"
    echo "  开发: cd services/pi-sandbox && ./scripts/pi-docker.sh run"
    echo "  文档: docs/pi-sandbox-guide.md"
    echo ""
    echo "完整文档索引: docs/README.md"
    echo ""
}

# =============================================================================
# 主命令处理
# =============================================================================

resolve_service() {
    local input="$1"
    case "$input" in
        pg|postgres|post|postgresql) echo "postgresql" ;;
        rd|redis) echo "redis" ;;
        es|elastic|elasticsearch) echo "elasticsearch" ;;
        qd|qdrant) echo "qdrant" ;;
        n4j|neo4j|neo) echo "neo4j" ;;
        ol|ollama) echo "ollama" ;;
        m0|mem0|mem0-server) echo "mem0" ;;
        pi|pisandbox|pisb|pi-sandbox) echo "pi-sandbox" ;;
        base|all) echo "$input" ;;
        *) echo "$input" ;;
    esac
}

cmd_start() {
    local service
    service=$(resolve_service "${1:-}")

    ensure_env_file

    case "$service" in
        base)
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack --profile base up --remove-orphans -d
            ;;
        mem0)
            start_mem0
            ;;
        pi-sandbox)
            start_pi_sandbox
            ;;
        all)
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack --profile base up --remove-orphans -d
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack up --remove-orphans -d ollama
            for svc in postgresql qdrant neo4j ollama; do
                wait_for_dependency "$svc" 120 || exit 1
            done
            start_mem0
            start_pi_sandbox
            print_success "全部服务已启动"
            ;;
        postgresql|redis|elasticsearch|qdrant|neo4j|ollama)
            docker compose --env-file "${ENV_FILE}" -f "${MIDDLEWARE_DIR}/docker-compose.yml" -p centag-stack up --remove-orphans -d "$service"
            ;;
        *)
            print_error "未知服务: $service"
            echo "可用服务: postgresql/pg, redis/rd, elasticsearch/es, qdrant/qd, neo4j/n4j, ollama/ol, mem0/m0, pi-sandbox/pi"
            exit 1
            ;;
    esac
}

cmd_logs() {
    local service
    service=$(resolve_service "${1:-}")
    local compose_file=""

    case "$service" in
        mem0)
            compose_file="${SERVICES_DIR}/mem0/docker-compose.yml"
            ;;
        pi-sandbox)
            compose_file="${SERVICES_DIR}/pi-sandbox/docker-compose.yml"
            ;;
        postgres|redis|elasticsearch|qdrant|neo4j|ollama)
            compose_file="${MIDDLEWARE_DIR}/docker-compose.yml"
            ;;
        *)
            print_error "请指定服务名"
            echo "可用服务: postgres/pg, redis/rd, elasticsearch/es, qdrant/qd, neo4j/n4j, ollama/ol, mem0/m0, pi-sandbox/pi"
            exit 1
            ;;
    esac

    if [ -n "$compose_file" ]; then
        docker compose --env-file "${ENV_FILE}" -f "$compose_file" logs -f "$service"
    fi
}

stack_main() {
    local command="${1:-}"
    local arg="${2:-}"
    # 保留完整参数列表供 ensure 等子命令使用

    if [ -z "$command" ]; then
        show_help
        exit 0
    fi

    case "$command" in
        list|ls|help|--help|-h)
            ;;
        *)
            check_dependencies
            ;;
    esac

    case "$command" in
        w|wizard)
            run_wizard
            ;;
        start)
            cmd_start "$arg"
            ;;
        stop)
            stop_service "$arg"
            ;;
        restart)
            stop_service "$arg"
            sleep 2
            cmd_start "$arg"
            ;;
        clean)
            clean_service "$arg"
            ;;
        logs)
            cmd_logs "$arg"
            ;;
        status)
            show_status
            ;;
        health)
            show_health
            ;;
        ensure)
            if [ $# -lt 2 ]; then
                print_error "用法: ${STACK_INVOKER} ensure <服务...>"
                echo "  例: ${STACK_INVOKER} ensure postgresql ollama"
                echo "  例: ${STACK_INVOKER} ensure mem0"
                exit 1
            fi
            stack_ensure_services "${@:2}"
            ;;
        list|ls)
            show_list
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知命令: $command"
            show_help
            exit 1
            ;;
    esac
}
