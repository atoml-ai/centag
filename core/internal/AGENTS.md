# internal/ — 核心业务代码

> 面向 Agent：本目录是项目的核心，包含所有业务逻辑。**这是最重要的目录。**

## 目录职责

存放核心业务实现，遵循分层架构，依赖只能向前。

## 子目录

### 配置与基础设施

| 目录 | 职责 |
|------|------|
| `config/` | 配置加载、默认值、环境变量 |
| `database/` | 数据库连接、迁移 |
| `logger/` | 日志抽象 |
| `errors/` | 错误定义 |
| `common/` | 公共工具 |

### 核心业务

| 目录 | 职责 |
|------|------|
| `backend/` | 后端服务管理、健康检查 |
| `cache/` | 缓存管理（精确 + 语义） |
| `scheduler/` | 请求调度、负载均衡 |
| `router/` | 路由规则、模型匹配 |
| `pipeline/` | 请求处理管道 |
| `processor/` | 请求处理器 |
| `proxy/` | 代理转发核心 |

### HTTP 层

| 目录 | 职责 |
|------|------|
| `server/` | HTTP 服务器、路由注册 |
| `handler/` | HTTP 请求处理器 |
| `middleware/` | 中间件（认证、限流等） |

### 扩展功能

| 目录 | 职责 |
|------|------|
| `auth/` | 认证逻辑 |
| `plugin/` | 插件管理 |
| `storage/` | 存储抽象 |
| `embedding/` | 向量化 |
| `llm/` | LLM 协议处理 |
| `session/` | 会话管理 |
| `tokenusage/` | Token 统计 |
| `metrics/` | 指标收集 |
| `monitor/` | 监控逻辑 |

## 依赖方向（铁律）

```
server/ → handler/ → 领域层 → config/database
                ↓
            plugins/ (通过接口)
```

- ✅ 外层可以依赖内层
- ❌ 内层不能依赖外层
- ❌ 同层之间避免循环依赖
- ❌ 不能引用 `plugins/`（只能通过 `_ import` 注册）

## 约束

- ❌ **禁止**跨层依赖（如 handler 直接调 database）
- ❌ **禁止**循环依赖
- ❌ **禁止**全局可变状态
- ❌ **禁止**在领域层引入 HTTP 框架依赖
- ✅ **必须**：新增功能配测试
- ✅ **必须**：遵循 `../docs/harness/CONVENTIONS.md` 编码规范

## 添加新模块

1. 确定模块在分层中的位置
2. 创建目录和接口定义
3. 实现功能
4. 添加测试
5. 在 `server.go` 中组装
6. 更新本文件的子目录表

## 相关文档

- 架构约束：`../docs/harness/ARCHITECTURE.md`
- 编码规范：`../docs/harness/CONVENTIONS.md`
- 设计模式：`../docs/harness/PATTERNS.md`
- 反模式：`../docs/harness/ANTI-PATTERNS.md`

---

*最后更新：2026-04-27*
