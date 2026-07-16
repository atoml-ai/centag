#!/bin/bash
# Centag clang wrapper — strips -no_warn_duplicate_libraries
# Required for Go 1.24+ with Apple CLT < 15 (macOS Ventura or older)
set -euo pipefail

args=()
for arg in "$@"; do
    if [[ "$arg" == "-Wl,-no_warn_duplicate_libraries" ]]; then
        continue
    fi
    args+=("$arg")
done
exec /usr/bin/clang "${args[@]}"
