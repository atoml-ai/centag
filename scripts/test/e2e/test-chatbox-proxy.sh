#!/bin/bash

# Chatbox测试脚本 - 模拟Chatbox的实际请求

echo "=========================================="
echo "测试1: 非流式请求（stream=false）"
echo "=========================================="
curl -s -x http://127.0.0.1:8081 http://api.ppio.com/openai/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk_test" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": false
  }' | jq '.'

echo ""
echo "=========================================="
echo "测试2: 流式请求（stream=true）"
echo "=========================================="
curl -N -s -x http://127.0.0.1:8081 http://api.ppio.com/openai/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk_test" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": true
  }' 2>&1 | head -20

echo ""
echo "=========================================="
echo "测试3: 多模态消息（array content）"
echo "=========================================="
curl -s -x http://127.0.0.1:8081 http://api.ppio.com/openai/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk_test" \
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
  }' | jq '.choices[0].message'

echo ""
echo "=========================================="
echo "测试4: 检查响应格式"
echo "=========================================="
RESPONSE=$(curl -s -x http://127.0.0.1:8081 http://api.ppio.com/openai/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk_test" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": false
  }')

echo "响应字段检查："
echo "- id: $(echo $RESPONSE | jq -r '.id')"
echo "- object: $(echo $RESPONSE | jq -r '.object')"
echo "- model: $(echo $RESPONSE | jq -r '.model')"
echo "- choices[0].message.role: $(echo $RESPONSE | jq -r '.choices[0].message.role')"
echo "- choices[0].message.content: $(echo $RESPONSE | jq -r '.choices[0].message.content' | head -c 50)..."
echo "- choices[0].finish_reason: $(echo $RESPONSE | jq -r '.choices[0].finish_reason')"

echo ""
echo "完整响应："
echo $RESPONSE | jq '.'
