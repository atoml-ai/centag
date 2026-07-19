#!/usr/bin/env bash
# Build centag-proxyctl (apps/proxyctl) for the *current* host OS/arch.
# Independent go.mod — keep outside root go.work (GOWORK=off).
#
# Output:
#   bin/proxyctl/<goos>-<goarch>/centag-proxyctl[.exe]
#   bin/proxyctl/centag-proxyctl[.exe]   (convenience copy for current host)
#
# Source-of-truth (client / CI without start.sh):
#   cd apps/proxyctl && GOWORK=off go build -o centag-proxyctl .
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROXYCTL_DIR="${ROOT}/apps/proxyctl"
OUT_ROOT="${ROOT}/bin/proxyctl"

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is required" >&2
  exit 1
fi

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
if [[ -n "${CENTAG_PROXYCTL_GOOS:-}" ]]; then
  GOOS="${CENTAG_PROXYCTL_GOOS}"
fi
if [[ -n "${CENTAG_PROXYCTL_GOARCH:-}" ]]; then
  GOARCH="${CENTAG_PROXYCTL_GOARCH}"
fi

case "$GOOS" in
  darwin|linux|windows) ;;
  *)
    echo "error: unsupported GOOS=$GOOS (proxyctl supports darwin|linux|windows)" >&2
    exit 1
    ;;
esac

HOST_OS="$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]' || echo unknown)"
HOST_ARCH="$(uname -m 2>/dev/null || echo unknown)"
echo "==> proxyctl host: ${HOST_OS}/${HOST_ARCH} → target ${GOOS}/${GOARCH}"

EXT=""
if [[ "$GOOS" == "windows" ]]; then
  EXT=".exe"
fi

OUT_DIR="${OUT_ROOT}/${GOOS}-${GOARCH}"
OUT_BIN="${OUT_DIR}/centag-proxyctl${EXT}"
LATEST_BIN="${OUT_ROOT}/centag-proxyctl${EXT}"

mkdir -p "${OUT_DIR}"

echo "==> building centag-proxyctl → ${OUT_BIN}"
(
  cd "${PROXYCTL_DIR}"
  # Keep proxyctl outside the root go.work so OS helpers never pollute core CI.
  GOWORK=off CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -trimpath -ldflags="-s -w" -o "${OUT_BIN}" .
)

cp -f "${OUT_BIN}" "${LATEST_BIN}"
echo "OK: ${OUT_BIN}"
echo "OK: ${LATEST_BIN} (current-host convenience link)"
echo "真源命令: cd apps/proxyctl && GOWORK=off go build -o centag-proxyctl ."
