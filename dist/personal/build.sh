#!/bin/bash
# 推荐优先使用: ./start.sh dist build personal（会注入正确 BUILD_TAGS）
set -e
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_use_edition personal
centag_layout_ensure_dirs personal

cd "$(dirname "$0")"

TAGS="${BUILD_TAGS:-protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

OUT="$(centag_server_bin_path personal)"
echo "Building Centag Personal (full-feature; same plugins as team) → ${OUT}"
go mod tidy
go build -tags "$TAGS" -ldflags="-s -w" -o "$OUT" .
centag_install_edition_links personal
echo "✅ centag-personal built ($(ls -lh "$OUT" | awk '{print $5}'))"
