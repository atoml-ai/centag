# shellcheck shell=bash
# Centag Stack 编排库 — 引导（由 centag-stack/start.sh 或 centag/start.sh stack 加载）
# 加载前可设置: STACK_ROOT（stack 仓库根目录）、STACK_INVOKER（帮助文案中的入口脚本名）

if [[ -n "${STACK_LIB_LOADED:-}" ]]; then
    return 0 2>/dev/null || exit 0
fi
STACK_LIB_LOADED=1

: "${STACK_ROOT:=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
: "${STACK_INVOKER:=./start.sh}"

readonly STACK_PROJECT_ROOT="$STACK_ROOT"
# 独立 bash 进程加载时设置；若已被主仓占用则仅用于 stack 路径
if [[ -z "${PROJECT_ROOT:-}" ]]; then
    readonly PROJECT_ROOT="$STACK_PROJECT_ROOT"
fi
readonly MIDDLEWARE_DIR="${STACK_PROJECT_ROOT}/middleware"
readonly SERVICES_DIR="${STACK_PROJECT_ROOT}/services"
readonly ENV_FILE="${STACK_PROJECT_ROOT}/.env"

if [[ -z "${STACK_QUIET_CD:-}" ]]; then
    cd "$STACK_PROJECT_ROOT"
fi

# 颜色与输出（若主仓已定义 print_* 则复用）
if ! declare -f print_info >/dev/null 2>&1; then
    readonly RED=$'\033[0;31m'
    readonly GREEN=$'\033[0;32m'
    readonly YELLOW=$'\033[1;33m'
    readonly BLUE=$'\033[0;34m'
    readonly CYAN=$'\033[0;36m'
    readonly NC=$'\033[0m'
    print_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
    print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
    print_error()   { echo -e "${RED}[ERROR]${NC} $1"; }
    print_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
fi