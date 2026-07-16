#!/bin/bash
set -e

echo "Starting Centag Web UI development server..."

cd "$(dirname "$0")"

# 检查是否安装了 Node.js
if ! command -v node &> /dev/null; then
    echo "Error: Node.js is not installed. Please install Node.js first."
    exit 1
fi

# 检查是否安装了 npm
if ! command -v npm &> /dev/null; then
    echo "Error: npm is not installed. Please install npm first."
    exit 1
fi

# 安装依赖（如果 node_modules 不存在）
if [ ! -d "node_modules" ]; then
    echo "Installing dependencies..."
    npm install
fi

# 启动开发服务器
echo "Starting development server at http://localhost:5173"
npm run dev
