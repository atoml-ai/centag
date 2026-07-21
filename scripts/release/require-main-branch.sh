#!/usr/bin/env bash
# Deprecated alias — use require-release-branch.sh (version branch gate).
# Kept so older docs/calls do not break; forwards all args.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
echo "warn: require-main-branch.sh is deprecated; use require-release-branch.sh" >&2
exec bash "${ROOT}/scripts/release/require-release-branch.sh" "$@"
