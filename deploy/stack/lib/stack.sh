# shellcheck shell=bash
# Centag Stack 编排库入口 — 由 start.sh 或 centag/start.sh stack 加载

STACK_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=stack-bootstrap.sh
source "${STACK_LIB_DIR}/stack-bootstrap.sh"
# shellcheck source=stack-core.sh
source "${STACK_LIB_DIR}/stack-core.sh"
# shellcheck source=stack-ensure.sh
source "${STACK_LIB_DIR}/stack-ensure.sh"