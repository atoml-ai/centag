#!/bin/bash

# Chatbox 直接访问测试脚本
# 测试 Centag 的标准 OpenAI API 接口

BASE_URL="http://localhost:20060/v1"
API_KEY="<YOUR_API_KEY_HERE>test"

echo "=========================================="
echo "Centag 标准 OpenAI API 测试"
echo "=========================================="
echo ""
echo "Base URL: $BASE_URL"
echo "配置用于 Chatbox 时使用此地址"
echo ""

echo "----------------------------------------"
echo "测试 1/5: 检查服务状态"
echo "----------------------------------------"
if curl -s --connect-timeout 3 http://localhost:20060/health > /dev/null 2>&1; then
    echo "✅ Centag 服务正常运行"
else
    echo "❌ Centag 服务未运行或无法连接"
    echo "   请运行: ./start.sh start"
    exit 1
fi
echo ""

echo "----------------------------------------"
echo "测试 2/5: 获取模型列表"
echo "----------------------------------------"
echo "请求: GET $BASE_URL/models"
MODELS=$(curl -s $BASE_URL/models)
if echo "$MODELS" | jq -e '.data' > /dev/null 2>&1; then
    echo "✅ 模型列表获取成功"
    echo "$MODELS" | jq -r '.data[0:5] | .[] | "  - " + .id'
else
    echo "❌ 获取模型列表失败"
    echo "$MODELS"
    exit 1
fi
echo ""

echo "----------------------------------------"
echo "测试 3/5: 聊天接口（非流式）"
echo "----------------------------------------"
echo "请求: POST $BASE_URL/chat/completions"
echo "模型: gpt-4 → qwen2.5:1.5b（自动转换）"
RESPONSE=$(curl -s $BASE_URL/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "你好，请简短回复"}],
    "stream": false
  }')

if echo "$RESPONSE" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
    echo "✅ 聊天请求成功"
    echo "   模型: $(echo "$RESPONSE" | jq -r '.model')"
    echo "   响应: $(echo "$RESPONSE" | jq -r '.choices[0].message.content')"
else
    echo "❌ 聊天请求失败"
    echo "$RESPONSE" | jq '.'
fi
echo ""

echo "----------------------------------------"
echo "测试 4/5: 多模态消息（数组格式）"
echo "----------------------------------------"
MULTIMODAL=$(curl -s $BASE_URL/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "你好"}
        ]
      }
    ],
    "stream": false
  }')

if echo "$MULTIMODAL" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
    echo "✅ 多模态消息支持正常"
    echo "   响应: $(echo "$MULTIMODAL" | jq -r '.choices[0].message.content')"
else
    echo "❌ 多模态消息处理失败"
    echo "$MULTIMODAL" | jq '.'
fi
echo ""

echo "----------------------------------------"
echo "测试 5/5: 流式响应"
echo "----------------------------------------"
echo "请求: POST $BASE_URL/chat/completions (stream=true)"
echo "预期: SSE格式输出"
echo ""
curl -N -s $BASE_URL/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "说一句话"}],
    "stream": true
  }' 2>&1 | head -8

echo ""
echo "（流式输出已截断，仅显示前几行）"
echo ""

echo "=========================================="
echo "✅ 所有测试完成"
echo "=========================================="
echo ""
echo "📋 Chatbox 配置参数："
echo "┌─────────────────────────────────────────┐"
echo "│ Provider: OpenAI API                    │"
echo "│ Base URL: $BASE_URL          │"
echo "│ API Key: $API_KEY                         │"
echo "│ Model: gpt-4（或任意模型）                │"
echo "│ Proxy: 不使用代理                        │"
echo "└─────────────────────────────────────────┘"
echo ""
echo "🎯 提示："
echo "  - 直接访问无需安装证书"
echo "  - 无论选择什么模型，都会使用 qwen2.5:1.5b"
echo "  - 支持流式和非流式响应"
echo "  - 支持多模态消息（文本部分）"
echo ""
