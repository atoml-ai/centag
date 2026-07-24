#!/usr/bin/env bash
# Build GitHub Release assets for install.sh (channel: github).
#
# Strategy (product form):
#   linux   → personal CLI tarball (cross-compile OK)
#   darwin  → tray desktop (Centag.app + .zip + .dmg) — host/native only
#   windows → tray desktop (Centag.zip) — host/native only
#
# npm channel is separate and still ships CLI for all platforms
# (see scripts/publish-centag-npm.sh / release.yml npm-publish).
#
# Usage:
#   ./scripts/release/build-github-artifacts.sh [--version 0.2.9] [--skip-frontend]
#   CENTAG_RELEASE_GITHUB_DESKTOP=0  # linux CLI only (skip host desktop)
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
EXTRA=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --skip-frontend) SKIP_FRONTEND=1; EXTRA+=(--skip-frontend); shift ;;
    --no-desktop) BUILD_DESKTOP=0; shift ;;
    -h|--help)
      sed -n '2,18p' "$0"
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

log "GitHub artifacts: linux CLI + host desktop (GOOS=${HOST_GOOS})"

# 1) Linux CLI (both arches) — shared with historical asset naming for install.sh
LINUX_ARGS=("${VER_ARGS[@]}" --components personal --platforms linux-amd64,linux-arm64)
if [[ "$SKIP_FRONTEND" == "1" ]]; then
  LINUX_ARGS+=(--skip-frontend)
fi
bash "${ROOT}/scripts/release/build-artifacts.sh" "${LINUX_ARGS[@]}" >/dev/null

# Resolve OUT_DIR / version after linux build
if [[ -z "$VERSION" ]]; then
  if [[ -f "${ROOT}/apps/wrap-npm/package.json" ]] && command -v node >/dev/null 2>&1; then
    VERSION="$(node -p "require('${ROOT}/apps/wrap-npm/package.json').version")"
  fi
fi
[[ -n "$VERSION" ]] || fail "version required"
VERSION="${VERSION#v}"
OUT_DIR="${CENTAG_RELEASE_DIR}/${VERSION}"
[[ -d "$OUT_DIR" ]] || fail "release dir missing after linux build: $OUT_DIR"

# 2) Host desktop tray package (darwin/windows only)
if [[ "$BUILD_DESKTOP" == "1" ]]; then
  case "$HOST_GOOS" in
    darwin|windows)
      DESK_ARGS=("${VER_ARGS[@]}" --edition personal)
      # Frontend already built by build-artifacts above into CENTAG_STATIC_DIR
      DESK_ARGS+=(--skip-frontend)
      bash "${ROOT}/scripts/release/package-desktop.sh" "${DESK_ARGS[@]}" >/dev/null
      ;;
    linux)
      log "host is linux: skipping desktop tray (CI should build darwin/windows on native runners)"
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
  -name 'centag-cli-*-linux-*.tar.gz' -o \
  -name 'centag-desktop-*.dmg' -o \
  -name 'centag-desktop-*.zip' -o \
  -name 'centag-personal-linux-*.tar.gz' -o \
  -name 'Centag-*.dmg' -o \
  -name 'Centag-*.zip' \
\) | sort)
cat "${OUT_DIR}/checksums.txt" >&2

log "GitHub artifacts in ${OUT_DIR}"
ls -lh "$OUT_DIR" >&2
echo "$OUT_DIR"
