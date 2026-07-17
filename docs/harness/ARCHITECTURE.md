# Centag 架构约束文档

> 面向 AI 编码智能体：定义系统的分层结构、模块边界、依赖关系。
> 本文档是架构的"宪法"，修改需谨慎并同步更新相关文档。

---

## 1. 系统概览

Centag 是一个高性能 LLM 反向代理/网关，采用 Go 语言构建，核心职责：

```
客户端请求 → [认证/限流] → [模式解析] → [流水线编排] → [插件节点执行] → [后端转发/记忆/工具] → 响应返回
```

**技术栈**：
- 语言：Go 1.23.7
- HTTP 框架：Gin
- 数据库：SQLite（默认）/ PostgreSQL（生产）
- 缓存：Redis（可选）
- 向量库：ChromaDB（语义缓存）
- 搜索：Elasticsearch（可选）
- 前端：Vue 3 + Vite（构建到 `static/`）

---

## 2. 分层架构

### 2.1 固定分层（依赖只能向前）

```
┌─────────────────────────────────────────────────────────────┐
│  cmd/                    ← 入口层：仅 main() 与组装         │
├─────────────────────────────────────────────────────────────┤
│  internal/server/        ← 服务层：HTTP 服务器、路由注册    │
├─────────────────────────────────────────────────────────────┤
│  internal/               ← 领域层：核心业务抽象             │
│    ├── backend/          │   后端管理                       │
│    ├── cache/            │   缓存管理                       │
│    ├── scheduler/        │   调度器                         │
│    ├── pipeline/         │   管道处理（含路由节点）           │
│    ├── processor/        │   请求处理器                     │
│    └── ...               │                                  │
├─────────────────────────────────────────────────────────────┤
│  internal/config/        ← 配置层：环境变量、默认值         │
├─────────────────────────────────────────────────────────────┤
│  internal/database/      ← 数据层：数据库连接与迁移         │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 依赖方向规则

```
cmd → server → 领域层 → config/database
             ↓
         plugins/ (通过接口/注册发现)
