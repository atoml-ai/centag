# Centag 插件开发指南 V2

> 面向开发者：从零开始创建 Centag 插件，包括后端、协议、存储、流水线节点等。

---

## 目录

1. [插件概述](#1-插件概述)
2. [后端插件开发](#2-后端插件开发)
3. [流水线节点插件开发](#3-流水线节点插件开发)
4. [协议插件开发](#4-协议插件开发)
5. [存储插件开发](#5-存储插件开发)
6. [CapabilityBroker 使用](#6-capabilitybroker-使用)
7. [插件生命周期](#7-插件生命周期)
8. [插件注册表](#8-插件注册表)
9. [最佳实践](#9-最佳实践)
10. [示例插件](#10-示例插件)

---

## 1. 插件概述

### 1.1 插件类型

Centag 采用 **插件化架构**，所有可扩展功能通过插件实现：

| 插件类型 | 位置 | 接口 | 用途 |
|----------|------|------|------|
| 后端插件 | `plugins/backend/` | `Backend` | LLM 后端（OpenAI、Anthropic、Ollama） |
| 协议插件 | `plugins/protocol/` | `Protocol` | 协议转换（OpenAI 协议、Anthropic 协议） |
| 存储插件 | `plugins/storage/` | `Storage` | 存储后端（Redis、ChromaDB、PostgreSQL） |
| 流水线节点插件 | `internal/pipeline/` | `NodePlugin` | 流水线处理节点 |

### 1.2 插件架构

```
┌─────────────────────────────────────────────────────────┐
│                    Centag Core                        │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │   Backend   │  │   Protocol  │  │     Storage     │  │
│  │   Plugin    │  │   Plugin    │  │     Plugin      │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
│  ┌────────────────────────────────────────────────────┐  │
│  │              Pipeline Node Plugin                   │  │
│  └────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

---

## 2. 后端插件开发

### 2.1 目录结构

```
plugins/backend/my-backend/
├── init.go          # 注册入口
├── backend.go       # 后端实现
├── config.go        # 配置结构
├── health.go        # 健康检查
└── backend_test.go  # 测试
```

### 2.2 实现步骤

**Step 1：定义配置结构**（`config.go`）

```go
package mybackend

type Config struct {
    APIKey   string `json:"api_key" yaml:"api_key"`
    BaseURL  string `json:"base_url" yaml:"base_url"`
    Model    string `json:"model" yaml:"model"`
    Timeout  int    `json:"timeout" yaml:"timeout"`
}
```

**Step 2：实现 Backend 接口**（`backend.go`）

```go
package mybackend

import (
    "context"
    "net/http"
    "time"
    
    "centag/internal/backend"
)

type Backend struct {
    config     Config
    httpClient *http.Client
}

func NewBackend(cfg Config) (*Backend, error) {
    return &Backend{
        config: cfg,
        httpClient: &http.Client{
            Timeout: time.Duration(cfg.Timeout) * time.Second,
        },
    }, nil
}

// Name 返回后端名称
func (b *Backend) Name() string {
    return "my-backend"
}

// Forward 转发请求到后端
func (b *Backend) Forward(ctx context.Context, req *backend.Request) (*backend.Response, error) {
    // 构建 HTTP 请求
    httpReq, err := b.buildHTTPRequest(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // 发送请求
    resp, err := b.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // 解析响应
    return b.parseResponse(resp)
}

// HealthCheck 健康检查
func (b *Backend) HealthCheck(ctx context.Context) bool {
    // 实现健康检查逻辑
    healthURL := b.config.BaseURL + "/health"
    req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
    if err != nil {
        return false
    }
    
    resp, err := b.httpClient.Do(req)
    if err != nil {
        return false
    }
    defer resp.Body.Close()
    
    return resp.StatusCode == http.StatusOK
}

// buildHTTPRequest 构建 HTTP 请求
func (b *Backend) buildHTTPRequest(ctx context.Context, req *backend.Request) (*http.Request, error) {
    // 实现请求构建逻辑
    return nil, nil
}

// parseResponse 解析响应
func (b *Backend) parseResponse(resp *http.Response) (*backend.Response, error) {
    // 实现响应解析逻辑
    return nil, nil
}
```

**Step 3：实现健康检查**（`health.go`）

```go
package mybackend

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// HealthStatus 健康状态
type HealthStatus struct {
    Status     string        `json:"status"`
    ResponseTime time.Duration `json:"response_time_ms"`
    LastCheck  time.Time     `json:"last_check"`
}

// CheckHealth 执行健康检查
func (b *Backend) CheckHealth(ctx context.Context) (*HealthStatus, error) {
    start := time.Now()
    
    healthURL := b.config.BaseURL + "/v1/models"
    req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create health request: %w", err)
    }
    
    req.Header.Set("Authorization", "Bearer "+b.config.APIKey)
    
    resp, err := b.httpClient.Do(req)
    if err != nil {
        return &HealthStatus{
            Status:     "unhealthy",
            ResponseTime: time.Since(start),
            LastCheck:  time.Now(),
        }, nil
    }
    defer resp.Body.Close()
    
    status := "healthy"
    if resp.StatusCode != http.StatusOK {
        status = "unhealthy"
    }
    
    return &HealthStatus{
        Status:     status,
        ResponseTime: time.Since(start),
        LastCheck:  time.Now(),
    }, nil
}
```

**Step 4：注册插件**（`init.go`）

```go
package mybackend

import (
    "centag/internal/plugin"
)

func init() {
    plugin.RegisterBackend("my-backend", func(configMap map[string]interface{}) (interface{}, error) {
        // 解析配置
        cfg := Config{
            APIKey:  getString(configMap, "api_key", ""),
            BaseURL: getString(configMap, "base_url", "https://api.example.com"),
            Model:   getString(configMap, "model", "default-model"),
            Timeout: getInt(configMap, "timeout", 30),
        }
        return NewBackend(cfg)
    })
}

func getString(m map[string]interface{}, key, defaultVal string) string {
    if v, ok := m[key]; ok {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return defaultVal
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
    if v, ok := m[key]; ok {
        if f, ok := v.(float64); ok {
            return int(f)
        }
    }
    return defaultVal
}
```

**Step 5：在 server 中导入**（让 init() 执行）

```go
// internal/server/server.go
package server

import (
    _ "centag/plugins/backend/my-backend" // 注册插件
)
```

### 2.3 测试

```go
// backend_test.go
package mybackend

import (
    "context"
    "testing"
)

func TestBackend_Forward(t *testing.T) {
    cfg := Config{
        APIKey:  "test-key",
        BaseURL: "https://api.test.com",
        Model:   "test-model",
        Timeout: 10,
    }
    
    backend, err := NewBackend(cfg)
    if err != nil {
        t.Fatalf("failed to create backend: %v", err)
    }
    
    // 测试转发逻辑
    ctx := context.Background()
    req := &backend.Request{
        Model: "test-model",
        Messages: []backend.Message{
            {Role: "user", Content: "Hello"},
        },
    }
    
    resp, err := backend.Forward(ctx, req)
    if err != nil {
        t.Fatalf("Forward failed: %v", err)
    }
    
    if resp == nil {
        t.Fatal("expected non-nil response")
    }
}

func TestBackend_HealthCheck(t *testing.T) {
    cfg := Config{
        APIKey:  "test-key",
        BaseURL: "https://api.test.com",
        Timeout: 10,
    }
    
    backend, err := NewBackend(cfg)
    if err != nil {
        t.Fatalf("failed to create backend: %v", err)
    }
    
    ctx := context.Background()
    healthy := backend.HealthCheck(ctx)
    
    // 根据实际测试环境调整断言
    t.Logf("Health check result: %v", healthy)
}
```

---

## 3. 流水线节点插件开发

### 3.1 节点插件接口

```go
// internal/pipeline/node_plugin.go
type NodePlugin interface {
    // Descriptor 返回插件描述符
    Descriptor() NodePluginDescriptor
    
    // ValidateConfig 校验节点配置
    ValidateConfig(config NodeConfig) error
    
    // Execute 执行节点逻辑
    Execute