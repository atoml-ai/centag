#!/bin/bash

# 清除 macOS 产生的 ._* 文件
# 这些文件是 macOS 在非 HFS+ 文件系统上创建的，用于存储扩展属性和资源分支

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 获取项目根目录（scripts/ops → ../..）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo -e "${YELLOW}开始清理 macOS 产生的 ._* 文件...${NC}"
echo "扫描目录: $PROJECT_ROOT"
echo ""

# 查找所有 ._* 文件
MACOS_FILES=$(find "$PROJECT_ROOT" -type f -name "._*" 2>/dev/null || true)

if [ -z "$MACOS_FILES" ]; then
    echo -e "${GREEN}✓ 未找到任何 ._* 文件${NC}"
    exit 0
fi

# 统计文件数量
FILE_COUNT=$(echo "$MACOS_FILES" | wc -l)
echo -e "${YELLOW}发现 $FILE_COUNT 个 ._* 文件:${NC}"
echo "$MACOS_FILES"
echo ""

# 询问是否删除
read -p "是否删除这些文件? (y/n) " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
    # 删除文件
    echo "$MACOS_FILES" | while read -r file; do
        if [ -f "$file" ]; then
            rm -f "$file"
            echo -e "${GREEN}✓ 已删除: $file${NC}"
        fi
    done
    echo ""
    echo -e "${GREEN}✓ 清理完成！共删除 $FILE_COUNT 个文件${NC}"
else
    echo -e "${YELLOW}取消删除操作${NC}"
    exit 0
fi

# 同时清理 .DS_Store 文件（可选）
DSSTORE_FILES=$(find "$PROJECT_ROOT" -type f -name ".DS_Store" 2>/dev/null || true)
if [ -n "$DSSTORE_FILES" ]; then
    DSSTORE_COUNT=$(echo "$DSSTORE_FILES" | wc -l)
    echo ""
    echo -e "${YELLOW}发现 $DSSTORE_COUNT 个 .DS_Store 文件${NC}"
    read -p "是否也删除这些文件? (y/n) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "$DSSTORE_FILES" | while read -r file; do
            if [ -f "$file" ]; then
                rm -f "$file"
                echo -e "${GREEN}✓ 已删除: $file${NC}"
            fi
        done
        echo -e "${GREEN}✓ .DS_Store 文件清理完成！${NC}"
    fi
fi






