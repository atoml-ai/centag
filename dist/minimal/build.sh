#!/bin/bash
# 推荐优先使用: ./start.sh dist build minimal（会注入正确 BUILD_TAGS）
set -e
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_use_edition minimal
centag_layout_ensure_dirs minimal

cd "$(dirname "$0")"

TAGS="${BUILD_TAGS:-minimal,protocol_openai,protocol_anthropic,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

OUT="$(centag_server_bin_path minimal)"
echo "Building Centag Minimal → ${OUT}"
go mod tidy
go build -tags "$TAGS" -ldflags="-s -w" -o "$OUT" .
centag_install_edition_links minimal
echo "✅ centag-minimal built ($(ls -lh "$OUT" | awk '{print $5}'))"
