#!/usr/bin/env bash
# Build GitHub Release assets for install.sh (channel: github).
#
# Strategy:
#   CLI (all platforms, cross-compile OK):
#     centag-cli-personal-{darwin,linux,windows}-{amd64,arm64}.tar.gz
#   Desktop (host/native CGO only; optional):
#     centag-desktop-personal-macos-<arch>.{dmg,zip}
#     centag-desktop-personal-windows-<arch>.zip
#
# install.sh defaults to CLI on every OS; use --desktop for desktop packages.
# npm channel is separate (also full-matrix CLI).
#
# Usage:
#   ./scripts/release/build-github-artifacts.sh [--version 0.2.9] [--skip-frontend]
#   CENTAG_RELEASE_GITHUB_DESKTOP=0  # CLI only (skip host desktop)
#
# Output: ~/.centag/var/release/<version>/
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init

log() { echo "==> $*" >&2; }
fail() { echo "error: $*" >&2; exit 1; }

VERSION=""
SKIP_FRONTEND=0
BUILD_DESKTOP="${CENTAG_RELEASE_GITHUB_DESKTOP:-1}"
CLI_PLATFORMS="${CENTAG_RELEASE_GITHUB_CLI_PLATFORMS:-darwin-amd64,darwin-arm64,linux-amd64,linux-arm64,windows-amd64,windows-arm64}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --skip-frontend) SKIP_FRONTEND=1; shift ;;
    --no-desktop) BUILD_DESKTOP=0; shift ;;
    -h|--help)
      sed -n '2,20p' "$0"
      exit 0
      ;;
    *) fail "unknown arg: $1" ;;
  esac
done

VER_ARGS=()
if [[ -n "$VERSION" ]]; then
  VER_ARGS+=(--version "$VERSION")
fi

command -v go >/dev/null 2>&1 || fail "go is required"
HOST_GOOS="$(go env GOOS)"

log "GitHub artifacts: full CLI matrix + host desktop (GOOS=${HOST_GOOS})"

# 1) CLI — all platforms (install.sh default)
CLI_ARGS=("${VER_ARGS[@]}" --components personal --platforms "${CLI_PLATFORMS}")
if [[ "$SKIP_FRONTEND" == "1" ]]; then
  CLI_ARGS+=(--skip-frontend)
fi
bash "${ROOT}/scripts/release/build-artifacts.sh" "${CLI_ARGS[@]}" >/dev/null

# Resolve OUT_DIR / version after CLI build
if [[ -z "$VERSION" ]]; then
  if [[ -f "${ROOT}/apps/wrap-npm/package.json" ]] && command -v node >/dev/null 2>&1; then
    VERSION="$(node -p "require('${ROOT}/apps/wrap-npm/package.json').version")"
  fi
fi
[[ -n "$VERSION" ]] || fail "version required"
VERSION="${VERSION#v}"
OUT_DIR="${CENTAG_RELEASE_DIR}/${VERSION}"
[[ -d "$OUT_DIR" ]] || fail "release dir missing after CLI build: $OUT_DIR"

# 2) Host desktop package (darwin/windows only; optional)
if [[ "$BUILD_DESKTOP" == "1" ]]; then
  case "$HOST_GOOS" in
    darwin|windows)
      DESK_ARGS=("${VER_ARGS[@]}" --edition personal --skip-frontend)
      bash "${ROOT}/scripts/release/package-desktop.sh" "${DESK_ARGS[@]}" >/dev/null
      ;;
    linux)
      log "host is linux: skipping desktop (CI builds darwin/windows on native runners)"
      ;;
    *)
      log "warn: unsupported host GOOS=${HOST_GOOS}; desktop skipped"
      ;;
  esac
else
  log "desktop build disabled (--no-desktop / CENTAG_RELEASE_GITHUB_DESKTOP=0)"
fi

# 3) Unified checksums for GitHub assets
sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

log "checksums.txt (GitHub assets)"
: > "${OUT_DIR}/checksums.txt"
while IFS= read -r path; do
  [[ -f "$path" ]] || continue
  base="$(basename "$path")"
  printf '%s  %s\n' "$(sha256_of "$path")" "$base" >> "${OUT_DIR}/checksums.txt"
done < <(find "$OUT_DIR" -maxdepth 1 -type f \( \
  -name 'centag-cli-*.tar.gz' -o \
  -name 'centag-desktop-*.dmg' -o \
  -name 'centag-desktop-*.zip' -o \
  -name 'centag-personal-*.tar.gz' -o \
  -name 'Centag-*.dmg' -o \
  -name 'Centag-*.zip' \
\) | sort)
cat "${OUT_DIR}/checksums.txt" >&2

log "GitHub artifacts in ${OUT_DIR}"
ls -lh "$OUT_DIR" >&2
echo "$OUT_DIR"
