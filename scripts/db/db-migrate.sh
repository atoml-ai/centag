#!/bin/bash
# Proxy Claw 数据库迁移管理工具
# 用法：./scripts/db-migrate.sh [command]

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 加载环境变量
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# 默认配置
DB_MODE="${DB_MODE:-auto}"
DB_TYPE="sqlite"

# 根据配置确定数据库类型
if [ "$DB_MODE" = "postgresql" ] || [ -n "$PG_HOST" ]; then
    DB_TYPE="postgres"
fi

echo -e "${BLUE}=================================================${NC}"
echo -e "${BLUE}Proxy Claw 数据库迁移工具${NC}"
echo -e "${BLUE}数据库类型：${DB_TYPE}${NC}"
echo -e "${BLUE}=================================================${NC}"
echo ""

# 显示帮助
show_help() {
    echo "用法：$0 [command]"
    echo ""
    echo "命令:"
    echo "  migrate    执行所有未执行的迁移"
    echo "  rollback   回滚最后一个迁移"
    echo "  status     显示迁移状态"
    echo "  help       显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 migrate    # 执行迁移"
    echo "  $0 rollback   # 回滚最后一个迁移"
    echo "  $0 status     # 查看迁移状态"
    echo ""
}

# 主逻辑
case "${1:-help}" in
    migrate)
        echo -e "${YELLOW}[INFO] 执行数据库迁移...${NC}"
        echo ""
        
        # 使用 Go 程序执行迁移
        if command -v go &> /dev/null; then
            go run ./cmd/migrate migrate
        else
            echo -e "${RED}[ERROR] Go 未安装，无法执行迁移${NC}"
            exit 1
        fi
        ;;
    
    rollback)
        echo -e "${YELLOW}[INFO] 回滚最后一个迁移...${NC}"
        echo ""
        
        if command -v go &> /dev/null; then
            go run ./cmd/migrate rollback
        else
            echo -e "${RED}[ERROR] Go 未安装，无法执行回滚${NC}"
            exit 1
        fi
        ;;
    
    status)
        echo -e "${YELLOW}[INFO] 检查迁移状态...${NC}"
        echo ""
        
        if command -v go &> /dev/null; then
            go run ./cmd/migrate status
        else
            echo -e "${RED}[ERROR] Go 未安装，无法检查状态${NC}"
            exit 1
        fi
        ;;
    
    help|--help|-h)
        show_help
        ;;
    
    *)
        echo -e "${RED}[ERROR] 未知命令：$1${NC}"
        echo ""
        show_help
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}=================================================${NC}"
