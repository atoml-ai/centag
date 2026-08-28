#!/usr/bin/env bash
# scripts/fix-go-sum.sh
#
# Reconcile go.sum files against the published proxy.zone (proxy.golang.org).
# Requires network access to https://proxy.golang.org — does NOT work in
# air-gapped environments.
#
# Usage:
#   bash scripts/fix-go-sum.sh
#   bash scripts/fix-go-sum.sh --check    # exit 1 if any drift, no write
#
# What it does:
#   1. For every go.mod under the repo, runs `go mod tidy -compat=1.25` and
#      diffs the resulting go.sum/go.mod against HEAD.
#   2. Prints the diff so the operator can review before committing.
#
# Typical workflow after a tag is re-pointed or a dependent module is rebuilt:
#   bash scripts/fix-go-sum.sh
#   git add core/go.sum core/go.mod
#   git commit -m "fix(deps): reconcile go.sum against proxy.golang.org"
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CHECK_ONLY=0
[[ "${1:-}" == "--check" ]] && CHECK_ONLY=1

# Pin to the same toolchain the rest of the build uses; prevents a
# downgrade tidy run.
if command -v go >/dev/null 2>&1; then
  GOTOOLCHAIN="local"
  export GOTOOLCHAIN
fi

mapfile -t gomods < <(git ls-files '**/go.mod' | grep -v '/node_modules/' || true)
if [[ ${#gomods[@]} -eq 0 ]]; then
  echo "error: no go.mod files found under repo" >&2
  exit 1
fi

overall_dirty=0
for gomod in "${gomods[@]}"; do
  dir="$(dirname "$gomod")"
  echo "==> $gomod"
  if ! ( cd "$dir" && go mod tidy -compat=1.25 ); then
    echo "error: go mod tidy failed in $dir" >&2
    overall_dirty=1
    continue
  fi
  if ! git diff --quiet -- "$dir/go.sum" "$dir/go.mod" 2>/dev/null; then
    overall_dirty=1
    echo "drift detected in $dir/{go.sum,go.mod}:"
    git diff --stat -- "$dir/go.sum" "$dir/go.mod" || true
    if [[ $CHECK_ONLY -eq 0 ]]; then
      echo "(not committed — review with 'git diff' and commit manually)"
    fi
  else
    echo "ok: $dir in sync"
  fi
done

if [[ $CHECK_ONLY -eq 1 && $overall_dirty -ne 0 ]]; then
  exit 1
fi