```

**铁律**：
- ✅ 外层可以依赖内层
- ❌ 内层不能依赖外层
- ❌ 同层之间避免循环依赖
- ❌ `internal/` 不能引用 `plugins/`（只能通过接口）

---

## 3. 模块边界

### 3.1 `cmd/` — 入口层

**职责**：程序入口、依赖注入组装

**约束**：
- 只放 `main.go` 和初始化逻辑
- 不写业务逻辑
- 每个子目录是一个独立可执行程序

**现有入口**：
| 入口 | 用途 |
|------|------|
| `cmd/centag/` | 主服务 |
| `cmd/migrate/` | 数据库迁移 |
| `cmd/processor-verify/` | 处理器验证工具 |

### 3.2 `internal/server/` — 服务层

**职责**：HTTP 服务器生命周期、路由注册、中间件配置

**约束**：
- 负责组装所有 handler
- 管理服务器启动/关闭
- 不包含业务逻辑

**关键文件**：
- `server.go` — 服务器核心，依赖注入中心
- `router.go` — 路由注册

### 3.3 `internal/` — 领域层

**职责**：核心业务逻辑、领域模型

**子模块职责边界**：

| 模块 | 职责 | 边界 |
|------|------|------|
| `backend/` | 后端服务管理、健康检查 | 不处理请求转发 |
| `cache/` | 缓存策略、精确/语义匹配 | 不处理存储细节 |
| `scheduler/` | 请求调度、负载均衡 | 不处理路由决策 |
| `pipeline/` | 请求处理管道编排（含 `NodeTypeRouter`） | 不实现具体处理 |
| `processor/` | 具体请求处理逻辑 | 不编排流程 |
| `proxy/` | 代理转发核心 | 不处理业务逻辑 |
| `config/` | 配置加载、默认值 | 不修改配置 |
| `database/` | 数据库连接、迁移 | 不包含业务查询 |

### 3.4 `plugins/` — 插件层

**职责**：可插拔实现，通过接口与核心交互

**插件类型**：
- `plugins/backend/` — 后端实现（OpenAI、Claude、Ollama）
- `plugins/protocol/` — 协议实现
- `plugins/storage/` — 存储实现（Redis、ChromaDB、Elasticsearch）

**约束**：
- 必须实现 `internal/plugin/` 中定义的接口
- 通过 `_ import` 注册
- 不能被 `internal/` 直接引用

### 3.5 `web/` — 客户端应用层

**职责**：面向用户的客户端应用，消费主项目的标准接口

**应用列表**：
| 应用 | 技术栈 | 用途 |
|------|--------|------|
| `web/` | Vue 3 + Element Plus | Web 管理界面 |
| `apps/launcher/` | Go + systray | 可选桌面启动器：菜单/托盘 + 系统浏览器（L1） |

**约束**：
- 仅通过 HTTP API / 子进程与主项目交互，不能直接引用 `core/internal` 业务包
- 构建产物：web 输出到 `bin/server/static/`；launcher 输出到 `bin/launcher/`（可选）
- launcher 独立 `go.mod`，不加入根 `go.work`，删除不影响发行版

**详细说明**：见 `web/README.md`、`apps/launcher/README.md`

### 3.6 `deploy/stack/` — 基础设施层（子模块）

**职责**：中间件和可选服务编排

**服务列表**：
| 服务包 | 路径 | 用途 |
|--------|------|------|
| middleware | `middleware/` | PG、Redis、ES、Qdrant、Neo4j、Ollama |
| mem0 | `services/mem0/` | 记忆服务（依赖 middleware） |
| pi-sandbox | `services/pi-sandbox/` | Pi Agent 沙盒（独立） |

**约束**：
- 是 git submodule，独立版本管理
- 被主项目依赖，不依赖主项目
- 按需启动，可独立部署

**详细说明**：见 `deploy/stack/README.md`

---

## 4. 应用层与基础设施层边界

### 4.1 架构分层全景

```
┌─────────────────────────────────────────────────────────────┐
│  Apps (应用层)           ← 用户界面、交互入口               │
│    ├── web/             │   Vue3 Web 管理界面              │
│    └── apps/launcher/   │   可选桌面启动器（浏览器 UI）     │
├─────────────────────────────────────────────────────────────┤
│  Centag (核心)       ← LLM 反向代理/网关                │
│    ├── cmd/ / dist/     │   入口层                         │
│    ├── core/            │   领域层                         │
│    └── plugins/         │   插件层                         │
├─────────────────────────────────────────────────────────────┤
│  Stack (基础设施)       ← 中间件、可选服务                  │
│    ├── middleware/       │   PG/Redis/ES/Qdrant/Neo4j      │
│    ├── mem0/            │   记忆服务                       │
│    └── pi-sandbox/      │   Pi Agent 沙盒                  │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 Apps 与 Stack 的区别

| 维度 | Apps | Stack |
|------|------|-------|
| **定位** | 客户端应用 | 基础设施 |
| **职责** | 用户界面、交互 | 中间件、可选服务 |
| **依赖方向** | 依赖主项目 API | 被主项目依赖 |
| **部署方式** | 可独立运行 | 按需启动 |
| **技术栈** | Vue/Go (客户端) | Docker/Compose (服务端) |
| **示例** | web、apps/launcher | PG、Redis、Mem0 |

### 4.3 依赖规则

```
Apps ──HTTP/API──▶ Centag ──配置──▶ Stack
  │                   │                  │
  └── 消费标准接口     └── 提供服务        └── 被依赖
```

**铁律**：
- ✅ Apps 可以调用主项目的 HTTP API
- ✅ 主项目可以配置和启动 Stack 服务
- ❌ Apps 不能直接引用 `internal/` 的包
- ❌ Stack 不能依赖主项目的代码
- ❌ Apps 不能直接访问 Stack 的服务（需通过主项目）

---

## 5. 横切关注点

### 5.0 插件运行时（核心架构）

#### 5.0.1 请求处理全链路

