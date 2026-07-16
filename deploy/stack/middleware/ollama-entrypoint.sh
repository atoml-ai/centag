#!/bin/bash
# =============================================================================
# Ollama 自动加载模型 Entrypoint 脚本
# =============================================================================
# 功能：
# 1. 启动 Ollama 服务
# 2. 根据环境变量自动下载配置的模型
# 3. 保持服务运行
# =============================================================================

set -e

readonly OLLAMA_MODELS="${OLLAMA_MODELS:-/models}"
readonly OLLAMA_HOST="${OLLAMA_HOST:-0.0.0.0}"
readonly DEFAULT_MODEL="${OLLAMA_DEFAULT_MODEL:-bge-m3}"
readonly AUTOLOAD="${OLLAMA_AUTOLOAD_MODEL:-false}"

echo "=========================================="
echo "Ollama Entrypoint"
echo "=========================================="
echo "OLLAMA_MODELS: $OLLAMA_MODELS"
echo "OLLAMA_HOST: $OLLAMA_HOST"
echo "DEFAULT_MODEL: $DEFAULT_MODEL"
echo "AUTOLOAD: $AUTOLOAD"
echo "=========================================="

# 启动 Ollama 服务（后台）
echo "启动 Ollama 服务..."
ollama serve &
OLLAMA_PID=$!

# 等待 Ollama 服务就绪
echo "等待 Ollama 服务就绪..."
timeout=120
elapsed=0
while ! ollama list > /dev/null 2>&1; do
    sleep 2
    elapsed=$((elapsed + 2))
    if [ $elapsed -ge $timeout ]; then
        echo "错误: Ollama 服务启动超时"
        exit 1
    fi
done
echo "Ollama 服务已就绪 (${elapsed}s)"

# 自动下载模型
if [ "$AUTOLOAD" = "true" ]; then
    echo "检查模型: $DEFAULT_MODEL"
    
    # 检查模型是否已存在
    if ollama list 2>/dev/null | grep -q "^$DEFAULT_MODEL"; then
        echo "模型已存在: $DEFAULT_MODEL"
    else
        echo "开始下载模型: $DEFAULT_MODEL"
        echo "（这可能需要几分钟，取决于模型大小和网速）"
        
        if ollama pull "$DEFAULT_MODEL"; then
            echo "模型下载成功: $DEFAULT_MODEL"
        else
            echo "警告: 模型下载失败: $DEFAULT_MODEL"
            echo "请检查网络连接或手动执行: ollama pull $DEFAULT_MODEL"
        fi
    fi
    
    # 可选：加载其他模型（通过环境变量 OLLAMA_MODELS_TO_LOAD 指定，逗号分隔）
    if [ -n "${OLLAMA_MODELS_TO_LOAD:-}" ]; then
        IFS=',' read -ra MODELS <<< "$OLLAMA_MODELS_TO_LOAD"
        for model in "${MODELS[@]}"; do
            model=$(echo "$model" | xargs)  # 去除空格
            if [ -n "$model" ]; then
                echo "检查额外模型: $model"
                if ollama list 2>/dev/null | grep -q "^$model"; then
                    echo "模型已存在: $model"
                else
                    echo "开始下载额外模型: $model"
                    ollama pull "$model" || echo "警告: 模型下载失败: $model"
                fi
            fi
        done
    fi
else
    echo "自动加载模型已禁用 (OLLAMA_AUTOLOAD_MODEL=$AUTOLOAD)"
fi

echo "=========================================="
echo "Ollama 服务运行中..."
echo "API: http://${OLLAMA_HOST}:11434"
echo "模型: $(ollama list 2>/dev/null | grep -v 'NAME' | head -5 || echo '无')"
echo "=========================================="

# 等待 Ollama 进程
wait $OLLAMA_PID
