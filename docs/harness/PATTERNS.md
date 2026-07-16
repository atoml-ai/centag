# Centag 设计模式文档

> 面向 AI 编码智能体：定义项目中使用的设计模式和代码模板。
> 遵循这些模式可确保代码一致性，降低认知负担。

---

## 1. 核心设计模式

### 1.1 插件模式（Plugin Pattern）

**用途**：实现可插拔的后端、存储、协议支持

**实现**：
```go
// internal/plugin/manager.go
package plugin

type Manager struct {
    backends map[string]BackendFactory
    storages map[string]StorageFactory
}

func (m *Manager) RegisterBackend(name string, factory BackendFactory) {
    m.backends[name] = factory
}

func (m *Manager) CreateBackend(name string, cfg Config) (Backend, error) {
    factory, ok := m.backends[name]
    if !ok {
        return nil, fmt.Errorf("backend %s not found", name)
    }
    return factory(cfg)
}
```

**使用**：
```go
// plugins/backend/openai/init.go
package openai

import "centag/internal/plugin"

func init() {
    plugin.RegisterBackend("openai", NewOpenAIBackend)
}
```

**注册位置**：
```go
// internal/server/server.go
import (
    _ "centag/plugins/backend/openai"
    _ "centag/plugins/backend/anthropic"
    _ "centag/plugins/storage/redis"
)
```

---

### 1.2 工厂模式（Factory Pattern）

**用途**：根据配置创建不同类型的对象

**实现**：
```go
// internal/backend/factory.go
type BackendFactory func(Config) (Backend, error)

// internal/storage/factory.go
type StorageFactory func(Config) (Storage, error)

// 使用
func CreateBackend(name string, cfg Config) (Backend, error) {
    switch name {
    case "openai":
        return NewOpenAIBackend(cfg)
    case "anthropic":
        return NewAnthropicBackend(cfg)
    default:
        return nil, ErrBackendNotFound
    }
}
```

---

### 1.3 策略模式（Strategy Pattern）

**用途**：支持多种调度策略、缓存策略

**实现**：
```go
// internal/strategy/strategy.go
type Strategy interface {
    Select(backends []*backend.Backend) (*backend.Backend, error)
    Name() string
}

// 轮询策略
type RoundRobinStrategy struct {
    index int
}

func (s *RoundRobinStrategy) Select(backends []*backend.Backend) (*backend.Backend, error) {
    if len(backends) == 0 {
        return nil, errors.New("no backends available")
    }
    selected := backends[s.index%len(backends)]
    s.index++
    return selected, nil
}

// 权重策略
type WeightedStrategy struct {
    // ...
}

// 调度器
func (s *Scheduler) Select() (*backend.Backend, error) {
    return s.strategy.Select(s.backends)
}
```

---

### 1.4 管道模式（Pipeline Pattern）

**用途**：请求处理的链式处理

**实现**：
```go
// internal/pipeline/pipeline.go
type Processor interface {
    Process(ctx context.Context, req *Request) (*Response, error)
}

type Pipeline struct {
    processors []Processor
}

func (p *Pipeline) Add(processor Processor) {
    p.processors = append(p.processors, processor)
}

func (p *Pipeline) Execute(ctx context.Context, req *Request) (*Response, error) {
    for _, proc := range p.processors {
        resp, err := proc.Process(ctx, req)
        if err != nil {
            return nil, err
        }
        if resp != nil {
            return resp, nil
        }
    }
    return nil, errors.New("no response from pipeline")
}
```

---

### 1.5 代理模式（Proxy Pattern）

**用途**：请求转发、中间人处理

**实现**：
```go
// internal/proxy/handler.go
type Handler struct {
    client *http.Client
}

func (h *Handler) Forward(ctx context.Context, req *Request) (*Response, error) {
    httpReq, err := h.buildHTTPRequest(req)
    if err != nil {
        return nil, err
    }
    
    resp, err := h.client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    
    return h.parseResponse(resp)
}
```