```
客户端请求
   ↓
[认证/限流中间件] → internal/middleware/
   ↓
[代理模式解析] → internal/proxymode/
   ↓
[流水线模板决策] → internal/proxy + internal/pipeline/
   ↓
[PipelineEngine 初始化]
   ↓
[CapabilityBroker 注入] ← 权限控制中心
   ↓
[节点执行（NodePlugin）] → internal/pipeline/
   ├─ LLM 调用（通过 CapabilityBroker）
   ├─ Storage 读写（通过 CapabilityBroker）
   ├─ Memory 访问（通过 CapabilityBroker）
   └─ Secrets 读取（通过 CapabilityBroker）
   ↓
[路由决策/调度选择] → internal/pipeline/（含 NodeTypeRouter）+ internal/scheduler/
   ↓
[缓存查询] → internal/cache/
   ↓ (未命中)
[后端转发] → internal/proxy/ → internal/backend/
   ↓
[响应处理] → internal/processor/
   ↓
[缓存存储] → internal/cache/
   ↓
客户端
```

#### 5.0.2 CapabilityBroker 架构

**职责**：统一管控插件能力访问，实现权限强制约束。

**位置**：`internal/pipeline/capability_broker.go`

**核心接口**：
```go
type CapabilityBroker interface {
    // 权限检查
    CheckPermission(pluginID string, capability string) error
    
    // LLM 能力
    CallLLM(ctx context.Context, pluginID string, req *LLMRequest) (*LLMResponse, error)
    
    // 存储能力
    ReadStorage(ctx context.Context, pluginID, key string) ([]byte, error)
    WriteStorage(ctx context.Context, pluginID, key string, value []byte) error
    
    // 记忆能力
    ReadMemory(ctx context.Context, pluginID, query string) ([]MemoryItem, error)
    WriteMemory(ctx context.Context, pluginID string, item *MemoryItem) error
    
    // 密钥能力
    ReadSecret(ctx context.Context, pluginID, key string) (string, error)
    
    // HTTP 能力
    HTTPClient(pluginID string) (*http.Client, error)
}
```

**权限模型**：
- 每个 NodePlugin 在 descriptor 中声明 `permissions` 字段
- 执行前由 CapabilityBroker 校验权限
- 未授权调用返回 `ErrPermissionDenied` 并记录审计日志

**约束**：
- ✅ 插件能力访问必须通过 `CapabilityBroker`，禁止直连内部全局管理器
- ✅ 远程插件必须有可验证 descriptor（`implementation`、`kind`、`version`）
- ✅ 权限越权时显式拒绝 + 审计事件（非 silent fail）
- ✅ 模式逻辑优先沉淀为"模板 + 节点"，减少代理层硬编码分支

**当前状态（2026-05-05）**：
- ✅ 接口定义完成
- 🔄 主链路注入点待完成（`internal/proxy/pipeline_mode.go` 有 TODO）
- ❌ 权限强制约束未完全接入（部分路径仍可绕过）
- ❌ 审计日志未完整实现

详细权限模型见 `docs/security/permission-model.md`。

### 5.1 认证（Auth）

```
internal/auth/        ← 认证核心
internal/middleware/  ← HTTP 中间件
```

**流向**：请求 → middleware → auth 验证 → handler

### 5.2 日志（Logger）

```
internal/logger/      ← 日志抽象
```

**约束**：全局使用 `zap`，通过 `internal/logger` 统一访问

### 5.3 监控（Metrics）

```
internal/metrics/     ← 指标收集
internal/monitor/     ← 监控逻辑
```

**约束**：通过 `/api/v1/monitor/stats` 暴露

### 5.4 会话（Session）

```
internal/session/     ← 会话管理
```

**约束**：代理模式状态存储

### 5.5 能力边界与分层策略

> 本节定义"核心层提供什么"与"流水线/插件提供什么"的边界。完整能力矩阵见 [CAPABILITY_BOUNDARY.md](CAPABILITY_BOUNDARY.md)。

**设计原则**：**薄核心 + 厚管道 + 可插拔实现**

