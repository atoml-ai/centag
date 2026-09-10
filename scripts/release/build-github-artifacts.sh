#!/usr/bin/env bash
# Build GitHub Release assets for install.sh (channel: github).
#
# Strategy:
#   CLI (linux only; cross-compile OK):
#     centag-cli-personal-linux-{amd64,arm64}.tar.gz
#   Desktop (host/native CGO only; optional — darwin/windows built on native CI runners):
#     centag-desktop-personal-macos-<arch>.{dmg,zip}
#     centag-desktop-personal-windows-<arch>.zip
#   fnOS (native mode):
#     centag-personal-native-<arch>.fpk
#
# GitHub channel: Win/mac ship desktop only (no CLI); Linux ships CLI. install.sh
# falls back to desktop on darwin/windows. npm channel is separate (full-matrix CLI).
#
# Usage:
#   ./scripts/release/build-github-artifacts.sh [--version 0.2.9] [--skip-frontend]
#   CENTAG_RELEASE_GITHUB_DESKTOP=0  # CLI only (skip host desktop)
#   CENTAG_RELEASE_GITHUB_FNOS=1     # Build fnOS packages
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
BUILD_FNOS="${CENTAG_RELEASE_GITHUB_FNOS:-0}"
CLI_PLATFORMS="${CENTAG_RELEASE_GITHUB_CLI_PLATFORMS:-linux-amd64,linux-arm64}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --skip-frontend) SKIP_FRONTEND=1; shift ;;
    --no-desktop) BUILD_DESKTOP=0; shift ;;
    --fnos) BUILD_FNOS=1; shift ;;
    -h|--help)
      sed -n '2,25p' "$0"
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

# 1) CLI — linux only (install.sh on linux; npm channel covers win/mac CLI)
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

# 3) Windows desktop cross-compile (amd64 + arm64) — can run on any host (pure Go, no CGO)
# Note: Windows/Linux systray is pure Go (no CGO needed), only macOS needs CGO for systray_darwin.m
if [[ "$BUILD_DESKTOP" == "1" ]]; then
  log "building Windows desktop packages"
  for WIN_ARCH in amd64 arm64; do
    log "building Windows desktop ${WIN_ARCH}"
    (
      cd "${ROOT}"
      # Build Windows desktop binary (pure Go, no CGO)
      CENTAG_LAUNCHER_GOOS=windows CENTAG_LAUNCHER_GOARCH="$WIN_ARCH" \
        bash scripts/build-launcher.sh --desktop 2>&1 | tail -3

      # Package Windows desktop with sidecar tree
      STAGE_DIR="${OUT_DIR}/.stage-windows-${WIN_ARCH}"
      rm -rf "$STAGE_DIR"
      mkdir -p "$STAGE_DIR/Centag"

      # Copy the desktop binary
      cp -f "${CENTAG_CROSS_DIR}/launcher/windows-${WIN_ARCH}/centag-desktop.exe" "$STAGE_DIR/Centag/"

      # Build and copy sidecar (CLI binary + static + config)
      SIDECAR_OUT="${OUT_DIR}/.build/sidecar-windows-${WIN_ARCH}"
      mkdir -p "$SIDECAR_OUT"
      (
        cd "${ROOT}/dist/personal"
        GOWORK=off CGO_ENABLED=0 GOOS=windows GOARCH="$WIN_ARCH" \
          go build -trimpath -tags "protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure" \
          -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')'" \
          -o "$SIDECAR_OUT/centag-personal.exe" .
      )
      cp -f "$SIDECAR_OUT/centag-personal.exe" "$STAGE_DIR/Centag/"

      # Copy static assets
      cp -R "$CENTAG_STATIC_DIR" "$STAGE_DIR/Centag/static"

      # Copy config/initdata
      if [[ -d "${ROOT}/config/initdata" ]]; then
        mkdir -p "$STAGE_DIR/Centag/config"
        cp -R "${ROOT}/config/initdata" "$STAGE_DIR/Centag/config/initdata"
        rm -rf "$STAGE_DIR/Centag/config/initdata/postgresql" \
          "$STAGE_DIR/Centag/config/initdata/scripts" \
          "$STAGE_DIR/Centag/config/initdata/update" \
          "$STAGE_DIR/Centag/config/initdata/secrets" 2>/dev/null || true
        find "$STAGE_DIR/Centag/config/initdata" \( -name 'README.md' -o -name 'AGENTS.md' \) -delete 2>/dev/null || true
        rm -rf "$STAGE_DIR/Centag/config/initdata/pipeline-templates/personal" \
          "$STAGE_DIR/Centag/config/initdata/pipeline-templates/team"
      fi

      # Create zip
      (
        cd "$STAGE_DIR"
        zip -qr "${OUT_DIR}/centag-desktop-personal-windows-${WIN_ARCH}.zip" Centag
      )
      rm -rf "$STAGE_DIR" "$SIDECAR_OUT"
      log "OK ${OUT_DIR}/centag-desktop-personal-windows-${WIN_ARCH}.zip"
    ) || log "warn: Windows desktop ${WIN_ARCH} build failed"
  done
fi

# 4) fnOS packages (native mode; optional)
if [[ "$BUILD_FNOS" == "1" ]]; then
  log "building fnOS packages"
  for FNOS_ARCH in amd64 arm64; do
    log "building fnOS native ${FNOS_ARCH}"
    (
      cd "${ROOT}"
      bash deploy/fnos/build-fpk.sh \
        --mode native \
        --edition personal \
        --arch "$FNOS_ARCH" \
        --output "${OUT_DIR}" 2>&1 | tail -5
    ) || log "warn: fnOS ${FNOS_ARCH} build failed"
  done
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
  -name 'centag-personal-*.fpk' -o \
  -name 'Centag-*.dmg' -o \
  -name 'Centag-*.zip' \
\) | sort)
cat "${OUT_DIR}/checksums.txt" >&2

log "GitHub artifacts in ${OUT_DIR}"
ls -lh "$OUT_DIR" >&2
echo "$OUT_DIR"
