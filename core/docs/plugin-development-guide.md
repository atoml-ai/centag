# Centag 插件开发指南

## 概述

Centag 采用核心框架 + 插件模块的架构。插件是独立的 Go module，通过接口契约与核心框架交互。

## 目录结构

```
centag/
├── core/                          # 核心框架
│   ├── go.mod                     # module centag/core
│   ├── pkg/                       # 公共 API（插件可导入）
│   │   ├── types/                 # 统一请求/响应类型
│   │   ├── protocol/              # 协议解析接口
│   │   ├── backend/               # 后端连接接口
│   │   ├── storage/               # 存储插件接口
│   │   ├── hooks/                 # 钩子系统接口
│   │   ├── plugin/                # 插件注册 API + Plugin 接口
│   │   └── entrypoint/            # 服务器启动逻辑
│   └── internal/                  # 核心内部实现（插件不可导入）
├── plugins/                       # 插件模块
│   ├── backend/                   # 后端插件
│   │   ├── openai/                # OpenAI 后端
│   │   ├── ollama/                # Ollama 后端
│   │   └── anthropic/             # Anthropic 后端
│   ├── protocol/                  # 协议插件
│   ├── storage/                   # 存储插件
│   ├── database/                  # 数据库插件
│   └── business/                  # 业务插件
└── dist/                          # 发行版
    ├── minimal/                   # 轻量（无 DB，仅 router）
    ├── personal/                  # 个人全功能（与 team 插件对齐）
    └── team/                      # 团队版（插件同 personal；部署默认外置中间件）
```

## 插件接口

### 后端插件 (Backend)

```go
// core/pkg/backend/backend.go
type Backend interface {
    Name() string
    Init(config map[string]interface{}) error
    Chat(ctx context.Context, req *UnifiedRequest) (*UnifiedResponse, error)
    ChatStream(ctx context.Context, req *UnifiedRequest) (<-chan *UnifiedChunk, error)
    HealthCheck(ctx context.Context) error
    ListModels(ctx context.Context) ([]ModelInfo, error)
}
```

### 协议插件 (Protocol)

```go
// core/pkg/protocol/protocol.go
type Protocol interface {
    Name() string
    DecodeRequest(ctx context.Context, raw []byte) (*UnifiedRequest, error)
    EncodeResponse(ctx context.Context, resp *UnifiedResponse) ([]byte, error)
    EncodeStreamResponse(ctx context.Context, chunk *UnifiedChunk) ([]byte, error)
    SupportStream() bool
}
```

### 存储插件 (Storage)

```go
// core/pkg/storage/storage.go
type StoragePlugin interface {
    KVStore
    VectorStore
}
```

## 创建后端插件

### 1. 创建模块

```bash
mkdir -p plugins/backend/mybackend
cd plugins/backend/mybackend

# 创建 go.mod
cat > go.mod << EOF
module centag/plugins/backend/mybackend

go 1.25.0

require (
    centag/core v0.0.0
)

replace centag/core => ../../../core
EOF
```

### 2. 实现插件

```go
package mybackend

import (
    "context"
    "centag/core/pkg/backend"
    "centag/core/pkg/types"
    "centag/core/pkg/plugin"
)

type Backend struct {
    config map[string]interface{}
}

func init() {
    plugin.RegisterBackend("mybackend", NewBackend)
}

func NewBackend(config map[string]interface{}) (interface{}, error) {
    b := &Backend{config: config}
    return b, nil
}

func (b *Backend) Name() string { return "mybackend" }

func (b *Backend) Init(config map[string]interface{}) error {
    b.config = config
    return nil
}

func (b *Backend) Chat(ctx context.Context, req *types.UnifiedRequest) (*types.UnifiedResponse, error) {
    // 实现聊天逻辑
    return &types.UnifiedResponse{}, nil
}

func (b *Backend) ChatStream(ctx context.Context, req *types.UnifiedRequest) (<-chan *types.UnifiedChunk, error) {
    ch := make(chan *types.UnifiedChunk)
    close(ch)
    return ch, nil
}

func (b *Backend) HealthCheck(ctx context.Context) error { return nil }
func (b *Backend) ListModels(ctx context.Context) ([]types.ModelInfo, error) {
    return []types.ModelInfo{}, nil
}
```

### 3. 验证编译

```bash
cd plugins/backend/mybackend
go build ./...
```

## 发行版组装

在 `dist/` 中创建发行版，通过 `_ import` + 对应 `-tags` 触发插件注册。

完整矩阵与 personal/team 定位见 [`docs/guide/dist-profiles.md`](../../docs/guide/dist-profiles.md)。

```go
// dist/mydist/main.go
package main

import (
    "centag/core/pkg/entrypoint"
    _ "centag/plugins/backend/mybackend"
    // 若插件 register.go 带 //go:build，构建时必须带上对应 tag
)

func main() {
    entrypoint.Run("dev", "unknown")
}
```

构建请使用：

```bash
./start.sh dist build <minimal|personal|team>
```
## 插件注册机制

插件通过 `init()` 函数调用 `plugin.Register*` 注册工厂函数：

```go
func init() {
    plugin.RegisterBackend("mybackend", NewBackend)
    plugin.RegisterProtocol("myprotocol", NewProtocol)
    plugin.RegisterStorage("mystorage", NewStorage)
}
```

框架启动时通过 `plugin.List*` 和 `plugin.Get*` 获取已注册的插件并初始化。

## 依赖规则

1. **插件只能导入 `core/pkg/`**，不能导入 `core/internal/`
2. **核心框架不导入任何插件**
3. **插件之间不互相导入**
4. **所有插件依赖通过接口解耦**