---

### 1.6 装饰器模式（Decorator Pattern）

**用途**：中间件、日志、认证

**实现**：
```go
// internal/middleware/middleware.go
type Middleware func(http.Handler) http.Handler

func Logger(logger *zap.Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            next.ServeHTTP(w, r)
            logger.Info("request processed",
                zap.String("path", r.URL.Path),
                zap.Duration("duration", time.Since(start)))
        })
    }
}

func Auth(authService *auth.Service) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !authService.Validate(r) {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// 链式使用
handler := Logger(logger)(Auth(authService)(mainHandler))
```

---

### 1.7 单例模式（Singleton Pattern）

**用途**：全局配置、连接池、日志

**实现**：
```go
// internal/logger/logger.go
type Logger struct {
    *zap.Logger
}

var (
    instance *Logger
    once     sync.Once
)

func GetLogger() *Logger {
    once.Do(func() {
        zapLogger, _ := zap.NewProduction()
        instance = &Logger{Logger: zapLogger}
    })
    return instance
}
```

**替代方案**（推荐）：依赖注入
```go
// 在 server.New() 中创建并传递
logger := logger.New(cfg.Log)
server := server.New(cfg, logger)
```

---

### 1.8 观察者模式（Observer Pattern）

**用途**：健康检查状态变更、配置热更新

**实现**：
```go
// internal/backend/observer.go
type Observer interface {
    OnBackendStatusChange(backend *Backend, status Status)
}

type Subject struct {
    observers []Observer
}

func (s *Subject) Register(obs Observer) {
    s.observers = append(s.observers, obs)
}

func (s *Subject) Notify(backend *Backend, status Status) {
    for _, obs := range s.observers {
        obs.OnBackendStatusChange(backend, status)
    }
}
```

---

### 1.9 流水线节点插件模式（Node Plugin Pattern）

**用途**：把代理模式能力沉淀为标准节点插件，减少硬编码逻辑分支。

**实现原则**：

```go
// internal/pipeline/plugin_contract.go
type NodePlugin interface {
    Descriptor() NodePluginDescriptor
    ValidateConfig(config NodeConfig) error
    Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error)
}
```

```go
// 推荐：模式 -> 模板 -> 节点实现
pipeline := "audit-mode"
node.Implementation = "builtin.reviewer" // 或 https://plugin.example.com
```

**实践规则**：
- 新能力优先作为节点插件实现，而不是在 handler/proxy 中追加分支。
- 插件访问外部能力（LLM、memory、secrets、HTTP）必须经 `CapabilityBroker`。
- 保持 descriptor 可发现（`implementation`、`kind`、`version`、`permissions`）。

---

## 2. Go 惯用法

### 2.1 选项模式（Functional Options）

**用途**：配置对象的灵活构建

**实现**：
```go
// internal/config/options.go
type Config struct {
    Port    int
    Timeout time.Duration
    Logger  *zap.Logger
}

type Option func(*Config)

func WithPort(port int) Option {
    return func(c *Config) {
        c.Port = port
    }
}

func WithTimeout(timeout time.Duration) Option {
    return func(c *Config) {
        c.Timeout = timeout
    }
}

func NewConfig(opts ...Option) *Config {
    cfg := &Config{
        Port:    DefaultPort,
        Timeout: DefaultTimeout,
    }
    for _, opt := range opts {
        opt(cfg)
    }
    return cfg
}

// 使用
config := NewConfig(
    WithPort(8080),
    WithTimeout(60*time.Second),
)
```

---

### 2.2 接口隔离

**用途**：定义最小可用接口

**实现**：
```go
// 好：小接口
package storage

type Reader interface {
    Get(key string) ([]byte, error)
}

type Writer interface {
    Set(key string, value []byte) error
}

type Storage interface {
    Reader
    Writer
    Delete(key string) error
}

// 使用者只需依赖需要的部分
func UseCache(r storage.Reader) {
    // 只需要读能力
}
```

