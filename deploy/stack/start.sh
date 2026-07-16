#!/bin/bash
# Centag Stack — 薄入口（编排逻辑在 lib/stack.sh，主仓通过 ./start.sh stack 同源加载）
set -euo pipefail

STACK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STACK_INVOKER="./start.sh"

# shellcheck source=lib/stack.sh
source "${STACK_ROOT}/lib/stack.sh"

stack_main "$@"