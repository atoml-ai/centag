#!/usr/bin/env bash
# Build centag-wrap (apps/wrap) for the *current* host OS/arch.
# Independent go.mod — keep outside root go.work (GOWORK=off).
#
# Output (install-compatible):
#   ~/.centag/bin/centag-wrap[.exe]                         # host PATH binary
#   ~/.centag/var/cross/wrap/<goos>-<goarch>/centag-wrap    # arch-specific copy
#
# Override root: CENTAG_INSTALL_ROOT
# Source-of-truth (client / CI without start.sh):
#   cd apps/wrap && GOWORK=off go build -o centag-wrap .
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init

WRAP_DIR="${ROOT}/apps/wrap"
CROSS_ROOT="${CENTAG_CROSS_DIR}/wrap"

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is required" >&2
  exit 1
fi

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
if [[ -n "${CENTAG_WRAP_GOOS:-}" ]]; then
  GOOS="${CENTAG_WRAP_GOOS}"
fi
if [[ -n "${CENTAG_WRAP_GOARCH:-}" ]]; then
  GOARCH="${CENTAG_WRAP_GOARCH}"
fi

case "$GOOS" in
  darwin|linux|windows) ;;
  *)
    echo "error: unsupported GOOS=$GOOS (wrap supports darwin|linux|windows)" >&2
    exit 1
    ;;
esac

HOST_OS="$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]' || echo unknown)"
HOST_ARCH="$(uname -m 2>/dev/null || echo unknown)"
echo "==> wrap host: ${HOST_OS}/${HOST_ARCH} → target ${GOOS}/${GOARCH}"

EXT=""
if [[ "$GOOS" == "windows" ]]; then
  EXT=".exe"
fi

OUT_DIR="${CROSS_ROOT}/${GOOS}-${GOARCH}"
OUT_BIN="${OUT_DIR}/centag-wrap${EXT}"

mkdir -p "${OUT_DIR}" "${CENTAG_BIN_DIR}"

echo "==> building centag-wrap → ${OUT_BIN}"
(
  cd "${WRAP_DIR}"
  # Keep wrap outside the root go.work so OS helpers never pollute core CI.
  GOWORK=off CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -trimpath -ldflags="-s -w" -o "${OUT_BIN}" .
)

# Host convenience / install layout: only promote when targeting current host.
HOST_GOOS="$(go env GOOS)"
HOST_GOARCH="$(go env GOARCH)"
if [[ "$GOOS" == "$HOST_GOOS" && "$GOARCH" == "$HOST_GOARCH" ]]; then
  centag_install_wrap_bin "${OUT_BIN}" "${EXT}"
  echo "OK: ${CENTAG_BIN_DIR}/centag-wrap${EXT} (install bin/)"
fi

echo "OK: ${OUT_BIN}"
echo "真源命令: cd apps/wrap && GOWORK=off go build -o centag-wrap ."
