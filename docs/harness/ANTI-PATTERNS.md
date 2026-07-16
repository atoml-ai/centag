# Centag 反模式文档

> 面向 AI 编码智能体：定义禁止的行为和代码模式。
> **本文档比 PATTERNS.md 更重要** — 违反这些规则会导致代码审查失败或 CI 失败。

---

## 1. 架构反模式

### 1.1 ❌ 跨层依赖

**禁止**：handler 直接调用 database

```go
// ❌ 错误：handler 直接操作数据库
func (h *Handler) GetBackend(c *gin.Context) {
    db := database.GetDB()
    row := db.QueryRow("SELECT * FROM backends WHERE id = ?", c.Param("id"))
    // ...
}

// ✅ 正确：通过 service 层
func (h *Handler) GetBackend(c *gin.Context) {
    backend, err := h.service.GetBackend(c.Request.Context(), c.Param("id"))
    // ...
}
```

**原因**：破坏分层，无法测试，无法复用

---

### 1.2 ❌ 循环依赖

**禁止**：A 包引用 B 包，B 包又引用 A 包

```go
// ❌ 错误
// internal/backend/backend.go
import "centag/internal/cache"

// internal/cache/cache.go
import "centag/internal/backend"  // 循环依赖！
```

**解决方案**：
1. 提取公共接口到第三个包
2. 使用依赖注入
3. 重新设计模块边界

---

### 1.3 ❌ internal/ 引用 plugins/

**禁止**：核心包依赖插件实现

```go
// ❌ 错误
// internal/server/server.go
import "centag/plugins/backend/openai"  // 直接引用插件

// ✅ 正确
import _ "centag/plugins/backend/openai"  // 只注册，不直接使用
```

**原因**：破坏插件架构，无法动态加载

---

### 1.4 ❌ cmd/ 写业务逻辑

**禁止**：在入口文件中写业务代码

```go
// ❌ 错误
// cmd/centag/main.go
func main() {
    // 100 行业务逻辑...
}

// ✅ 正确
// cmd/centag/main.go
func main() {
    cfg := config.Load()
    server := server.New(cfg)
    server.Run()
}

// internal/server/server.go
func (s *Server) Run() {
    // 业务逻辑在这里
}
```

---

### 1.5 ❌ 代理模式硬编码分支膨胀

**禁止**：在 `internal/proxy/` 中持续堆叠模式 if/switch 业务逻辑，绕开流水线模板与节点插件。

```go
// ❌ 错误：每新增模式都改 handler 分支
if mode == "#new-mode" {
    // 直接写大段业务流程
}

// ✅ 正确：模式映射到流水线模板
pipelineID := resolvePipelineByMode(mode)
output, err := engine.Execute(ctx, pipelineID, input)
```

**原因**：难以扩展、难测、无法复用、前后端配置难统一。

---

### 1.6 ❌ 插件越权直接访问内部依赖

**禁止**：节点插件直接持有数据库连接、全局 manager 或未受控 HTTP 客户端。

```go
// ❌ 错误：插件直接拿内部依赖
func (p *Plugin) Execute(...) {
    db := database.Get().GetDB()
    // ...
}

// ✅ 正确：通过 CapabilityBroker 申请受控能力
storage, err := broker.GetStorage(ctx, []string{"storage.read:plugin-a"})
```

**原因**：破坏隔离边界，难审计，权限无法治理。

---

### 1.7 ❌ 引用 archive/deprecated/ 目录内容

**禁止**：在生产代码或新文档中引用 `archive/deprecated/` 下的任何包、脚本或文档。

```go
// ❌ 错误
import "centag/archive/deprecated/agents/openclaw/pkg/something"

// ❌ 错误：文档指引读者去 archive/deprecated/ 找现成方案
// 见 archive/deprecated/docs/some-guide.md

// ✅ 正确：archive/deprecated/ 仅供只读考古，不引入依赖
// 如需类似能力，在 internal/ 或 plugins/ 重新实现
```

**原因**：`archive/deprecated/` 是历史归档，不再维护；引入会导致不可维护的遗留依赖、版本冲突和安全隐患。

---

## 2. Go 语言反模式