---

### 2.3 错误处理模式

**实现**：
```go
// 自定义错误类型
type NotFoundError struct {
    Resource string
    ID       string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s %s not found", e.Resource, e.ID)
}

// 错误检查
func IsNotFound(err error) bool {
    var nf *NotFoundError
    return errors.As(err, &nf)
}

// 使用
if err != nil {
    if IsNotFound(err) {
        // 特殊处理
    }
    return err
}
```

---

### 2.4 上下文使用

**实现**：
```go
// 传递上下文
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// 超时控制
if err := doWork(ctx); err != nil {
    return fmt.Errorf("work failed: %w", err)
}

// 存储值（谨慎使用）
type contextKey string

func WithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, contextKey("request_id"), id)
}

func GetRequestID(ctx context.Context) string {
    id, _ := ctx.Value(contextKey("request_id")).(string)
    return id
}
```

---

## 3. 并发模式

### 3.1 Worker Pool

**用途**：限制并发数、批量处理

**实现**：
```go
// 工作池
type Pool struct {
    workers int
    jobs    chan Job
    results chan Result
}

func (p *Pool) Run() {
    var wg sync.WaitGroup
    for i := 0; i < p.workers; i++ {
        wg.Add(1)
        go p.worker(&wg)
    }
    wg.Wait()
    close(p.results)
}

func (p *Pool) worker(wg *sync.WaitGroup) {
    defer wg.Done()
    for job := range p.jobs {
        result := job.Process()
        p.results <- result
    }
}
```

---

### 3.2 扇出/扇入

**用途**：并行处理多个后端

**实现**：
```go
// 并行健康检查
func (m *Manager) CheckAll(ctx context.Context) []Result {
    var wg sync.WaitGroup
    results := make(chan Result, len(m.backends))
    
    for _, b := range m.backends {
        wg.Add(1)
        go func(backend *Backend) {
            defer wg.Done()
            healthy := backend.HealthCheck(ctx)
            results <- Result{Backend: backend, Healthy: healthy}
        }(b)
    }
    
    go func() {
        wg.Wait()
        close(results)
    }()
    
    var all []Result
    for r := range results {
        all = append(all, r)
    }
    return all
}
```

---

### 3.3 超时控制

**实现**：
```go
// 带超时的操作
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

select {
case result := <-doAsyncWork(ctx):
    return result, nil
case <-ctx.Done():
    return nil, ctx.Err()
}
```

---

## 4. 测试模式

### 4.1 模拟/桩（Mock/Stub）

**实现**：
```go
// 接口
type Backend interface {
    Forward(ctx context.Context, req *Request) (*Response, error)
}

// 模拟实现
type MockBackend struct {
    MockResponse *Response
    MockError    error
}

func (m *MockBackend) Forward(ctx context.Context, req *Request) (*Response, error) {
    return m.MockResponse, m.MockError
}

// 测试使用
func TestHandler(t *testing.T) {
    mock := &MockBackend{
        MockResponse: &Response{Body: []byte("ok")},
    }
    handler := NewHandler(mock)
    // 测试...
}
```

---

### 4.2 测试服务器

**实现**：
```go
func setupTestServer() (*Server, func()) {
    cfg := &config.Config{Port: 0}
    server := New(cfg)
    go server.Start()
    
    cleanup := func() {
        server.Shutdown(context.Background())
    }
    return server, cleanup
}

func TestAPI(t *testing.T) {
    server, cleanup := setupTestServer()
    defer cleanup()
    
    resp, err := http.Get("http://" + server.Addr() + "/api/test")
    // 断言...
}
```

---

## 5. 代码模板

### 5.1 新 Handler 模板

