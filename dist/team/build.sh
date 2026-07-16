#!/bin/bash
# 推荐优先使用: ./start.sh dist build team（会注入正确 BUILD_TAGS）
set -e
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$(dirname "$0")"

TAGS="${BUILD_TAGS:-protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

echo "Building Centag Team (full-feature; same plugins as gateway)..."
go mod tidy
go build -tags "$TAGS" -ldflags="-s -w" -o "$ROOT/bin/server/centag-team" .
echo "✅ centag-team built ($(ls -lh "$ROOT/bin/server/centag-team" | awk '{print $5}'))"
