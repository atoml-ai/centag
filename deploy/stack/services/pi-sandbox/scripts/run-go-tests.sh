#!/bin/bash

# 运行 Go 客户端测试

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "🧪 运行 Go 客户端测试"
echo "===================="
echo ""

cd "$PROJECT_DIR/go-client"

echo "📦 下载依赖..."
go mod download

echo ""
echo "🔍 运行单元测试..."
go test -v ./...

echo ""
echo "✅ 测试完成"
