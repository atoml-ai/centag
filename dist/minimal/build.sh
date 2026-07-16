#!/bin/bash
# 推荐优先使用: ./start.sh dist build <name>（会注入正确 BUILD_TAGS）
set -e
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$(dirname "$0")"

TAGS="${BUILD_TAGS:-minimal,protocol_openai,protocol_anthropic,backend_openai,backend_ollama,backend_anthropic,business_router}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

echo "Building Centag Minimal (no DB; backends+protocols+router)..."
go mod tidy
go build -tags "$TAGS" -ldflags="-s -w" -o "$ROOT/bin/server/centag-minimal" .
echo "✅ centag-minimal built ($(ls -lh "$ROOT/bin/server/centag-minimal" | awk '{print $5}'))"