```
核心层 (internal/)        → 协议翻译 + 请求编排 + 基础设施
流水线层 (internal/pipeline/) → 处理链编排 + 节点调度
插件层 (plugins/)         → 具体实现（协议/后端/存储/业务）
```

**核心层能力**（所有 LLM 网关都必须具备）：

| 模块 | 职责 | 边界 |
|------|------|------|
| `server/` | HTTP 服务器生命周期 | 路由注册、中间件组装 / 不含业务逻辑 |
| `proxy/` | 请求转发、代理模式解析 | 协议翻译、模式检测、转发 / 不做路由决策 |
| `pipeline/` | 流水线引擎、节点编排 | 流程控制、节点执行、CapabilityBroker / 不实现具体节点 |
| `scheduler/` | 调度框架、负载均衡 | 评分算法、熔断器、负载均衡 / 不做具体后端选择策略 |
| `backend/` | 后端抽象、健康检查 | 接口定义、状态管理 / 不含具体协议实现 |
| `auth/` | 认证授权 | JWT 验证、API Key 校验 / 不含业务权限 |
| `middleware/` | HTTP 中间件 | 认证注入、限流、代理模式解析 / 不含业务处理 |
| `config/` | 配置加载 | 环境变量、数据库配置、默认值 / 不含运行时逻辑 |
| `database/` | 数据库连接、迁移 | 连接管理、schema 迁移 / 不含业务查询 |
| `tokenusage/` | Token 用量服务 | 用量记录、配额检查、成本汇总 / 不含计费策略 |

**流水线节点能力**（可编排的处理单元）：

| 类型 | 节点 | 说明 |
|------|------|------|
| LLM 核心 | `builtin.generator` / `builtin.processor` / `builtin.reviewer` | 生成、后处理、审核 |
| 流量控制 | `builtin.router` / `builtin.aggregator` / `builtin.parallel` | 分流、聚合、并行 |
| 能力插件 | `builtin.memory` / `builtin.token_optimizer` / `builtin.content_moderator` | 记忆、优化、审核 |
| 业务插件 | `business.question_splitter` / `business.answer_synthesizer` / `business.tasktype_detector` | 问题拆分、答案合成、任务检测 |

**插件层能力**（具体技术实现）：

| 类型 | 插件目录 | 说明 |
|------|---------|------|
| 协议 | `plugins/protocol/` | OpenAI、Anthropic、可扩展 |
| 后端 | `plugins/backend/` | OpenAI、Anthropic、Ollama、可扩展 |
| 存储 | `plugins/storage/` | Redis、PostgreSQL、本地文件、可扩展 |
| 业务 | `plugins/business/` | 行业场景处理 |

**边界划分判断**：

```
这是所有 LLM 网关都需要的基础能力？
  → 是 → 核心层 (internal/)
  → 否 ↓
这是一个可编排的处理步骤？
  → 是 → 流水线节点 (internal/pipeline/nodes/)
  → 否 ↓
这是某个具体技术的实现？
  → 是 → 插件 (plugins/)
  → 否 → 不做，或通过配置表达
```

---

## 6. 数据流架构

### 6.1 请求处理流程

```
客户端
  ↓
[认证中间件] → internal/middleware/
  ↓
[代理模式解析] → internal/proxymode/
  ↓
[流水线模板决策] → internal/proxy + internal/pipeline/
  ↓
[节点执行（含插件）] → internal/pipeline/
  ↓
[路由决策/调度选择] → internal/pipeline/（含 NodeTypeRouter）+ internal/scheduler/
  ↓
[缓存查询] → internal/cache/
  ↓ (未命中)
[后端转发] → internal/proxy/ → internal/backend/
  ↓
[响应处理] → internal/processor/
  ↓
[缓存存储] → internal/cache/
  ↓
客户端
```

### 6.2 配置加载流程

```
环境变量 → internal/config/bootstrap.go
  ↓
配置文件 → internal/config/config.go
  ↓
数据库配置 → internal/config/db_loader.go
  ↓
运行时配置 → internal/config/
```

---

## 7. 接口契约

### 7.1 核心接口位置

