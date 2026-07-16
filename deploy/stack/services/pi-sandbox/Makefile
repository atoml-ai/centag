.PHONY: help build start stop restart logs status test clean shell quickstart

help: ## 显示帮助信息
	@echo "Pi Sandbox 管理命令"
	@echo ""
	@echo "用法: make <command>"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

quickstart: ## 快速开始
	@./scripts/quickstart.sh

build: ## 构建 Docker 镜像
	@./scripts/manage.sh build

start: ## 启动服务
	@./scripts/manage.sh start

stop: ## 停止服务
	@./scripts/manage.sh stop

restart: ## 重启服务
	@./scripts/manage.sh restart

logs: ## 查看日志
	@./scripts/manage.sh logs

status: ## 查看状态
	@./scripts/manage.sh status

shell: ## 进入 Pi 容器
	@./scripts/manage.sh shell

test: ## 运行测试
	@./scripts/test.sh

clean: ## 清理所有数据
	@./scripts/manage.sh clean

build-client: ## 编译 Go 客户端
	@cd go-client && go build -o ../bin/pi-cli ./cmd/cli && go build -o ../bin/pi-server ./cmd/server
	@echo "编译完成: bin/pi-cli, bin/pi-server"

test-client: ## 测试 Go 客户端
	@cd go-client && go test ./...

fmt: ## 格式化 Go 代码
	@cd go-client && go fmt ./...

lint: ## 检查 Go 代码
	@cd go-client && go vet ./...

docker-up: ## 启动所有容器
	@cd docker && docker-compose up -d

docker-down: ## 停止所有容器
	@cd docker && docker-compose down

docker-logs: ## 查看容器日志
	@cd docker && docker-compose logs -f

env: ## 创建 .env 文件
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "已创建 .env 文件，请编辑填入 API 密钥"; \
	else \
		echo ".env 文件已存在"; \
	fi
