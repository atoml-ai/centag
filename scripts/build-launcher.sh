#!/usr/bin/env bash
# Build Centag desktop shell (apps/launcher) — product form: desktop.
#
#   --desktop / CENTAG_DESKTOP=1  → centag-desktop (CGO + systray)
#   (no flag)                    → centag-launcher lite (dev only, not a product form)
#
# Output:
#   ~/.centag/var/cross/launcher/<goos>-<goarch>/centag-desktop[.exe]
#   ~/.centag/bin/centag-desktop[.exe]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init

LAUNCHER_DIR="${ROOT}/apps/launcher"
CROSS_ROOT="${CENTAG_CROSS_DIR}/launcher"

DESKTOP=0
if [[ "${1:-}" == "--desktop" ]] || [[ "${CENTAG_DESKTOP:-}" == "1" ]]; then
  DESKTOP=1
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
    echo "error: unsupported GOOS=$GOOS (desktop supports darwin|linux|windows)" >&2
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

if [[ "$DESKTOP" == "1" ]]; then
  OUT_BIN="${OUT_DIR}/centag-desktop${EXT}"
  LATEST_BIN="${CENTAG_BIN_DIR}/centag-desktop${EXT}"
  echo "==> desktop shell host: ${HOST_OS}/${HOST_ARCH} → target ${GOOS}/${GOARCH} (CGO)" >&2
  if [[ "${GOOS}" != "$(go env GOOS)" || "${GOARCH}" != "$(go env GOARCH)" ]]; then
    echo "    warn: desktop/systray cross-compile often fails — prefer building on the target OS" >&2
  fi
  echo "==> building centag-desktop → ${OUT_BIN}" >&2
  (
    cd "${LAUNCHER_DIR}"
    LDFLAGS="-s -w"
    if [[ "${GOOS}" == "windows" ]]; then
      LDFLAGS="${LDFLAGS} -H windowsgui"
    fi
    GOWORK=off CGO_ENABLED=1 GOOS="${GOOS}" GOARCH="${GOARCH}" \
      go build -tags tray -trimpath -ldflags="${LDFLAGS}" -o "${OUT_BIN}" .
  )
else
  OUT_BIN="${OUT_DIR}/centag-launcher${EXT}"
  LATEST_BIN="${CENTAG_BIN_DIR}/centag-launcher${EXT}"
  echo "==> launcher lite (dev only) host: ${HOST_OS}/${HOST_ARCH} → ${GOOS}/${GOARCH}" >&2
  echo "==> building centag-launcher → ${OUT_BIN}" >&2
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
echo "${OUT_BIN}"
