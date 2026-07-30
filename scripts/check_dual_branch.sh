#!/usr/bin/env bash
set -euo pipefail

# 双项目分支校验脚本
# 本脚本必须在每次涉及 centag 与 centag-pro 双项目的编码任务前执行。
# 任一检查失败即退出并返回非零状态码，开发工具应据此停止继续编码。

CENTAG_DIR="${CENTAG_DIR:-$(pwd)}"
CENTAG_PRO_DIR="${CENTAG_PRO_DIR:-$CENTAG_DIR/../centag-pro}"
EXPECTED_BRANCH="feature/v0.3.2"

check_repo() {
    local name=$1
    local path=$2

    if [ ! -d "$path" ]; then
        echo "错误：未找到 $name 项目目录：$path" >&2
        echo "请确保 centag-pro 与 centag 位于同一级目录下。" >&2
        exit 1
    fi

    if [ ! -d "$path/.git" ]; then
        echo "错误：$name 不是 git 仓库：$path" >&2
        exit 1
    fi

    local branch
    branch=$(git -C "$path" branch --show-current)
    if [ "$branch" != "$EXPECTED_BRANCH" ]; then
        echo "错误：$name 当前分支为 '$branch'，请切换或新建 '$EXPECTED_BRANCH' 分支。" >&2
        echo "建议执行：cd $path && git checkout -b $EXPECTED_BRANCH" >&2
        exit 1
    fi

    echo "✅ $name ($path) 分支正确：$branch"
}

echo "开始校验双项目分支..."
echo "期望分支：$EXPECTED_BRANCH"
echo ""

check_repo "centag" "$CENTAG_DIR"
check_repo "centag-pro" "$CENTAG_PRO_DIR"

echo ""
echo "✅ 双项目分支校验通过，可以继续编码。"