```go
package handler

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

// XxxHandler 处理 xxx 请求
type XxxHandler struct {
    logger  *zap.Logger
    service *Service // 依赖
}

// NewXxxHandler 创建 XxxHandler
func NewXxxHandler(logger *zap.Logger, service *Service) *XxxHandler {
    return &XxxHandler{
        logger:  logger,
        service: service,
    }
}

// RegisterRoutes 注册路由
func (h *XxxHandler) RegisterRoutes(router *gin.RouterGroup) {
    group := router.Group("/xxx")
    {
        group.GET("", h.List)
        group.GET("/:id", h.Get)
        group.POST("", h.Create)
        group.PUT("/:id", h.Update)
        group.DELETE("/:id", h.Delete)
    }
}

// List 获取列表
func (h *XxxHandler) List(c *gin.Context) {
    items, err := h.service.List(c.Request.Context())
    if err != nil {
        h.logger.Error("list failed", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, items)
}

// Get 获取详情
func (h *XxxHandler) Get(c *gin.Context) {
    id := c.Param("id")
    item, err := h.service.Get(c.Request.Context(), id)
    if err != nil {
        h.logger.Error("get failed", zap.Error(err), zap.String("id", id))
        c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, item)
}

// Create 创建
func (h *XxxHandler) Create(c *gin.Context) {
    var req CreateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    item, err := h.service.Create(c.Request.Context(), &req)
    if err != nil {
        h.logger.Error("create failed", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusCreated, item)
}

// Update 更新
func (h *XxxHandler) Update(c *gin.Context) {
    id := c.Param("id")
    var req UpdateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    item, err := h.service.Update(c.Request.Context(), id, &req)
    if err != nil {
        h.logger.Error("update failed", zap.Error(err), zap.String("id", id))
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, item)
}

// Delete 删除
func (h *XxxHandler) Delete(c *gin.Context) {
    id := c.Param("id")
    if err := h.service.Delete(c.Request.Context(), id); err != nil {
        h.logger.Error("delete failed", zap.Error(err), zap.String("id", id))
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.Status(http.StatusNoContent)
}
```

---

### 5.2 新 Service 模板

```go
package service

import (
    "context"
    "fmt"
)

// Service 业务逻辑
type Service struct {
    repo   Repository
    cache  Cache
    logger *zap.Logger
}

// NewService 创建 Service
func NewService(repo Repository, cache Cache, logger *zap.Logger) *Service {
    return &Service{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}

// Get 获取详情
func (s *Service) Get(ctx context.Context, id string) (*Entity, error) {
    // 先查缓存
    if item, err := s.cache.Get(ctx, id); err == nil {
        return item, nil
    }
    
    // 再查数据库
    item, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get entity: %w", err)
    }
    
    // 写入缓存
    _ = s.cache.Set(ctx, id, item)
    
    return item, nil
}
```

---

### 5.3 新 Plugin 模板

```go
// plugins/backend/xxx/init.go
package xxx

import (
    "centag/internal/plugin"
)

func init() {
    plugin.RegisterBackend("xxx", NewBackend)
}

// plugins/backend/xxx/backend.go
package xxx

import (
    "context"
)

// Backend xxx 后端实现
type Backend struct {
    config Config
    client *http.Client
}

// NewBackend 创建后端
func NewBackend(cfg Config) (*Backend, error) {
    return &Backend{
        config: cfg,
        client: &http.Client{Timeout: cfg.Timeout},
    }, nil
}

// Forward 转发请求
func (b *Backend) Forward(ctx context.Context, req *Request) (*Response, error) {
    // 实现转发逻辑
    return nil, nil
}

// Name 返回后端名称
func (b *Backend) Name() string {
    return "xxx"
}

// HealthCheck 健康检查
func (b *Backend) HealthCheck(ctx context.Context) bool {
    // 实现健康检查
    return true
}
```

---

## 6. 反模式（DO NOT）

见 ANTI-PATTERNS.md 获取完整列表。

快速参考：
- ❌ 全局变量（除了 logger）
- ❌ 包级别的 init() 做复杂初始化
- ❌ 忽略错误
- ❌ 循环依赖
- ❌ 大接口（超过 3-4 个方法）

---

*最后更新：2026-05-03*
