#!/usr/bin/env bash
# Build Centag desktop launcher (apps/launcher).
#
# Default: lite (no CGO, cross-compile OK) → centag-launcher
# Tray:    --tray / CENTAG_LAUNCHER_TRAY=1 (CGO + systray) → centag-launcher-tray
#
# Output:
#   bin/launcher/<goos>-<goarch>/centag-launcher[.exe]          # lite
#   bin/launcher/<goos>-<goarch>/centag-launcher-tray[.exe]     # tray
#   bin/launcher/centag-launcher[.exe]                          # host convenience (active variant)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LAUNCHER_DIR="${ROOT}/apps/launcher"
OUT_ROOT="${ROOT}/bin/launcher"

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

OUT_DIR="${OUT_ROOT}/${GOOS}-${GOARCH}"
mkdir -p "${OUT_DIR}"

if [[ "$TRAY" == "1" ]]; then
  OUT_BIN="${OUT_DIR}/centag-launcher-tray${EXT}"
  LATEST_BIN="${OUT_ROOT}/centag-launcher-tray${EXT}"
  # Also refresh convenience "centag-launcher" when building tray on request? Keep separate.
  echo "==> launcher tray host: ${HOST_OS}/${HOST_ARCH} → target ${GOOS}/${GOARCH} (CGO)"
  if [[ "${GOOS}" != "$(go env GOOS)" || "${GOARCH}" != "$(go env GOARCH)" ]]; then
    echo "    warn: tray/systray cross-compile often fails — prefer building on the target OS" >&2
  fi
  echo "==> building centag-launcher-tray → ${OUT_BIN}"
  (
    cd "${LAUNCHER_DIR}"
    GOWORK=off CGO_ENABLED=1 GOOS="${GOOS}" GOARCH="${GOARCH}" \
      go build -tags tray -trimpath -ldflags="-s -w" -o "${OUT_BIN}" .
  )
else
  OUT_BIN="${OUT_DIR}/centag-launcher${EXT}"
  LATEST_BIN="${OUT_ROOT}/centag-launcher${EXT}"
  echo "==> launcher lite host: ${HOST_OS}/${HOST_ARCH} → target ${GOOS}/${GOARCH} (CGO_ENABLED=0)"
  echo "==> building centag-launcher (lite) → ${OUT_BIN}"
  (
    cd "${LAUNCHER_DIR}"
    GOWORK=off CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
      go build -trimpath -ldflags="-s -w" -o "${OUT_BIN}" .
  )
fi

cp -f "${OUT_BIN}" "${LATEST_BIN}"
echo "OK: ${OUT_BIN}"
echo "OK: ${LATEST_BIN} (current-host convenience copy)"
