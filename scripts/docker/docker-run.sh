#!/bin/bash
# Proxy Claw 容器启动脚本
# 使用 docker run 命令启动容器

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# 镜像名称
IMAGE="centag:latest"
CONTAINER_NAME="centag"

# 默认配置
SERVER_PORT=${SERVER_PORT:-20060}
SERVER_HOST=${SERVER_HOST:-0.0.0.0}
LOG_LEVEL=${LOG_LEVEL:-info}

# Redis 配置
REDIS_ENABLED=${REDIS_ENABLED:-false}
REDIS_ADDR=${REDIS_ADDR:-}
REDIS_PASSWORD=${REDIS_PASSWORD:-}
REDIS_DB=${REDIS_DB:-0}

# Elasticsearch 配置
ES_ENABLED=${ES_ENABLED:-false}
ES_ADDR=${ES_ADDR:-}

# 创建命名卷（如果不存在）
print_info "创建数据卷..."
docker volume create proxyclaw_storage 2>/dev/null || true
docker volume create proxyclaw_logs 2>/dev/null || true

# 停止并删除已存在的容器
print_info "清理旧容器..."
docker stop $CONTAINER_NAME 2>/dev/null || true
docker rm $CONTAINER_NAME 2>/dev/null || true

# 构建环境变量列表
ENV_ARGS=""
if [ -n "$REDIS_ENABLED" ]; then
    ENV_ARGS="$ENV_ARGS -e REDIS_ENABLED=$REDIS_ENABLED"
fi
if [ -n "$REDIS_ADDR" ]; then
    ENV_ARGS="$ENV_ARGS -e REDIS_ADDR=$REDIS_ADDR"
fi
if [ -n "$REDIS_PASSWORD" ]; then
    ENV_ARGS="$ENV_ARGS -e REDIS_PASSWORD=$REDIS_PASSWORD"
fi
if [ -n "$REDIS_DB" ]; then
    ENV_ARGS="$ENV_ARGS -e REDIS_DB=$REDIS_DB"
fi
if [ -n "$ES_ENABLED" ]; then
    ENV_ARGS="$ENV_ARGS -e ES_ENABLED=$ES_ENABLED"
fi
if [ -n "$ES_ADDR" ]; then
    ENV_ARGS="$ENV_ARGS -e ES_ADDR=$ES_ADDR"
fi

# 启动容器
print_info "启动容器: $CONTAINER_NAME"
print_info "配置信息:"
echo "  - 服务端口: ${SERVER_PORT}"
echo "  - Redis: ${REDIS_ENABLED}"
echo "  - Elasticsearch: ${ES_ENABLED}"
echo ""

docker run -d \
  --name $CONTAINER_NAME \
  --platform linux/amd64 \
  --restart unless-stopped \
  -p ${SERVER_PORT}:20060 \
  --add-host "host.docker.internal:host-gateway" \
  -v proxyclaw_storage:/app/storage \
  -v proxyclaw_logs:/app/logs \
  -v /etc/localtime:/etc/localtime:ro \
  -v /etc/timezone:/etc/timezone:ro \
  $ENV_ARGS \
  -e SERVER_PORT=$SERVER_PORT \
  -e SERVER_HOST=$SERVER_HOST \
  -e LOG_LEVEL=$LOG_LEVEL \
  $IMAGE

print_success "容器已启动: $CONTAINER_NAME"
echo ""
echo -e "${YELLOW}常用命令:${NC}"
echo -e "  查看日志: ${BLUE}docker logs -f $CONTAINER_NAME${NC}"
echo -e "  查看状态: ${BLUE}docker ps | grep $CONTAINER_NAME${NC}"
echo -e "  停止容器: ${BLUE}docker stop $CONTAINER_NAME${NC}"
echo -e "  重启容器: ${BLUE}docker restart $CONTAINER_NAME${NC}"
echo -e "  进入容器: ${BLUE}docker exec -it $CONTAINER_NAME bash${NC}"
