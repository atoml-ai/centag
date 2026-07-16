#!/bin/bash
# Compatibility wrapper: moved to scripts/ops/load-secrets.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

exec "${SCRIPT_DIR}/ops/load-secrets.sh" "$@"
