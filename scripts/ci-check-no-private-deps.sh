#!/usr/bin/env bash
# E4 / R01: open-core must never import the private centag-pro module.
# D6 / R09: internal dev docs & agent entries must not return to open-core.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

tracked() { git ls-files -- "$1" | grep -q .; }

echo "== no private module imports =="
if grep -rn --include='*.go' --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=var 'github.com/atoml-ai/centag-pro' .; then
  echo "error: open-core Go sources must not import github.com/atoml-ai/centag-pro" >&2
  exit 1
fi

echo "== no OSS teamadmin package =="
if [[ -d core/pkg/teamadmin ]]; then
  echo "error: core/pkg/teamadmin must not exist in open-core (D6)" >&2
  exit 1
fi

echo "== no internal dev docs / agent entries (back-flow guard) =="
forbidden=(
  'docs/harness'
  'docs/versions'
  'docs/architecture'
  '.opencode'
  '.cursor'
  'AGENT.md'
  'docs/guide/mode-behavior-matrix.md'
  'docs/guide/deployment-profiles-and-stack.md'
  'docs/guide/dist-profiles.md'
  'docs/guide/external-business-plugins.md'
  'docs/guide/multi-tenant.md'
  'docs/guide/billing.md'
  'deploy/stack/docs/AGENTS.md'
)
fail=0
for p in "${forbidden[@]}"; do
  if tracked "$p"; then
    echo "error: forbidden path tracked in open-core: $p" >&2
    fail=1
  fi
done

# AGENTS.md only allowed at root and config/initdata (runtime data 说明)
while IFS= read -r f; do
  case "$f" in
    AGENTS.md|config/initdata/AGENTS.md) ;;
    *)
      echo "error: unexpected AGENTS.md tracked in open-core: $f" >&2
      fail=1
      ;;
  esac
done < <(git ls-files -- '*AGENTS.md')

if [[ "$fail" -ne 0 ]]; then
  exit 1
fi

echo "ok: no private module imports; no OSS teamadmin; no internal docs/agent entries"
