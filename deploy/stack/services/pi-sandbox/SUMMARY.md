# Pi Sandbox 项目总结

## 项目概述

这是一个完整的 Pi Agent 沙盒容器方案，使用 Go 语言实现客户端，提供安全隔离和资源控制。

## 已创建的文件

### 1. 配置文件

| 文件 | 说明 |
|------|------|
| `.env.example` | 环境变量配置示例 |
| `Makefile` | 简化操作的 Make 命令 |

### 2. Docker 相关

| 文件 | 说明 |
|------|------|
| `docker/Dockerfile.pi` | Pi 容器镜像定义 |
| `docker/docker-compose.yml` | 容器编排配置 |
| `docker/extensions/security.ts` | 示例安全扩展 |

### 3. Go 客户端

| 文件 | 说明 |
|------|------|
| `go-client/go.mod` | Go 模块配置 |
| `go-client/Dockerfile` | Go 客户端容器镜像 |
| `go-client/cmd/cli/main.go` | 命令行客户端 |
| `go-client/cmd/server/main.go` | HTTP API 服务器 |
| `go-client/pkg/client/pi_client.go` | RPC 客户端核心库 |
| `go-client/pkg/client/manager.go` | 多会话管理器 |
| `go-client/pkg/security/policy.go` | 安全策略实现 |
| `go-client/pkg/security/policy_test.go` | 安全策略测试 |

### 4. 脚本

| 文件 | 说明 |
|------|------|
| `scripts/manage.sh` | 主管理脚本 |
| `scripts/quickstart.sh` | 快速开始脚本 |
| `scripts/test.sh` | 集成测试脚本 |
| `scripts/run-go-tests.sh` | Go 测试脚本 |

### 5. 示例和文档

| 文件 | 说明 |
|------|------|
| `examples/test_examples.go` | 测试示例 |
| `examples/simple_client.go` | 简单客户端示例 |
| `README.md` | 项目文档 |

## 核心功能

### 1. 容器编排
- ✅ Docker Compose 一键启动
- ✅ 环境变量通过 `.env` 注入
- ✅ 资源限制（CPU、内存）
- ✅ 健康检查
- ✅ 日志管理

### 2. Go 客户端
- ✅ RPC 协议客户端
- ✅ 流式输出支持
- ✅ 多会话管理
- ✅ 安全策略控制
- ✅ 审计日志
- ✅ HTTP API 服务器
- ✅ 命令行工具

### 3. 安全控制
- ✅ 文件路径白名单/黑名单
- ✅ 命令黑名单
- ✅ 网络访问控制
- ✅ 工具调用验证

## 快速开始

### 1. 配置环境

```bash
# 复制配置文件
cp .env.example .env

# 编辑配置，填入 API 密钥
vim .env
```

### 2. 启动服务

```bash
# 方法 1: 使用 Make
make quickstart

# 方法 2: 使用脚本
./scripts/quickstart.sh

# 方法 3: 手动启动
./scripts/manage.sh build
./scripts/manage.sh start
```

### 3. 测试连接

```bash
# 查看服务状态
make status

# 测试 RPC 连接
make test

# 进入 Pi 容器
make shell
```

### 4. 使用 Go 客户端

```bash
# 编译客户端
make build-client

# 使用命令行客户端
./bin/pi-cli --help

# 启动 HTTP 服务器
./bin/pi-server
```

## 使用示例

### 1. 基本对话

```go
package main

import (
    "context"
    "github.com/pi-sandbox/go-client/pkg/client"
)

func main() {
    // 创建客户端
    piClient, _ := client.NewPiClient(".", "--no-session")
    defer piClient.Close()
    
    // 连接
    ctx := context.Background()
    piClient.Connect(ctx)
    
    // 发送提示
    piClient.Prompt("Hello, what can you help me with?")
}
```

### 2. 多会话管理

```go
// 创建管理器
config := client.DefaultManagerConfig()
manager := client.NewAgentManager(".", config)
defer manager.CloseAll()

// 创建多个会话
session1, _ := manager.CreateSession("session-1")
session2, _ := manager.CreateSession("session-2")

// 使用不同会话
session1.Client.Prompt("Help me with Go")
session2.Client.Prompt("Help me with Python")
```

### 3. HTTP API

```bash
# 创建会话
curl -X POST http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{"id": "my-session"}'

# 发送提示
curl -X POST http://localhost:8080/api/prompt \
  -H "Content-Type: application/json" \
  -d '{"session_id": "my-session", "message": "Hello!"}'
```

## 管理命令

```bash
# 构建镜像
make build

# 启动服务
make start

# 停止服务
make stop

# 查看日志
make logs

# 查看状态
make status

# 进入容器
make shell

# 运行测试
make test

# 清理数据
make clean

# 编译 Go 客户端
make build-client

# 运行 Go 测试
make test-client
```

## 安全策略

### 文件访问控制

```go
policy := &security.SecurityPolicy{
    AllowedPaths: []string{"/workspace", "/tmp"},
    BlockedPaths: []string{"/etc", "/root"},
    MaxFileSize:  10 * 1024 * 1024, // 10MB
}
```

### 命令黑名单

```go
policy.BlockedCommands = []string{
    "rm -rf /",
    "sudo",
    "chmod 777",
    "curl | sh",
}
```

## 架构图

```
┌─────────────────────────────────────────────────────────┐
│                    Go 宿主进程                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │ 安全控制层   │  │ 资源管理     │  │ 业务逻辑层      │ │
│  │ - 文件系统   │  │ - CPU/内存   │  │ - 权限策略      │ │
│  │ - 网络访问   │  │ - 超时控制   │  │ - 审计日志      │ │
│  │ - 系统调用   │  │ - 进程隔离   │  │ - 工作流编排    │ │
│  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘ │
│         │                │                   │          │
│         └────────────────┼───────────────────┘          │
│                          │                              │
│                    ┌─────▼─────┐                        │
│                    │ RPC 客户端 │                        │
│                    │ (stdin/   │                        │
│                    │  stdout)  │                        │
│                    └─────┬─────┘                        │
└──────────────────────────┼──────────────────────────────┘
                           │ JSONL 协议
┌──────────────────────────┼──────────────────────────────┐
│                    ┌─────▼─────┐                        │
│                    │ Pi 容器    │                        │
│                    │ (沙盒内)   │                        │
│                    └───────────┘                        │
└─────────────────────────────────────────────────────────┘
```

## 下一步

1. **配置 API 密钥**: 编辑 `.env` 文件，填入你的 LLM API 密钥
2. **启动服务**: 运行 `make quickstart` 快速启动
3. **测试连接**: 运行 `make test` 验证服务正常
4. **开发应用**: 使用 Go 客户端库开发你的应用

## 许可证

MIT License
