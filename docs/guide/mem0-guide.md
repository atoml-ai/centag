# Mem0 插件集成 - 技术方案

## 📋 方案概述

本方案将 **Mem0 记忆服务**集成到 Centag 的流水线系统中，通过**业务插件**方式实现用户输入和模型输出的**自动记忆存储**，实现**用户无感知的记忆存储**。

## 🎯 核心目标

1. **用户无感知**：用户正常使用 API，无需额外调用记忆存储接口
2. **自动存储**：用户输入 + 模型输出自动保存到 Mem0
3. **流水线集成**：BusinessPlugin 插件方式接入 Centag 流水线系统
4. **可配置**：支持开关、命名空间、过滤规则等配置

## 📁 完成的工作

### 1. 插件实现

**文件**: `plugins/business/mem0/plugin.go`

实现了完整的 Mem0 插件，包括：
- ✅ Mem0Config 配置结构
- ✅ Mem0Plugin 插件主实现
- ✅ Mem0Client Mem0 API 客户端
- ✅ Execute() 插件执行逻辑
- ✅ 过滤规则检查
- ✅ 插件注册函数
- ✅ BusinessPlugin 接口实现

### 2. 流水线模板

**文件**: `internal/pipeline/config.go`

添加了 Mem0 记忆存储模板：
- ✅ 模板 ID: `mem0-memory`
- ✅ Generator 节点（生成回答）
- ✅ Mem0 Storage 节点（记忆存储）
- ✅ 默认配置和过滤规则

### 3. 模式映射

**文件**: `internal/proxy/mode_dispatcher.go`

添加了 Mem0 模式映射：
- ✅ Mode: `mem0-memory`
- ✅ PipelineID: `mem0-memory`

### 4. 文档

- ✅ `docs/mem0-integration-tech-spec.md` - 完整技术方案
- ✅ `docs/mem0-quickstart.md` - 快速开始指南
- ✅ `docs/MEM0_INTEGRATION_SUMMARY.md` - 集成总结
- ✅ `plugins/business/mem0/BUSINESS_PLUGIN.md` - 插件文档

### 5. 测试脚本

- ✅ `test-mem0-integration.sh` - 集成测试脚本

## 🚀 快速开始

### 1. 启动 Mem0 服务

```bash
cd /root/workspaces/centag/deploy/stack

# 启动基础中间件（PostgreSQL, Qdrant, Neo4j）
./start.sh start base

# 启动 Mem0 服务
./start.sh start mem0

# 验证 Mem0 服务状态
./start.sh status
```

### 2. 配置环境变量

在 `deploy/stack/.env` 中配置：

```bash
MEM0_PORT=20061
MEM0_SERVER_IMAGE=qceank/mem0-server:production
MEM0_OPENAI_API_KEY=<你的 OpenAI API Key>
MEM0_ADMIN_API_KEY=<自定义管理密钥>
MEM0_AUTH_DISABLED=false
```

### 3. 启动 Centag

```bash
cd /root/workspaces/centag
./start.sh run be
```

### 4. 测试 Mem0 插件

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "X-Pipeline-ID: mem0-memory" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "qwen3.5-plus",
    "messages": [
      {"role": "user", "content": "我今天学习了 Go 语言的接口概念"}
    ]
  }'
```

### 5. 验证记忆存储

```bash
# 查询 Mem0 记忆
ADMIN_KEY=$(grep "MEM0_ADMIN_API_KEY" /root/workspaces/centag/deploy/stack/.env | cut -d'=' -f2 | tr -d '"')

# 查询记忆列表（Mem0 v2.0+ API）
curl -X GET "http://localhost:20061/memories?user_id=0&agent_id=centag_conversation" \
  -H "Authorization: Bearer ${ADMIN_KEY}"

# 搜索记忆（Mem0 v2.0+ API）
curl -X POST "http://localhost:20061/search" \
  -H "Authorization: Bearer ${ADMIN_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Go 语言",
    "user_id": "0",
    "agent_id": "centag_conversation"
  }'
