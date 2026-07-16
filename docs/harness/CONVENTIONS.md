# Centag 编码规范文档

> 面向 AI 编码智能体：定义代码风格、命名规则、文件组织方式。
> 遵循本文档可确保代码一致性，降低维护成本。

---

## 1. Go 代码规范

### 1.1 格式化

- **强制**：使用 `gofmt` 或 `goimports` 格式化
- **缩进**：Tab（Go 标准）
- **行宽**：建议 120 字符，硬限制 150
- **空行**：函数间一个空行，逻辑块间一个空行

```bash
# 格式化
make fmt
# 或
go fmt ./...
```

### 1.2 命名规范

#### 包名（Package）
- 全小写，不使用下划线或混合大小写
- 简短且有意义：`cache`、`backend`、`config`
- 避免：`utils`、`common`、`helpers`（太泛化）

#### 文件名
- 全小写，用下划线分隔：`db_loader.go`、`proxy_mode.go`
- 测试文件：`*_test.go`
- 平台特定：`*_linux.go`、`*_windows.go`

#### 类型名
- 驼峰命名（CamelCase）：`BackendManager`、`CacheConfig`
- 接口名不加 `I` 前缀：用 `Reader` 而非 `IReader`
- 接口方法返回错误时，接口名可加 `er`：`StorageProvider`

#### 变量/函数
- 驼峰命名：`backendName`、`GetConfig()`
- 首字母大写 = 导出：`func NewServer()` 
- 首字母小写 = 内部：`func parseConfig()`
- 常量：全大写 + 下划线：`MAX_RETRIES`、`DefaultTimeout`

#### 常用命名约定
```go
// 好
type BackendManager struct {}
func NewBackendManager() *BackendManager {}
func (m *BackendManager) GetBackend(name string) (*Backend, error) {}

// 避免
type backendManager struct {}  // 未导出，外部无法使用
func newBackendManager() *BackendManager {}  // 未导出
```

### 1.3 错误处理

#### 错误定义
```go
// 在 internal/errors/ 或包级别定义
var (
    ErrBackendNotFound = errors.New("backend not found")
    ErrInvalidConfig   = errors.New("invalid configuration")
)
```

#### 错误包装
```go
// 使用 fmt.Errorf 和 %w 包装
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}
```

#### 错误检查
```go
// 标准模式
result, err := doSomething()
if err != nil {
    return fmt.Errorf("context: %w", err)
}
// 使用 result
```

### 1.4 注释规范

#### 包注释
```go
// Package cache provides caching functionality for the proxy service.
// It supports both exact match and semantic similarity caching.
package cache
```

#### 导出类型/函数
```go
// BackendManager manages backend service connections and health checks.
type BackendManager struct {
    // ...
}

// NewBackendManager creates a new BackendManager with the given config.
func NewBackendManager(cfg *config.Config) (*BackendManager, error) {
    // ...
}
```

#### 复杂逻辑
```go
// 调度算法：根据权重和健康状态选择后端
// 1. 过滤不健康的后端
// 2. 按权重计算选择概率
// 3. 随机选择一个后端
func (s *Scheduler) Select() (*Backend, error) {
    // ...
}
```

---

## 2. 项目文件组织

### 2.1 目录结构

```
centag/
├── cmd/                    # 入口程序
│   ├── centag/         # 主服务入口
│   │   └── main.go
│   └── migrate/           # 迁移工具
│       └── main.go
├── internal/              # 内部包（不可外部引用）
│   ├── backend/           # 后端管理
│   ├── cache/             # 缓存
│   ├── config/            # 配置
│   ├── handler/           # HTTP 处理器
│   ├── server/            # 服务器
│   └── ...
├── plugins/               # 可插拔实现
│   ├── backend/           # 后端实现
│   ├── protocol/          # 协议实现
│   └── storage/           # 存储实现
├── web/                 # Vue 前端
├── static/                # 静态资源（构建产物）
├── docs/                  # 文档
├── scripts/               # 脚本
├── deploy/docker/                # Docker 配置
└── config/initdata/              # 初始化数据
```

### 2.2 包内文件组织

每个包建议的文件组织：

```
package_name/
├── README.md              # 包说明（可选）
├── types.go               # 类型定义
├── errors.go              # 错误定义
├── interface.go           # 接口定义
├── manager.go             # 主要实现
├── manager_test.go        # 测试
└── helpers.go             # 辅助函数
```

### 2.3 文件大小

- 单文件建议不超过 500 行
- 超过 300 行考虑拆分
- 测试文件可适当放宽

---

## 3. 测试规范

### 3.1 测试文件位置

- 测试文件与被测文件同目录
- 命名：`*_test.go`
- 包名：`package_name_test`（黑盒）或 `package_name`（白盒）

### 3.2 测试函数命名

```go
// 单元测试
func TestFunctionName(t *testing.T) { ... }
func TestFunctionName_Scenario(t *testing.T) { ... }

// 示例
func TestBackendManager_GetBackend(t *testing.T) { ... }
func TestBackendManager_GetBackend_NotFound(t *testing.T) { ... }
```

### 3.3 表驱动测试

```go
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Config
        wantErr bool
    }{
        {
            name:  "valid config",
            input: `{"port": 8080}`,
            want:  &Config{Port: 8080},
        },
        {
            name:    "invalid json",
            input:   `{invalid}`,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseConfig(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParseConfig() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### 3.4 运行测试

```bash
# 运行所有测试
make test

