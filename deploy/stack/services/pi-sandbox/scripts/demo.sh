#!/bin/bash

# Pi Sandbox 演示脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# 颜色输出
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# 清屏
clear

echo "=========================================="
echo "       Pi Sandbox 演示"
echo "=========================================="
echo ""

# 步骤 1: 检查环境
log_step "1/6 检查环境"

if ! command -v docker &> /dev/null; then
    log_warn "Docker 未安装，请先安装 Docker"
    exit 1
fi

if ! command -v go &> /dev/null; then
    log_warn "Go 未安装，请先安装 Go"
    exit 1
fi

log_info "环境检查通过"
echo ""

# 步骤 2: 创建配置
log_step "2/6 创建配置文件"

cd "$PROJECT_DIR"

if [ ! -f .env ]; then
    cp .env.example .env
    log_info "已创建 .env 文件"
    log_warn "请编辑 .env 文件填入 API 密钥后重新运行"
    echo ""
    echo "示例："
    echo "  ANTHROPIC_API_KEY=sk-ant-..."
    echo ""
    exit 1
else
    log_info "配置文件已存在"
fi
echo ""

# 步骤 3: 构建 Docker 镜像
log_step "3/6 构建 Docker 镜像"

cd docker
if docker compose version &> /dev/null; then
    docker compose build
else
    docker-compose build
fi

log_info "Docker 镜像构建完成"
echo ""

# 步骤 4: 编译 Go 客户端
log_step "4/6 编译 Go 客户端"

cd "$PROJECT_DIR/go-client"
go mod download
go build -o "$PROJECT_DIR/bin/pi-cli" ./cmd/cli
go build -o "$PROJECT_DIR/bin/pi-server" ./cmd/server

log_info "Go 客户端编译完成"
echo ""

# 步骤 5: 运行测试
log_step "5/6 运行测试"

cd "$PROJECT_DIR/go-client"
go test ./...

log_info "测试通过"
echo ""

# 步骤 6: 启动服务
log_step "6/6 启动服务"

cd "$PROJECT_DIR/docker"
if docker compose version &> /dev/null; then
    docker compose up -d
else
    docker-compose up -d
fi

log_info "服务已启动"
echo ""

# 等待服务就绪
log_info "等待服务就绪..."
sleep 5

# 显示状态
echo ""
echo "=========================================="
echo "       演示完成！"
echo "=========================================="
echo ""
echo "服务状态："
docker ps --filter "name=pi-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
echo ""
echo "常用命令："
echo "  make logs     - 查看日志"
echo "  make shell    - 进入 Pi 容器"
echo "  make test     - 测试连接"
echo "  make stop     - 停止服务"
echo ""
echo "Go 客户端："
echo "  ./bin/pi-cli --help"
echo "  ./bin/pi-server"
echo ""
echo "HTTP API："
echo "  curl http://localhost:8080/api/health"
echo ""
