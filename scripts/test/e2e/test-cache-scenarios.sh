#!/bin/bash

# 缓存场景测试脚本
# 测试不同情况下的缓存行为

BASE_URL="http://localhost:20060/v1/chat/completions"

echo "=========================================="
echo "Centag 缓存行为测试"
echo "=========================================="
echo ""

test_cache() {
    local test_name="$1"
    local request_data="$2"
    
    echo "----------------------------------------"
    echo "测试: $test_name"
    echo "----------------------------------------"
    
    # 第一次请求
    echo "第一次请求（应该调用后端）..."
    START=$(date +%s%3N)
    curl -s "$BASE_URL" \
        -H "Content-Type: application/json" \
        -d "$request_data" > /dev/null
    END=$(date +%s%3N)
    FIRST_TIME=$((END - START))
    echo "耗时: ${FIRST_TIME}ms"
    
    sleep 1
    
    # 第二次请求
    echo "第二次请求（检查缓存）..."
    START=$(date +%s%3N)
    RESPONSE=$(curl -s -D - "$BASE_URL" \
        -H "Content-Type: application/json" \
        -d "$request_data")
    END=$(date +%s%3N)
    SECOND_TIME=$((END - START))
    
    # 检查缓存头
    CACHE_HEADER=$(echo "$RESPONSE" | grep "X-Cache:" | tr -d '\r')
    
    if [ -z "$CACHE_HEADER" ]; then
        CACHE_STATUS="MISS（无X-Cache头）"
        RESULT="❌"
    elif echo "$CACHE_HEADER" | grep -q "HIT"; then
        CACHE_STATUS=$(echo "$CACHE_HEADER" | cut -d: -f2 | xargs)
        RESULT="✅"
    else
        CACHE_STATUS=$(echo "$CACHE_HEADER" | cut -d: -f2 | xargs)
        RESULT="❌"
    fi
    
    echo "耗时: ${SECOND_TIME}ms"
    echo "缓存状态: $CACHE_STATUS $RESULT"
    echo "速度提升: $((FIRST_TIME - SECOND_TIME))ms"
    echo ""
}

# 测试1: 完全相同的请求
test_cache "完全相同的请求（应该命中缓存）" '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "测试缓存场景1"}],
    "temperature": 0.7,
    "stream": false
}'

# 测试2: Temperature 不同
echo "清空前一个测试的缓存影响..."
sleep 2
test_cache "Temperature 不同（0.7 vs 0.8）" '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "测试缓存场景2"}],
    "temperature": 0.8,
    "stream": false
}'

# 第二次用不同的temperature
echo "第二次请求使用 temperature=0.9"
START=$(date +%s%3N)
RESPONSE=$(curl -s -D - "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "测试缓存场景2"}],
        "temperature": 0.9,
        "stream": false
    }')
END=$(date +%s%3N)
TIME=$((END - START))
CACHE_HEADER=$(echo "$RESPONSE" | grep "X-Cache:" | tr -d '\r')
echo "耗时: ${TIME}ms"
echo "缓存状态: ${CACHE_HEADER:-无X-Cache头} （预期：MISS，因为temperature不同）"
echo ""

# 测试3: 消息历史不同
test_cache "单条消息" '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Python能做什么？"}],
    "temperature": 0.7,
    "stream": false
}'

echo "第二次请求包含对话历史"
START=$(date +%s%3N)
RESPONSE=$(curl -s -D - "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "gpt-4",
        "messages": [
            {"role": "user", "content": "你好"},
            {"role": "assistant", "content": "你好！"},
            {"role": "user", "content": "Python能做什么？"}
        ],
        "temperature": 0.7,
        "stream": false
    }')
END=$(date +%s%3N)
TIME=$((END - START))
CACHE_HEADER=$(echo "$RESPONSE" | grep "X-Cache:" | tr -d '\r')
echo "耗时: ${TIME}ms"
echo "缓存状态: ${CACHE_HEADER:-无X-Cache头} （预期：MISS，因为消息历史不同）"
echo ""

# 测试4: Model 不同
test_cache "Model: gpt-4" '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "1+1=?"}],
    "temperature": 0.7,
    "stream": false
}'

echo "第二次请求使用 model=gpt-3.5-turbo"
START=$(date +%s%3N)
RESPONSE=$(curl -s -D - "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "gpt-3.5-turbo",
        "messages": [{"role": "user", "content": "1+1=?"}],
        "temperature": 0.7,
        "stream": false
    }')
END=$(date +%s%3N)
TIME=$((END - START))
CACHE_HEADER=$(echo "$RESPONSE" | grep "X-Cache:" | tr -d '\r')
echo "耗时: ${TIME}ms"
echo "缓存状态: ${CACHE_HEADER:-无X-Cache头}"
echo "注意: 由于 direct-backend 模式，两个模型实际都转为 qwen2.5:1.5b，可能命中缓存"
echo ""

# 测试5: 流式请求
echo "----------------------------------------"
echo "测试: 流式请求缓存"
echo "----------------------------------------"
echo "第一次流式请求..."
START=$(date +%s%3N)
curl -N -s "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "说一句话"}],
        "temperature": 0.7,
        "stream": true
    }' > /dev/null 2>&1
END=$(date +%s%3N)
FIRST_TIME=$((END - START))
echo "耗时: ${FIRST_TIME}ms"

sleep 2

echo "第二次相同的流式请求..."
START=$(date +%s%3N)
RESPONSE=$(curl -N -s -D - "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "说一句话"}],
        "temperature": 0.7,
        "stream": true
    }' 2>&1 | head -20)
END=$(date +%s%3N)
SECOND_TIME=$((END - START))

CACHE_HEADER=$(echo "$RESPONSE" | grep "X-Cache:" | tr -d '\r')
echo "耗时: ${SECOND_TIME}ms"
echo "缓存状态: ${CACHE_HEADER:-无X-Cache头}"
echo "响应前几行:"
echo "$RESPONSE" | head -5
echo ""

echo "=========================================="
echo "测试总结"
echo "=========================================="
echo ""
echo "缓存生效条件："
echo "  ✅ Model 相同（或转换后相同）"
echo "  ✅ Messages 完全相同"
echo "  ✅ Temperature 相同"
echo "  ✅ Max Tokens 相同"
echo ""
echo "缓存不生效情况："
echo "  ❌ Temperature 不同"
echo "  ❌ 消息历史不同（messages数组不同）"
echo "  ❌ 其他参数不同（max_tokens等）"
echo ""
echo "在 Chatbox 中使用缓存的建议："
echo "  1. 固定 Temperature 参数"
echo "  2. 使用新对话测试（清除历史）"
echo "  3. 查看响应头的 X-Cache 字段"
echo "  4. 观察响应时间（缓存应 <100ms）"
echo ""
