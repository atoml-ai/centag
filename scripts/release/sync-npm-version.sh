#!/usr/bin/env bash
# Sync npm package.json versions to the release version (no leading v).
#
# Files:
#   apps/centag-npm/package.json
#   apps/centag-npm/package.offline.json
#   apps/wrap-npm/package.json
#
# Usage:
#   bash scripts/release/sync-npm-version.sh --version 0.3.0
#   bash scripts/release/sync-npm-version.sh --version 0.3.0 --check   # exit 1 if mismatch (no write)
#
# Env:
#   CENTAG_RELEASE_VERSION  used if --version omitted
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

VERSION="${CENTAG_RELEASE_VERSION:-}"
CHECK=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --check)
      CHECK=1
      shift
      ;;
    -h|--help)
      sed -n '2,16p' "$0"
      exit 0
      ;;
    *)
      echo "error: unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

VERSION="${VERSION#v}"
[[ -n "$VERSION" ]] || { echo "error: --version / CENTAG_RELEASE_VERSION required" >&2; exit 1; }
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-].*)?$ ]] \
  || { echo "error: invalid version: ${VERSION}" >&2; exit 1; }

FILES=(
  apps/centag-npm/package.json
  apps/centag-npm/package.offline.json
  apps/wrap-npm/package.json
)

if [[ "$CHECK" == "1" ]]; then
  mism=0
  for f in "${FILES[@]}"; do
    cur="$(node -p "require('./${f}').version")"
    if [[ "$cur" != "$VERSION" ]]; then
      echo "mismatch: ${f} has ${cur}, want ${VERSION}" >&2
      mism=1
    else
      echo "ok: ${f} = ${cur}"
    fi
  done
  [[ "$mism" == "0" ]] || exit 1
  exit 0
fi

node -e '
const fs = require("fs");
const ver = process.argv[1];
const files = process.argv.slice(2);
for (const f of files) {
  const j = JSON.parse(fs.readFileSync(f, "utf8"));
  const prev = j.version;
  j.version = ver;
  fs.writeFileSync(f, JSON.stringify(j, null, 2) + "\n");
  console.log(`updated ${f}: ${prev} → ${ver}`);
}
' "$VERSION" "${FILES[@]}"

echo "OK: npm package versions → ${VERSION}"