### 2.1 ❌ 忽略错误

**禁止**：不检查函数返回的错误

```go
// ❌ 错误
result, _ := doSomething()  // 忽略错误
result := doSomething()     // 不接收错误

// ✅ 正确
result, err := doSomething()
if err != nil {
    return fmt.Errorf("doSomething failed: %w", err)
}
```

**例外**：
- `fmt.Fprintf` 等写入 io.Writer（通常无法处理）
- 明确知道不会出错的情况（需注释说明）

---

### 2.2 ❌ panic 用于业务错误

**禁止**：用 panic 处理可恢复的错误

```go
// ❌ 错误
func GetBackend(name string) *Backend {
    b, ok := backends[name]
    if !ok {
        panic("backend not found")  // 不要这样做！
    }
    return b
}

// ✅ 正确
func GetBackend(name string) (*Backend, error) {
    b, ok := backends[name]
    if !ok {
        return nil, fmt.Errorf("backend %s not found", name)
    }
    return b, nil
}
```

**例外**：
- 真正的不可恢复错误（程序启动时缺少必要配置）
- `init()` 中的严重错误

---

### 2.3 ❌ 全局可变状态

**禁止**：使用全局变量存储状态

```go
// ❌ 错误
var (
    backends = make(map[string]*Backend)  // 全局可变
    mu       sync.Mutex
)

func RegisterBackend(name string, b *Backend) {
    mu.Lock()
    backends[name] = b
    mu.Unlock()
}

// ✅ 正确：使用 Manager
type BackendManager struct {
    backends map[string]*Backend
    mu       sync.RWMutex
}

func NewBackendManager() *BackendManager {
    return &BackendManager{
        backends: make(map[string]*Backend),
    }
}
```

**例外**：
- logger（但推荐依赖注入）
- 包级别的常量

---

### 2.4 ❌ 复杂的 init() 函数

**禁止**：在 init() 中做复杂初始化

```go
// ❌ 错误
func init() {
    // 连接数据库
    db, err := sql.Open("postgres", os.Getenv("DB_URL"))
    if err != nil {
        log.Fatal(err)
    }
    // 加载配置
    cfg, err := loadConfig()
    // ... 50 行代码
}

// ✅ 正确：显式初始化
func Initialize(cfg *Config) (*Service, error) {
    db, err := sql.Open(cfg.DB.Driver, cfg.DB.URL)
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }
    return &Service{db: db}, nil
}
```

**允许**：
- 插件注册（`plugin.RegisterBackend("name", factory)`）
- 简单的类型注册

---

### 2.5 ❌ 字符串拼接 SQL

**禁止**：使用字符串拼接构建 SQL

```go
// ❌ 错误：SQL 注入风险
query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", userID)
db.Query(query)

// ✅ 正确：参数化查询
db.Query("SELECT * FROM users WHERE id = $1", userID)
```

---

### 2.6 ❌ 不使用 context

**禁止**：函数不接受 context

```go
// ❌ 错误
func (s *Service) GetBackend(name string) (*Backend, error) {
    // 无法取消、超时
}

// ✅ 正确
func (s *Service) GetBackend(ctx context.Context, name string) (*Backend, error) {
    // 可以检查 ctx.Done()
}
```

---

### 2.7 ❌ 大函数

**禁止**：超过 100 行的函数

```go
// ❌ 错误：200 行的函数
func (s *Server) handleRequest(c *gin.Context) {
    // 200 行代码...
}

// ✅ 正确：拆分成小函数
func (s *Server) handleRequest(c *gin.Context) {
    req := s.parseRequest(c)
    if err := s.validateRequest(req); err != nil {
        s.handleError(c, err)
        return
    }
    resp, err := s.processRequest(c.Request.Context(), req)
    if err != nil {
        s.handleError(c, err)
        return
    }
    s.sendResponse(c, resp)
}
```

---

## 3. 并发反模式

### 3.1 ❌ goroutine 泄漏

**禁止**：启动 goroutine 但不管理生命周期

