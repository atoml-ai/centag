#!/usr/bin/env bash
# Build the optional Centag desktop launcher (apps/launcher) for the *current* host OS/arch.
# energye/systray needs CGO — native build only (no reliable cross-compile).
#
# Supported hosts: darwin (macOS), linux, windows
# Output:
#   bin/launcher/<goos>-<goarch>/centag-launcher[.exe]
#   bin/launcher/centag-launcher[.exe]   (convenience copy for current host)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LAUNCHER_DIR="${ROOT}/apps/launcher"
OUT_ROOT="${ROOT}/bin/launcher"

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is required" >&2
  exit 1
fi

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
# Allow explicit override, but warn — CGO cross builds usually fail for systray.
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
echo "==> launcher host: ${HOST_OS}/${HOST_ARCH} → target ${GOOS}/${GOARCH}"

if [[ "${GOOS}" != "$(go env GOOS)" || "${GOARCH}" != "$(go env GOARCH)" ]]; then
  echo "    warn: cross-compile requested; CGO/systray often fails — prefer building on the target OS"
fi

EXT=""
if [[ "$GOOS" == "windows" ]]; then
  EXT=".exe"
fi

OUT_DIR="${OUT_ROOT}/${GOOS}-${GOARCH}"
OUT_BIN="${OUT_DIR}/centag-launcher${EXT}"
LATEST_BIN="${OUT_ROOT}/centag-launcher${EXT}"

mkdir -p "${OUT_DIR}"
export CGO_ENABLED="${CGO_ENABLED:-1}"

if [[ "${CGO_ENABLED}" != "1" ]]; then
  echo "error: launcher requires CGO_ENABLED=1 (systray)" >&2
  exit 1
fi

echo "==> building centag-launcher → ${OUT_BIN}"
(
  cd "${LAUNCHER_DIR}"
  # Keep launcher outside the root go.work so CGO/systray never pollutes core CI.
  GOWORK=off CGO_ENABLED=1 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -trimpath -ldflags="-s -w" -o "${OUT_BIN}" .
)

cp -f "${OUT_BIN}" "${LATEST_BIN}"
echo "OK: ${OUT_BIN}"
echo "OK: ${LATEST_BIN} (current-host convenience link)"
