#!/bin/bash
# Centag PostgreSQL 数据库初始化脚本
# 用法：./scripts/init-postgres.sh

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置（可从环境变量读取）
PG_HOST="${PG_HOST:-localhost}"
PG_PORT="${PG_PORT:-5432}"
PG_USER="${PG_USER:-postgres}"
PG_PASSWORD="${PG_PASSWORD:-}"
PG_DATABASE="${PG_DATABASE:-centag}"
PG_APP_USER="${PG_APP_USER:-llmproxy}"
PG_APP_PASSWORD="${PG_APP_PASSWORD:-$(openssl rand -base64 32)}"

echo -e "${BLUE}=================================================${NC}"
echo -e "${BLUE}Centag PostgreSQL 数据库初始化${NC}"
echo -e "${BLUE}=================================================${NC}"
echo ""

# 设置 PGPASSWORD
export PGPASSWORD="$PG_PASSWORD"

# 检查 PostgreSQL 连接
echo -e "${YELLOW}[1/5] 检查 PostgreSQL 连接...${NC}"
if ! psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -c "SELECT 1" &>/dev/null; then
    echo -e "${RED}✗ 无法连接到 PostgreSQL${NC}"
    echo "请检查："
    echo "  - PostgreSQL 服务是否运行"
    echo "  - 连接参数是否正确（host, port, user, password）"
    exit 1
fi
echo -e "${GREEN}✓ PostgreSQL 连接成功${NC}"
echo ""

# 创建数据库
echo -e "${YELLOW}[2/5] 创建数据库 ${PG_DATABASE}...${NC}"
if psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d postgres -c "SELECT 1 FROM pg_database WHERE datname = '$PG_DATABASE'" | grep -q 1; then
    echo -e "${YELLOW}⚠ 数据库 ${PG_DATABASE} 已存在，跳过创建${NC}"
else
    psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d postgres -c "CREATE DATABASE ${PG_DATABASE}"
    echo -e "${GREEN}✓ 数据库 ${PG_DATABASE} 创建成功${NC}"
fi
echo ""

# 创建应用用户
echo -e "${YELLOW}[3/5] 创建应用用户 ${PG_APP_USER}...${NC}"
if psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d postgres -c "SELECT 1 FROM pg_roles WHERE rolname = '$PG_APP_USER'" | grep -q 1; then
    echo -e "${YELLOW}⚠ 用户 ${PG_APP_USER} 已存在${NC}"
    psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d postgres -c "ALTER USER ${PG_APP_USER} WITH PASSWORD '${PG_APP_PASSWORD}'"
    echo -e "${GREEN}✓ 已更新用户密码${NC}"
else
    psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d postgres -c "CREATE USER ${PG_APP_USER} WITH PASSWORD '${PG_APP_PASSWORD}'"
    echo -e "${GREEN}✓ 用户 ${PG_APP_USER} 创建成功${NC}"
fi
echo ""

# 授予权限
echo -e "${YELLOW}[4/5] 授予数据库权限...${NC}"
psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d postgres -c "GRANT ALL PRIVILEGES ON DATABASE ${PG_DATABASE} TO ${PG_APP_USER}"
psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d ${PG_DATABASE} -c "GRANT ALL ON SCHEMA public TO ${PG_APP_USER}"
echo -e "${GREEN}✓ 权限授予完成${NC}"
echo ""

# 显示连接信息
echo -e "${YELLOW}[5/5] 数据库连接信息${NC}"
echo -e "${BLUE}=================================================${NC}"
echo -e "${GREEN}✓ 数据库初始化完成！${NC}"
echo ""
echo "请将以下配置添加到 .env 文件："
echo ""
echo -e "${BLUE}# PostgreSQL 连接配置${NC}"
echo "DB_MODE=postgresql"
echo "PG_HOST=${PG_HOST}"
echo "PG_PORT=${PG_PORT}"
echo "PG_USER=${PG_APP_USER}"
echo "PG_PASSWORD=${PG_APP_PASSWORD}"
echo "PG_DATABASE=${PG_DATABASE}"
echo "PG_SSL_MODE=disable"
echo ""
echo -e "${BLUE}=================================================${NC}"
echo ""
echo -e "${YELLOW}下一步：${NC}"
echo "1. 将上述配置添加到 .env 文件"
echo "2. 运行服务，系统将自动执行数据库迁移"
echo "3. 使用 .env 中配置的管理员账号登录"
echo ""

# 清理
unset PGPASSWORD