| 接口 | 位置 | 用途 |
|------|------|------|
| Backend | `internal/backend/` | 后端服务抽象 |
| Storage | `internal/storage/` | 存储抽象 |
| Protocol | `internal/plugin/` | 协议抽象 |
| Processor | `internal/processor/` | 处理器抽象 |
| NodePlugin | `internal/pipeline/` | 流水线节点插件抽象 |
| CapabilityBroker | `internal/pipeline/` | 插件受控能力发放 |

### 7.2 插件注册模式

```go
// plugins/backend/openai/init.go
package openai

import "centag/internal/plugin"

func init() {
    plugin.RegisterBackend("openai", NewOpenAIBackend)
}
```

---

## 8. 边界强制执行

### 8.1 机械化检查

运行 `make harness-check` 或 `bash scripts/check-harness-hygiene.sh` 验证：
- 关键文档存在
- Go 包列表正确（排除 `web/node_modules`）

### 8.2 CI 门禁

GitHub Actions `.github/workflows/ci.yml` 执行：
- `go test` — 单元测试
- `golangci-lint` — 代码规范
- `webui` ESLint — 前端规范

### 8.3 本地对齐

```bash
make test           # 运行测试
make lint           # 运行 lint
cd webui && npm run lint:ci  # 前端 lint
```

---

## 9. 扩展指南

### 9.1 添加新后端

1. 在 `plugins/backend/` 创建新目录
2. 实现 Backend 接口
3. 通过 `init()` 注册
4. 在 `internal/server/server.go` 添加 `_ import`

### 9.2 添加新处理器

1. 在 `internal/processor/` 创建处理器
2. 实现 Processor 接口
3. 在 `internal/pipeline/` 注册

### 9.3 添加新存储

1. 在 `plugins/storage/` 创建新目录
2. 实现 Storage 接口
3. 通过 `init()` 注册

### 9.4 添加新流水线节点插件

1. 在 `internal/pipeline/nodes/` 创建新目录
2. 实现 NodePlugin 接口
3. 在描述符中声明所需 `permissions`
4. 在 `init()` 中注册到 NodeRegistry
5. 通过 CapabilityBroker 访问外部能力

详细插件开发指南见 `docs/plugin-development-guide.md`。

---

## 10. 禁止事项

- ❌ 在 `internal/` 中直接引用 `plugins/`
- ❌ 在 `cmd/` 中写业务逻辑
- ❌ 跨层跳跃依赖（如 handler 直接调 database）
- ❌ 在领域层引入 HTTP 框架依赖
- ❌ 循环依赖

---

## 11. 实施状态（2026-05-05）

### 11.1 架构完成度

| 模块 | 状态 | 备注 |
|------|------|------|
| 分层架构 | ✅ 完成 | 清晰的 cmd → server → internal → config/database |
| 插件机制 | ✅ 完成 | init() 注册 + 接口抽象 |
| 流水线引擎 | 🔄 进行中 | 模板化完成，CapabilityBroker 待接入 |
| 权限模型 | 🔄 进行中 | 接口定义完成，运行时强制待实现 |
| 持久化 | ❌ 待完成 | DBPluginRegistryStore 方法未完整实现 |
| 并发安全 | ❌ 待完成 | 远程节点需添加并发保护 |

### 11.2 严重技术问题

1. **CapabilityBroker 未完全接入**
   - 位置：`internal/proxy/pipeline_mode.go`
   - 风险：插件权限仅声明未强制，存在越权风险
   - 优先级：P0

2. **插件注册表持久化不完整**
   - 位置：`internal/plugin/registry/`
   - 风险：服务重启后插件状态丢失
   - 优先级：P0

3. **并发安全问题**
   - 位置：`plugins/` 远程节点健康检查、状态更新
   - 风险：多 goroutine 访问共享状态无保护
   - 优先级：P0

4. **测试覆盖率不足**
   - 68 个测试文件相对代码量偏低
   - `go test ./...` 超时存在慢测试或死锁
   - 优先级：P1

---

*最后更新：2026-07-03*
*维护者：见 AGENTS.md*