```go
// ❌ 错误
func (s *Server) Start() {
    go s.backgroundWorker()  // 没有停止机制
}

// ✅ 正确
func (s *Server) Start(ctx context.Context) {
    go s.backgroundWorker(ctx)
}

func (s *Server) backgroundWorker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case job := <-s.jobs:
            s.processJob(job)
        }
    }
}
```

---

### 3.2 ❌ 竞态条件

**禁止**：不加锁访问共享数据

```go
// ❌ 错误
type Counter struct {
    value int
}

func (c *Counter) Increment() {
    c.value++  // 竞态条件！
}

// ✅ 正确
type Counter struct {
    value int
    mu    sync.Mutex
}

func (c *Counter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.value++
}
```

---

### 3.3 ❌ 死锁

**禁止**：嵌套锁或锁顺序不一致

```go
// ❌ 错误：可能导致死锁
func (m *Manager) Transfer(from, to string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // 如果另一个 goroutine 调用 Transfer(to, from)
    // 就会死锁
}

// ✅ 正确：按固定顺序获取锁
func (m *Manager) Transfer(from, to string) {
    first, second := from, to
    if from > to {
        first, second = to, from
    }
    
    m.locks[first].Lock()
    defer m.locks[first].Unlock()
    m.locks[second].Lock()
    defer m.locks[second].Unlock()
}
```

---

## 4. 测试反模式

### 4.1 ❌ 测试依赖外部服务

**禁止**：单元测试依赖数据库、Redis 等

```go
// ❌ 错误
func TestBackendManager(t *testing.T) {
    db := sql.Open("postgres", "real-connection-string")
    manager := NewBackendManager(db)
    // 测试...
}

// ✅ 正确：使用 mock
func TestBackendManager(t *testing.T) {
    mockDB := &MockDB{}
    manager := NewBackendManager(mockDB)
    // 测试...
}
```

---

### 4.2 ❌ 测试间共享状态

**禁止**：测试之间相互影响

```go
// ❌ 错误
var globalBackend *Backend

func TestCreate(t *testing.T) {
    globalBackend = &Backend{Name: "test"}
}

func TestGet(t *testing.T) {
    // 依赖 TestCreate 先运行
    if globalBackend == nil {
        t.Fatal("backend not created")
    }
}

// ✅ 正确：每个测试独立
func TestCreate(t *testing.T) {
    backend := &Backend{Name: "test"}
    // 使用 backend
}

func TestGet(t *testing.T) {
    backend := &Backend{Name: "test"}
    // 使用 backend
}
```

---

### 4.3 ❌ 测试不清理

**禁止**：测试后留下临时文件、数据库记录

```go
// ❌ 错误
func TestWithFile(t *testing.T) {
    f, _ := os.Create("/tmp/test.txt")
    defer f.Close()
    // 测试后文件还在
}

// ✅ 正确
func TestWithFile(t *testing.T) {
    f, _ := os.CreateTemp("", "test-*.txt")
    defer os.Remove(f.Name())
    defer f.Close()
    // 测试后清理
}
```

---

## 5. 配置反模式

### 5.1 ❌ 硬编码配置

**禁止**：在代码中写死配置值

```go
// ❌ 错误
const (
    ServerPort = 8080
    Timeout    = 30 * time.Second
)

// ✅ 正确：从配置加载
type Config struct {
    Port    int           `yaml:"port" env:"SERVER_PORT" default:"8080"`
    Timeout time.Duration `yaml:"timeout" env:"SERVER_TIMEOUT" default:"30s"`
}
```

---

### 5.2 ❌ 敏感信息提交到仓库

**禁止**：密码、API Key 等提交到 Git

```go
// ❌ 错误
const APIKey = "<YOUR_API_KEY_HERE>1234567890abcdef"

// ✅ 正确：从环境变量或 secrets 目录加载
apiKey := os.Getenv("API_KEY")
```

**检查**：
```bash
# .gitignore
config/secrets/
*.env
*.key
```

### 5.3 ❌ Docker Compose / 脚本中使用弱密码默认值

**禁止**：在 `docker-compose.yml`、Shell 脚本或配置模板中为敏感变量提供不安全默认值