# 运行特定包
go test ./internal/cache/...

# 运行特定测试
go test -run TestBackendManager ./internal/backend/

# 带覆盖率
go test -cover ./...
```

---

## 4. API 规范

### 4.1 RESTful 命名

```
GET    /api/v1/backends          # 列表
GET    /api/v1/backends/:id      # 详情
POST   /api/v1/backends          # 创建
PUT    /api/v1/backends/:id      # 更新
DELETE /api/v1/backends/:id      # 删除
```

### 4.2 响应格式

```go
// 成功
{
    "code": 0,
    "message": "success",
    "data": { ... }
}

// 错误
{
    "code": 400,
    "message": "invalid request",
    "error": "detailed error message"
}
```

### 4.3 分页

```
GET /api/v1/backends?page=1&page_size=20

Response:
{
    "data": [...],
    "total": 100,
    "page": 1,
    "page_size": 20
}
```

---

## 5. 配置规范

### 5.1 环境变量

- 前缀：`LLM_PROXY_`
- 大写 + 下划线：`LLM_PROXY_SERVER_PORT`
- 敏感值：`LLM_PROXY_*_SECRET`、`LLM_PROXY_*_KEY`

### 5.2 配置文件

- 格式：YAML 或 JSON
- 位置：`archive/deprecated/configs/` 或 `config/secrets/`
- 敏感配置：`config/secrets/` 目录，不提交到 Git

### 5.3 默认值

```go
// internal/config/defaults.go
const (
    DefaultServerPort = 20060
    DefaultTimeout    = 30 * time.Second
    DefaultMaxRetries = 3
)
```

---

## 6. Git 规范

### 6.1 提交信息

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Type**：
- `feat`: 新功能
- `fix`: 修复
- `docs`: 文档
- `style`: 格式（不影响代码运行）
- `refactor`: 重构
- `test`: 测试
- `chore`: 构建/工具

**示例**：
```
feat(cache): add semantic cache support

- Implement ChromaDB integration
- Add embedding service
- Update cache manager

Closes #123
```

### 6.2 分支命名

```
feature/add-semantic-cache
fix/backend-health-check
docs/update-api-docs
```

### 6.3 代码审查

- 所有代码通过 PR 合并
- 至少一人审查
- CI 通过后才能合并

---

## 7. 依赖管理

### 7.1 Go Modules

```bash
# 添加依赖
go get github.com/package/name

# 整理依赖
go mod tidy

# 更新依赖
go get -u github.com/package/name
```

### 7.2 版本锁定

- 使用 `go.sum` 锁定版本
- 定期更新依赖
- 避免使用未发布的版本

---

## 8. 日志规范

### 8.1 日志级别

```go
logger.Debug("debug message", zap.String("key", "value"))
logger.Info("info message", zap.Int("count", 42))
logger.Warn("warning message", zap.Error(err))
logger.Error("error message", zap.Error(err))
```

### 8.2 日志字段

- 必须包含：操作、对象、结果
- 可选：请求 ID、用户 ID、耗时
- 敏感信息：脱敏或不记录

### 8.3 日志格式

```json
{
    "level": "info",
    "ts": 1714233600,
    "msg": "backend health check completed",
    "backend": "openai",
    "healthy": true,
    "latency_ms": 150
}
```

---

## 9. 性能规范

### 9.1 并发安全

- 共享状态必须同步
- 优先使用 `sync.Mutex` 或 `sync.RWMutex`
- 避免全局变量

### 9.2 内存管理

- 避免内存泄漏
- 及时释放资源（使用 `defer`）
- 大对象使用池化

### 9.3 连接池

```go
// 数据库连接池
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

---

## 10. 安全规范

### 10.1 敏感数据

- 密码：使用 `bcrypt` 或 `argon2`
- Token：使用安全随机数生成
- API Key：加密存储

### 10.2 输入验证

```go
// 验证所有外部输入
if err := validate.Struct(request); err != nil {
    return nil, fmt.Errorf("validation failed: %w", err)
}
```

### 10.3 SQL 注入防护

```go
// 使用参数化查询
db.Query("SELECT * FROM users WHERE id = $1", userID)

// 避免字符串拼接
// db.Query("SELECT * FROM users WHERE id = " + userID) // 危险！
```

---

## 11. Lint 配置

项目使用 `golangci-lint`，配置见 `.golangci.yml`：

```bash
# 运行 lint
make lint

# 自动修复
golangci-lint run --fix
```

---

## 12. 快速参考

### 常用命令

```bash
make fmt          # 格式化代码
make lint         # 运行 lint
make test         # 运行测试
make build        # 构建
make run          # 构建并运行
make harness-check # Harness 卫生检查
```

### 文件模板

**新处理器**：
```go
package handler

import (
    "github.com/gin-gonic/gin"
)

// XxxHandler 处理 xxx 请求
type XxxHandler struct {
    // 依赖
}

// NewXxxHandler 创建 XxxHandler
func NewXxxHandler(...) *XxxHandler {
    return &XxxHandler{...}
}

// Handle 处理请求
func (h *XxxHandler) Handle(c *gin.Context) {
    // 实现
}
```

---

*最后更新：2026-04-27*
*参考：Go Code Review Comments、Effective Go*
