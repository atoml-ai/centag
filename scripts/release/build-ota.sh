#!/usr/bin/env bash
# Build OTA update packages (personal/minimal) for a GitHub Release with an
# explicit release version. Each package carries the matching Web static so
# OTA does not ship a stale UI. Platforms: linux (fnOS NAS), darwin + windows
# (desktop sidecar self-update; the GUI shell itself still updates via dmg/zip).
#
#   ./scripts/release/build-ota.sh --version 0.2.9
#   ./scripts/release/build-ota.sh --version 0.2.9 --platforms linux-amd64,linux-arm64,darwin-arm64,windows-amd64
#   ./scripts/release/build-ota.sh --version 0.2.9 --skip-frontend
#
# Output (default ~/.centag/var/packages/):
#   update-package-centag-<edition>-<version>-<goos>-<goarch>.tar.gz  + .sha256 / .manifest.json
#
# This is the CI version of `start.sh package ota` (which uses the build
# timestamp as version). CI needs deterministic version + cross-compile.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init

log() { echo "==> $*" >&2; }
fail() { echo "error: $*" >&2; exit 1; }

VERSION=""
EDITION="${CENTAG_PACKAGE_EDITION:-personal}"
PLATFORMS="${CENTAG_OTA_PLATFORMS:-linux-amd64,linux-arm64,darwin-amd64,darwin-arm64,windows-amd64}"
SKIP_FRONTEND=0
OUT_DIR="${CENTAG_PACKAGES_DIR}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --edition) EDITION="${2:-}"; shift 2 ;;
    --platforms) PLATFORMS="${2:-}"; shift 2 ;;
    --skip-frontend) SKIP_FRONTEND=1; shift ;;
    --out-dir) OUT_DIR="${2:-}"; shift 2 ;;
    -h|--help)
      sed -n '2,20p' "$0"
      exit 0
      ;;
    *) fail "unknown arg: $1" ;;
  esac
done

[[ -n "$VERSION" ]] || fail "version required (--version <X.Y.Z>)"
VERSION="${VERSION#v}"
case "$EDITION" in
  personal|minimal) ;;
  *) fail "unsupported edition: $EDITION (want personal|minimal)" ;;
esac

command -v go >/dev/null 2>&1 || fail "go is required"

dist_tags() {
  case "$1" in
    minimal)
      echo "minimal,protocol_openai,protocol_anthropic,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic"
      ;;
    personal)
      echo "protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure"
      ;;
  esac
}

mkdir -p "$OUT_DIR"

# 1) Frontend static (shared across all target platforms)
STATIC_SRC="${CENTAG_STATIC_DIR}"
if [[ "$SKIP_FRONTEND" == "1" ]]; then
  [[ -d "$STATIC_SRC" ]] || fail "frontend static missing at ${STATIC_SRC} (drop --skip-frontend or build web first)"
else
  log "building frontend → ${STATIC_SRC}"
  (
    cd "${ROOT}/web"
    export CENTAG_INSTALL_ROOT CENTAG_EDITION="$EDITION" CENTAG_STATIC_DIR="${STATIC_SRC}"
    if [[ -f package-lock.json ]]; then npm ci; else npm install; fi
    npm run build
  )
  [[ -d "$STATIC_SRC" ]] || fail "frontend static missing at ${STATIC_SRC} after build"
fi

# 2) Per-platform cross-compile + service package
CROSS_DIR="$(mktemp -d)"
trap 'rm -rf "$CROSS_DIR"' EXIT
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

IFS=',' read -r -a PLAT_ARR <<< "$PLATFORMS"
for plat in "${PLAT_ARR[@]}"; do
  plat="$(printf '%s' "$plat" | tr -d '[:space:]')"
  [[ -z "$plat" ]] && continue
  goos="${plat%-*}"
  goarch="${plat##*-}"
  [[ "$goos" == "linux" || "$goos" == "darwin" || "$goos" == "windows" ]] || fail "unsupported goos: ${goos} (want linux|darwin|windows)"
  case "$goarch" in amd64|arm64) ;; *) fail "unsupported arch: ${goarch}" ;; esac

  local_bin="${CROSS_DIR}/centag-${EDITION}-${goos}-${goarch}"
  mkdir -p "$(dirname "$local_bin")"

  log "build centag-${EDITION} ${goos}/${goarch}"
  tags="$(dist_tags "$EDITION")"
  (
    cd "${ROOT}/dist/${EDITION}"
    GOWORK=off CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -tags "$tags" \
      -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}'" \
      -o "$local_bin" .
  )

  log "packaging service update ${goos}/${goarch}"
  bash "${ROOT}/scripts/release/package.sh" service \
    --version "$VERSION" \
    --edition "$EDITION" \
    --goos "$goos" \
    --goarch "$goarch" \
    --build-time "$BUILD_TIME" \
    --source-bin "$local_bin" \
    --source-static "$STATIC_SRC" \
    --out-dir "$OUT_DIR" >/dev/null
done

log "OTA packages in ${OUT_DIR}"
ls -lh "${OUT_DIR}" >&2