```yaml
# ❌ 错误：使用众所周知的弱默认值
environment:
  - POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-postgres}
  - JWT_SECRET=${JWT_SECRET:-changeme}
  - ADMIN_API_KEY=${ADMIN_API_KEY:-admin}

# ✅ 正确：强制要求设置，未设置时立即报错停止
environment:
  - POSTGRES_PASSWORD=${POSTGRES_PASSWORD:?error: POSTGRES_PASSWORD must be set}
  - JWT_SECRET=${JWT_SECRET:?error: JWT_SECRET must be set}
  - ADMIN_API_KEY=${ADMIN_API_KEY:?error: ADMIN_API_KEY must be set}
```

**禁止的弱默认值示例**：`postgres`、`admin`、`changeme`、`123456`、`password`、`secret`、`default`

**原因**：
- 环境变量未设置时，服务会使用弱默认值启动，导致未授权访问
- 攻击者熟知这些默认值，可轻易入侵
- `:?` 语法确保容器在变量缺失时立即失败，不会以不安全状态运行

**例外**：非敏感配置（如主机名、端口、布尔开关）可使用合理默认值。

---

## 6. 日志反模式

### 6.1 ❌ 日志中包含敏感信息

**禁止**：记录密码、Token、密钥

```go
// ❌ 错误
log.Info("user login", zap.String("password", req.Password))
log.Info("api call", zap.String("token", req.Token))

// ✅ 正确
log.Info("user login", zap.String("username", req.Username))
log.Info("api call", zap.String("request_id", req.RequestID))
```

---

### 6.2 ❌ 使用 fmt.Println 日志

**禁止**：用 fmt.Println 输出日志

```go
// ❌ 错误
fmt.Println("processing request")

// ✅ 正确
logger.Info("processing request")
```

**原因**：
- 无法控制级别
- 无法结构化
- 无法输出到文件
- 性能差

---

## 7. API 反模式

### 7.1 ❌ 不一致的响应格式

**禁止**：不同接口返回不同格式

```go
// ❌ 错误
// GET /api/backends 返回
[{...}, {...}]

// GET /api/backends/:id 返回
{"data": {...}}

// ✅ 正确：统一格式
// GET /api/backends 返回
{"code": 0, "data": [{...}, {...}]}

// GET /api/backends/:id 返回
{"code": 0, "data": {...}}
```

---

### 7.2 ❌ 不验证输入

**禁止**：直接使用用户输入

```go
// ❌ 错误
func (h *Handler) Create(c *gin.Context) {
    var req Request
    c.BindJSON(&req)  // 不验证
    h.service.Create(&req)
}

// ✅ 正确
func (h *Handler) Create(c *gin.Context) {
    var req Request
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    h.service.Create(&req)
}
```

---

## 8. 性能反模式

### 8.1 ❌ N+1 查询

**禁止**：循环中查询数据库

```go
// ❌ 错误
backends := getAllBackends()
for _, b := range backends {
    b.Health = getHealthStatus(b.ID)  // 每次循环都查询
}

// ✅ 正确：批量查询
backends := getAllBackends()
ids := extractIDs(backends)
healthMap := getHealthStatusBatch(ids)
for _, b := range backends {
    b.Health = healthMap[b.ID]
}
```

---

### 8.2 ❌ 不使用连接池

**禁止**：每次请求创建新连接

```go
// ❌ 错误
func doRequest() {
    client := &http.Client{}
    client.Get(url)
}

// ✅ 正确：复用连接
var client = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
    },
}
```

---

## 9. 快速检查清单

提交代码前检查：

- [ ] 没有忽略错误（除非有注释说明）
- [ ] 没有全局可变状态
- [ ] 没有循环依赖
- [ ] 没有硬编码配置
- [ ] 没有敏感信息
- [ ] 没有 SQL 拼接
- [ ] 没有 goroutine 泄漏
- [ ] 测试可以独立运行
- [ ] 日志不包含敏感信息
- [ ] API 响应格式一致

---

## 10. 违规处理

1. **CI 检查**：`make lint` 会捕获部分问题
2. **Code Review**：人工审查
3. **自动修复**：`make fmt` 可修复格式问题

---

*最后更新：2026-05-31*
*重要性：本文档优先级高于 PATTERNS.md*
