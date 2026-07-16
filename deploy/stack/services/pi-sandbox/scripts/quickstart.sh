#!/bin/bash

# Pi Sandbox 快速开始脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "🚀 Pi Sandbox 快速开始"
echo "===================="
echo ""

# 检查 .env 文件
if [ ! -f "$PROJECT_DIR/.env" ]; then
    echo "📝 创建配置文件..."
    cp "$PROJECT_DIR/.env.example" "$PROJECT_DIR/.env"
    echo ""
    echo "⚠️  请编辑 .env 文件，填入你的 API 密钥："
    echo "   ANTHROPIC_API_KEY=sk-ant-..."
    echo "   OPENAI_API_KEY=sk-..."
    echo ""
    echo "然后重新运行此脚本。"
    exit 1
fi

# 检查 API 密钥
source "$PROJECT_DIR/.env"

if [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$OPENAI_API_KEY" ] && [ -z "$GOOGLE_API_KEY" ]; then
    echo "❌ 错误：至少需要配置一个 API 密钥"
    echo ""
    echo "请编辑 .env 文件，填入以下至少一项："
    echo "   ANTHROPIC_API_KEY=sk-ant-..."
    echo "   OPENAI_API_KEY=sk-..."
    echo "   GOOGLE_API_KEY=..."
    exit 1
fi

echo "✅ 配置检查通过"
echo ""

# 构建镜像
echo "🔨 构建 Docker 镜像..."
cd "$PROJECT_DIR"
./scripts/manage.sh build

echo ""
echo "🚀 启动服务..."
./scripts/manage.sh start

echo ""
echo "⏳ 等待服务就绪..."
sleep 5

echo ""
echo "📊 服务状态："
./scripts/manage.sh status

echo ""
echo "✅ Pi Sandbox 启动完成！"
echo ""
echo "📚 常用命令："
echo "   ./scripts/manage.sh logs     - 查看日志"
echo "   ./scripts/manage.sh shell    - 进入 Pi 容器"
echo "   ./scripts/manage.sh test     - 测试连接"
echo "   ./scripts/manage.sh stop     - 停止服务"
echo ""
echo "🔧 Go 客户端："
echo "   cd go-client"
echo "   go build -o pi-cli ./cmd/cli"
echo "   ./pi-cli --help"
echo ""
