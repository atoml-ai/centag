#!/usr/bin/env bash
# Cross-compile the Windows desktop product from Linux/WSL2 and assemble a
# Windows-ready directory tree.
#
#   ./scripts/build-windows-cross.sh [EDITION] [ARCH] [--skip-frontend]
#
#   EDITION        personal | minimal   (default: personal)
#   ARCH           amd64 | arm64        (default: amd64)
#   --skip-frontend do not build the Vue frontend (reuse an existing static/)
#
# Output: dist/windows-<arch>/
#   centag-desktop.exe                 # tray launcher (Win32 systray, pure Go)
#   lib/<edition>/
#     centag-<edition>.exe             # backend sidecar (Windows, CGO-free)
#     static/                          # frontend assets
#     config/initdata/                 # runtime init data
#     config/pricing/                  # pricing YAML
#     storage/ logs/ data/             # created at first run
#
# The Windows systray path (systray_windows.go) is pure Go, so no MinGW /
# cross C compiler is required — CGO_ENABLED=0 works. The official
# `make package FORM=desktop OS=windows` instead REQUIRES a Windows host, so
# this script is the way to produce the same artifacts from WSL2/Linux.
#
# After copying the tree to Windows:
#   - simplest: copy dist/windows-<arch>/lib  ->  C:\Users\<you>\.centag\lib
#               then run centag-desktop.exe from anywhere (launcher finds the
#               sidecar via ~/.centag/lib/<edition>).
#   - or: set CENTAG_BIN=C:\path\lib\<edition>\centag-<edition>.exe
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

EDITION="${1:-personal}"
ARCH="${2:-amd64}"
SKIP_FRONTEND=0
for a in "$@"; do
  case "$a" in
    --skip-frontend) SKIP_FRONTEND=1 ;;
  esac
done

case "$EDITION" in
  personal|minimal) ;;
  *) echo "error: unsupported EDITION=$EDITION (want personal|minimal)" >&2; exit 1 ;;
esac
case "$ARCH" in
  amd64|arm64) ;;
  *) echo "error: unsupported ARCH=$ARCH (want amd64|arm64)" >&2; exit 1 ;;
esac

# --- toolchain / network ---------------------------------------------------
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
unset GOROOT 2>/dev/null || true

if ! command -v go >/dev/null 2>&1; then
  echo "error: go not found on PATH (need >=1.25)" >&2
  exit 1
fi
GO_VER="$(go version | awk '{print $3}')"
echo "==> go $GO_VER  GOPROXY=$GOPROXY  GOTOOLCHAIN=$GOTOOLCHAIN"

# --- version & tags -------------------------------------------------------
VERSION="$(bash scripts/lib/centag-version.sh 2>/dev/null || echo dev)"
BUILD_TIME="$(date '+%Y-%m-%d %H:%M:%S')"
BUILD_TAGS="protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure"

OUT="${ROOT}/dist/windows-${ARCH}"
LIB_DIR="${OUT}/lib/${EDITION}"
BIN_NAME="centag-${EDITION}"
mkdir -p "${OUT}" "${LIB_DIR}/static" "${LIB_DIR}/storage" "${LIB_DIR}/logs" "${LIB_DIR}/data"

echo "==> output: ${OUT}  (edition=${EDITION}, arch=${ARCH}, version=${VERSION})"

# --- 1) Windows desktop launcher (tray) -----------------------------------
echo "==> building centag-desktop.exe (windows/${ARCH}, tray, CGO-free)..."
( cd apps/launcher && \
  GOWORK=off CGO_ENABLED=0 GOOS=windows GOARCH="${ARCH}" \
  go build -tags tray -trimpath \
    -ldflags "-s -w -H windowsgui" \
    -o "${OUT}/centag-desktop.exe" . )
echo "    OK: ${OUT}/centag-desktop.exe"

# --- 2) Windows backend sidecar -------------------------------------------
echo "==> building ${BIN_NAME}.exe (windows/${ARCH}, CGO-free)..."
CGO_ENABLED=0 GOOS=windows GOARCH="${ARCH}" \
  go build -tags "${BUILD_TAGS}" -trimpath \
    -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}'" \
    -o "${LIB_DIR}/${BIN_NAME}.exe" cmd/centag/main.go
echo "    OK: ${LIB_DIR}/${BIN_NAME}.exe"

# --- 3) frontend (platform-independent) -----------------------------------
if [[ "$SKIP_FRONTEND" == "1" ]]; then
  echo "==> skipping frontend (--skip-frontend)"
  if [[ ! -d "${LIB_DIR}/static" ]] || [[ -z "$(ls -A "${LIB_DIR}/static" 2>/dev/null)" ]]; then
    echo "    warn: ${LIB_DIR}/static is empty — run without --skip-frontend once." >&2
  fi
else
  if ! command -v npm >/dev/null 2>&1; then
    echo "error: npm not found; install Node.js or pass --skip-frontend" >&2
    exit 1
  fi
  echo "==> building frontend -> ${LIB_DIR}/static ..."
  ( cd web && \
    CENTAG_STATIC_DIR="${LIB_DIR}/static" npm install && \
    CENTAG_STATIC_DIR="${LIB_DIR}/static" npm run build )
  echo "    OK: ${LIB_DIR}/static"
fi

# --- 4) config payloads (auto-detected by launcher/sidecar) ---------------
echo "==> copying config/initdata and config/pricing ..."
if [[ -d config/initdata ]]; then
  mkdir -p "${LIB_DIR}/config"
  cp -R config/initdata "${LIB_DIR}/config/initdata"
fi
if [[ -d config/pricing ]]; then
  mkdir -p "${LIB_DIR}/config"
  cp -R config/pricing "${LIB_DIR}/config/pricing"
fi

# --- 5) quick sanity ------------------------------------------------------
echo
echo "==> build complete. Layout under ${OUT}:"
( cd "$OUT" && find . -maxdepth 3 -type f \( -name '*.exe' -o -name '*.yaml' -o -name '*.html' -o -name '*.js' \) | sort | head -40 )
echo
echo "On Windows, copy 'lib/' into C:\\Users\\<you>\\.centag\\  and run centag-desktop.exe"
echo "(or set CENTAG_BIN to the sidecar path). The tray icon appears in the Windows taskbar."
