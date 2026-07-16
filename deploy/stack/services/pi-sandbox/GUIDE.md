# Pi Sandbox 使用指南

## 🚀 快速开始（5 分钟）

### 1. 克隆并配置

```bash
cd pi-sandbox

# 复制配置文件
cp .env.example .env

# 编辑配置，填入你的 API 密钥
# 至少需要配置一个：
#   ANTHROPIC_API_KEY=sk-ant-...
#   OPENAI_API_KEY=sk-...
#   GOOGLE_API_KEY=...
vim .env
```

### 2. 一键启动

```bash
# 运行演示脚本（自动构建、编译、测试、启动）
./scripts/demo.sh

# 或者使用 Make
make quickstart
```

### 3. 验证服务

```bash
# 查看服务状态
make status

# 测试 RPC 连接
make test

# 查看日志
make logs
```

---

## 📚 详细使用

### 方式 1: 使用 Make 命令

```bash
# 查看所有可用命令
make help

# 常用命令
make build        # 构建 Docker 镜像
make start        # 启动服务
make stop         # 停止服务
make restart      # 重启服务
make logs         # 查看日志
make status       # 查看状态
make shell        # 进入 Pi 容器
make test         # 运行测试
make clean        # 清理所有数据
make build-client # 编译 Go 客户端
make test-client  # 运行 Go 测试
```

### 方式 2: 使用管理脚本

```bash
# 查看帮助
./scripts/manage.sh help

# 常用命令
./scripts/manage.sh build    # 构建镜像
./scripts/manage.sh start    # 启动服务
./scripts/manage.sh stop     # 停止服务
./scripts/manage.sh logs     # 查看日志
./scripts/manage.sh status   # 查看状态
./scripts/manage.sh shell    # 进入容器
./scripts/manage.sh test     # 测试连接
```

### 方式 3: 使用 Docker Compose

```bash
cd docker

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

---

## 🔧 Go 客户端使用

### 编译客户端

```bash
cd go-client

# 编译命令行客户端
go build -o pi-cli ./cmd/cli

# 编译 HTTP 服务器
go build -o pi-server ./cmd/server
```

### 命令行客户端

```bash
# 查看帮助
./pi-cli --help

# 基本使用
./pi-cli --dir /path/to/workspace

# 指定模型
./pi-cli --provider anthropic --model claude-sonnet-4-20250514

# 不保存会话
./pi-cli --no-session
```

### HTTP API 服务器

```bash
# 启动服务器
./pi-server

# 或指定端口
PORT=9090 ./pi-server
```

### API 调用示例

```bash
# 健康检查
curl http://localhost:8080/api/health

# 创建会话
curl -X POST http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{"id": "my-session"}'

# 发送提示
curl -X POST http://localhost:8080/api/prompt \
  -H "Content-Type: application/json" \
  -d '{"session_id": "my-session", "message": "Hello!"}'

# 查看状态
curl http://localhost:8080/api/status

# 删除会话
curl -X DELETE http://localhost:8080/api/sessions/my-session
```

---

## 🛡️ 安全配置

### 文件访问控制

编辑 `.env` 文件：

```bash
# 允许的路径（逗号分隔）
PI_ALLOWED_PATHS=/workspace,/tmp

# 阻止的路径（逗号分隔）
PI_BLOCKED_PATHS=/etc,/root,/home

# 最大文件大小（字节）
PI_MAX_FILE_SIZE=10485760
```

### 网络控制

```bash
# 是否允许网络访问
PI_ALLOW_NETWORK=true
```

### 资源限制

```bash
# 内存限制
PI_MAX_MEMORY=512m

# CPU 限制
PI_MAX_CPU=1.0

# 超时时间（秒）
PI_TIMEOUT_SECONDS=300
```

---

## 📁 目录结构

```
pi-sandbox/
├── .env.example              # 环境变量示例
├── .env                      # 环境变量配置（需创建）
├── README.md                 # 项目文档
├── SUMMARY.md                # 项目总结
├── Makefile                  # Make 命令
├── docker/
│   ├── Dockerfile.pi         # Pi 容器镜像
│   ├── docker-compose.yml    # 容器编排
│   ├── workspace/            # 工作目录
│   ├── extensions/           # 自定义扩展
│   └── skills/               # 自定义技能
├── go-client/
│   ├── go.mod                # Go 模块
│   ├── Dockerfile            # Go 客户端镜像
│   ├── cmd/
│   │   ├── cli/              # 命令行客户端
│   │   └── server/           # HTTP 服务器
│   └── pkg/
│       ├── client/           # RPC 客户端库
│       └── security/         # 安全控制库
├── examples/
│   ├── test_examples.go      # 测试示例
│   └── simple_client.go      # 简单客户端示例
├── scripts/
│   ├── manage.sh             # 管理脚本
│   ├── quickstart.sh         # 快速开始
│   ├── demo.sh               # 演示脚本
│   ├── test.sh               # 集成测试
│   └── run-go-tests.sh       # Go 测试
└── bin/                      # 编译产物
    ├── pi-cli                # 命令行客户端
    └── pi-server             # HTTP 服务器
```

---

## 🧪 测试

### 运行所有测试

```bash
# 运行集成测试
./scripts/test.sh

# 运行 Go 测试
./scripts/run-go-tests.sh

# 或使用 Make
make test
make test-client
```

### 测试 RPC 连接

```bash
# 进入容器
make shell

# 在容器内测试
echo '{"type": "prompt", "message": "Hello"}' | pi --mode rpc --no-session
```

---

## 🔍 故障排查

### 查看日志

```bash
# 查看所有服务日志
make logs

# 查看 Pi 容器日志
docker logs pi-sandbox

# 实时跟踪日志
docker logs -f pi-sandbox
```

### 常见问题

**Q: 容器启动失败**
```bash
# 检查 Docker 状态
docker ps -a

# 查看容器日志
docker logs pi-sandbox

# 重新构建
make clean
make build
make start
```

**Q: API 密钥错误**
```bash
# 检查配置
cat .env | grep API_KEY

# 确保密钥有效
# 重新编辑 .env 文件
vim .env
```

**Q: Go 编译失败**
```bash
# 更新依赖
cd go-client
go mod tidy

# 重新编译
go build -o pi-cli ./cmd/cli
```

---

## 📖 更多资源

- [Pi 官方文档](https://pi.dev)
- [Pi GitHub](https://github.com/badlogic/pi-mono)
- [Docker 文档](https://docs.docker.com)
- [Go 文档](https://golang.org/doc)

---

## 📝 许可证

MIT License
