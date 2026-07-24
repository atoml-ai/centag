#!/usr/bin/env bash
# Build Centag desktop launcher (apps/launcher).
#
# Default: lite (no CGO, cross-compile OK) → centag-launcher
# Tray:    --tray / CENTAG_LAUNCHER_TRAY=1 (CGO + systray) → centag-launcher-tray
#
# Output (install-compatible root):
#   ~/.centag/var/cross/launcher/<goos>-<goarch>/centag-launcher[.exe]
#   ~/.centag/bin/centag-launcher[.exe]   # host convenience (active variant)
#
# Override root: CENTAG_INSTALL_ROOT
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init

LAUNCHER_DIR="${ROOT}/apps/launcher"
CROSS_ROOT="${CENTAG_CROSS_DIR}/launcher"

TRAY=0
if [[ "${1:-}" == "--tray" ]] || [[ "${CENTAG_LAUNCHER_TRAY:-}" == "1" ]]; then
  TRAY=1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is required" >&2
  exit 1
fi

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
if [[ -n "${CENTAG_LAUNCHER_GOOS:-}" ]]; then
  GOOS="${CENTAG_LAUNCHER_GOOS}"
fi
if [[ -n "${CENTAG_LAUNCHER_GOARCH:-}" ]]; then
  GOARCH="${CENTAG_LAUNCHER_GOARCH}"
fi

case "$GOOS" in
  darwin|linux|windows) ;;
  *)
    echo "error: unsupported GOOS=$GOOS (launcher supports darwin|linux|windows)" >&2
    exit 1
    ;;
esac

HOST_OS="$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]' || echo unknown)"
HOST_ARCH="$(uname -m 2>/dev/null || echo unknown)"

EXT=""
if [[ "$GOOS" == "windows" ]]; then
  EXT=".exe"
fi

OUT_DIR="${CROSS_ROOT}/${GOOS}-${GOARCH}"
mkdir -p "${OUT_DIR}" "${CENTAG_BIN_DIR}"

if [[ "$TRAY" == "1" ]]; then
  OUT_BIN="${OUT_DIR}/centag-launcher-tray${EXT}"
  LATEST_BIN="${CENTAG_BIN_DIR}/centag-launcher-tray${EXT}"
  echo "==> launcher tray host: ${HOST_OS}/${HOST_ARCH} → target ${GOOS}/${GOARCH} (CGO)" >&2
  if [[ "${GOOS}" != "$(go env GOOS)" || "${GOARCH}" != "$(go env GOARCH)" ]]; then
    echo "    warn: tray/systray cross-compile often fails — prefer building on the target OS" >&2
  fi
  echo "==> building centag-launcher-tray → ${OUT_BIN}" >&2
  (
    cd "${LAUNCHER_DIR}"
    GOWORK=off CGO_ENABLED=1 GOOS="${GOOS}" GOARCH="${GOARCH}" \
      go build -tags tray -trimpath -ldflags="-s -w" -o "${OUT_BIN}" .
  )
else
  OUT_BIN="${OUT_DIR}/centag-launcher${EXT}"
  LATEST_BIN="${CENTAG_BIN_DIR}/centag-launcher${EXT}"
  echo "==> launcher lite host: ${HOST_OS}/${HOST_ARCH} → target ${GOOS}/${GOARCH} (CGO_ENABLED=0)" >&2
  echo "==> building centag-launcher (lite) → ${OUT_BIN}" >&2
  (
    cd "${LAUNCHER_DIR}"
    GOWORK=off CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
      go build -trimpath -ldflags="-s -w" -o "${OUT_BIN}" .
  )
fi

HOST_GOOS="$(go env GOOS)"
HOST_GOARCH="$(go env GOARCH)"
if [[ "$GOOS" == "$HOST_GOOS" && "$GOARCH" == "$HOST_GOARCH" ]]; then
  cp -f "${OUT_BIN}" "${LATEST_BIN}"
  echo "OK: ${LATEST_BIN} (install bin/)" >&2
fi
echo "OK: ${OUT_BIN}" >&2
# stdout: path only (for callers that capture)
echo "${OUT_BIN}"
