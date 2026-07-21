#!/usr/bin/env bash
# E4 / R01: open-core must never import the private centag-pro module.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

pattern='github.com/atoml-ai/centag-pro'
if rg -n --glob '*.go' "$pattern" .; then
  echo "error: open-core Go sources must not import $pattern" >&2
  exit 1
fi

if [[ -d core/pkg/teamadmin ]]; then
  echo "error: core/pkg/teamadmin must not exist in open-core (D6)" >&2
  exit 1
fi

echo "ok: no private module imports; no OSS teamadmin package"
