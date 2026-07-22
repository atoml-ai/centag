#!/bin/bash

# SDK 兼容性测试运行脚本

set -e

# 默认配置
CENTAG_BASE_URL="${CENTAG_BASE_URL:-http://localhost:20060}"
CENTAG_API_KEY="${CENTAG_API_KEY:-test-key}"

echo "=========================================="
echo "SDK 兼容性测试"
echo "=========================================="
echo "Centag 地址: $CENTAG_BASE_URL"
echo ""

# 检查依赖
echo "检查 Python 依赖..."
pip install -r requirements.txt -q

# 运行 OpenAI SDK 测试
echo ""
echo "运行 OpenAI SDK 测试..."
echo "------------------------------------------"
CENTAG_BASE_URL=$CENTAG_BASE_URL CENTAG_API_KEY=$CENTAG_API_KEY \
    python -m pytest test_openai_sdk.py -v --tb=short 2>&1 || true

# 运行 Anthropic SDK 测试
echo ""
echo "运行 Anthropic SDK 测试..."
echo "------------------------------------------"
CENTAG_BASE_URL=$CENTAG_BASE_URL CENTAG_API_KEY=$CENTAG_API_KEY \
    python -m pytest test_anthropic_sdk.py -v --tb=short 2>&1 || true

echo ""
echo "=========================================="
echo "测试完成"
echo "=========================================="