```

## 📖 文档索引

- **快速开始**: `docs/mem0-quickstart.md`
- **技术方案**: `docs/mem0-integration-tech-spec.md`
- **集成总结**: `docs/MEM0_INTEGRATION_SUMMARY.md`
- **插件文档**: `plugins/business/mem0/BUSINESS_PLUGIN.md`

## 🎯 核心特性

### 用户无感知
- 用户正常使用 API，无需额外调用记忆存储接口
- 插件自动拦截用户输入和模型输出
- 记忆存储对用户完全透明

### 自动存储
- 用户输入自动保存到 Mem0
- 模型输出自动保存到 Mem0
- 可选存储元数据（时间戳、请求 ID 等）

### 可配置
- 支持开关（enabled）
- 支持命名空间隔离（namespace）
- 支持过滤规则（min_length, max_length, exclude_patterns）
- 支持用户 ID 来源配置（header, metadata, constant）

### 灵活过滤
- 最小长度过滤
- 最大长度过滤
- 正则排除模式

## 🔧 技术细节

### 插件类型

**BusinessPlugin**（业务插件）
- 业务类型：`memory`（记忆存储）
- 节点类型：`NodeTypeProcessor`
- 实现：`business.mem0`

### 执行流程

1. 检查插件是否启用（enabled）
2. 构建记忆内容（用户输入 + 模型输出）
3. 检查过滤规则（min_length, max_length, exclude_patterns）
4. 调用 Mem0 API 保存记忆
5. 返回执行结果

### Mem0 API 客户端

```go
type Mem0Client struct {
    baseURL  string
    apiKey   string
    httpClient *http.Client
}

func (c *Mem0Client) AddMemory(ctx context.Context, messages []Mem0Message, userID, namespace string) error
```

## 📋 配置参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `base_url` | string | 是 | `http://localhost:20061` | Mem0 服务地址 |
| `api_key` | string | 是 | - | Mem0 管理密钥 |
| `user_id_source` | string | 否 | `metadata` | 用户 ID 来源: `header`, `metadata`, `constant` |
| `user_id` | string | 否 | `0` | 固定用户 ID（当 `user_id_source=constant` 时） |
| `namespace` | string | 否 | `centag_conversation` | 记忆命名空间 |
| `enabled` | boolean | 否 | `true` | 是否启用插件 |

## 🧪 测试验证

### 运行集成测试

```bash
cd /root/workspaces/centag
bash test-mem0-integration.sh
```

### 手动测试

```bash
# 发送测试请求
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "X-Pipeline-ID: mem0-memory" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key" \
  -d '{"model": "qwen3.5-plus", "messages": [{"role": "user", "content": "Hello"}]}'

# 查询 Mem0 记忆
ADMIN_KEY=$(grep "MEM0_ADMIN_API_KEY" /root/workspaces/centag/deploy/stack/.env | cut -d'=' -f2 | tr -d '"')

# 查询记忆列表（Mem0 v2.0+ API）
curl -X GET "http://localhost:20061/memories?user_id=0&agent_id=centag_conversation" \
  -H "Authorization: Bearer ${ADMIN_KEY}"
```

## ⚠️ 注意事项

1. **Mem0 依赖**：确保 PostgreSQL、Qdrant、Neo4j 已启动
2. **API Key**：使用 `MEM0_ADMIN_API_KEY` 作为管理密钥
3. **用户 ID**：从请求头或元数据中提取，确保唯一性
4. **过滤规则**：可配置最小长度、排除模式等
5. **命名空间**：建议使用 `centag_conversation` 区分不同场景

## 📚 扩展功能

### 记忆查询插件
- 检索相关历史记忆
- 上下文增强
- RAG 场景

### 多用户支持
- 通过 `user_id_source` 配置支持
- `header`: 从请求头 `X-User-ID` 读取
- `metadata`: 从流水线元数据读取
- `constant`: 使用固定用户 ID

### 记忆过期
- TTL（Time To Live）
- 自动清理策略

## 📁 文件清单

### 新增文件

```
plugins/business/mem0/
├── plugin.go              # Mem0 插件主实现
└── BUSINESS_PLUGIN.md     # 插件文档

docs/
├── mem0-integration-tech-spec.md  # 完整技术方案
├── mem0-quickstart.md             # 快速开始指南
└── MEM0_INTEGRATION_SUMMARY.md    # 集成总结

test-mem0-integration.sh  # 集成测试脚本
```

### 修改文件

```
internal/pipeline/config.go          # 添加 Mem0 模板
internal/proxy/mode_dispatcher.go    # 添加 Mem0 模式映射
```

## 🎉 总结

Mem0 插件集成已完成，实现了用户无感知的记忆存储功能。通过 BusinessPlugin 插件方式接入 Centag 流水线系统，支持可配置的过滤规则和命名空间隔离。

所有核心功能已实现并测试通过，包括：
- ✅ 用户无感知
- ✅ 自动存储
- ✅ 可配置
- ✅ 灵活过滤
- ✅ 完整文档
- ✅ 测试脚本

---

**技术方案版本**: v1.0.0  
**最后更新**: 2026-05-26  
**作者**: AI Assistant
