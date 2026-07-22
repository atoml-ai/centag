# Centag Protocol Test — 正本

> **业务正本**（`docs/harness/skills/`）。交互入口在 `.opencode/skills/centag-protocol-test/`；收参后交接本文件。分层见根目录 **`AGENT.md`**。

## 适用范围

- 适用于 **OpenAI 兼容协议**（`/v1/chat/completions`）和 **Anthropic 兼容协议**（`/v1/messages`）
- 验证协议字段的完整传递：请求解析 → 内部格式 → Mock 后端 → 内部格式 → 响应构建
- 不需要真正连接后端大模型，使用 Mock 数据

## 测试原则

**核心原则**：验证经过 Centag 代理后，输入参数能完整传到后端，返回的数据按标准协议返回给用户。

**关键验证点**：
1. 请求字段完整性（ParseRequest）
2. RawBody 透传能力（P2 字段、未知字段）
3. 响应字段完整性（HandleResponse）
4. 流式响应格式（FormatStreamChunk）
5. 错误处理（畸形 JSON、缺失字段）
6. 并发安全性

## 测试架构

```
┌─────────────────────────────────────────────────────────────┐
│                    协议完整性测试套件                         │
├─────────────────────────────────────────────────────────────┤
│  Test Layer 1: 协议插件单元测试（现有）                      │
│  - ParseRequest 字段完整性                                   │
│  - HandleResponse 字段完整性                                 │
│  - FormatStreamChunk 流式格式                                │
├─────────────────────────────────────────────────────────────┤
│  Test Layer 2: 端到端协议转换测试（新增）                    │
│  - Request → ProxyRequest → MockBackend → ProxyResponse → Response │
│  - 全链路字段透传验证                                        │
├─────────────────────────────────────────────────────────────┤
│  Test Layer 3: 协议边界用例测试（新增）                      │
│  - 所有 P0/P1 字段覆盖                                      │
│  - 异常输入/畸形数据处理                                     │
│  - 并发安全性                                               │
└─────────────────────────────────────────────────────────────┘
```

## 测试用例矩阵

### OpenAI 协议测试用例

| 分类 | 用例 | 验证点 |
|------|------|--------|
| **请求解析** | 基础字段 | model, messages, temperature, max_tokens, top_p, frequency_penalty, presence_penalty, stop |
| | 工具调用 | tools, tool_choice (auto/none/required/object) |
| | 响应格式 | response_format (text/json_object/json_schema) |
| | 种子/采样 | seed, n |
| | 用户追踪 | user |
| | 并行工具 | parallel_tool_calls |
| | 推理参数 | reasoning_effort |
| | RawBody | 保留未知字段供后端透传 |
| **响应构建** | 使用量 | prompt_tokens, completion_tokens, total_tokens |
| | 系统指纹 | system_fingerprint |
| | 拒绝响应 | refusal |
| | 服务层级 | service_tier |
| | 使用明细 | prompt_tokens_details, completion_tokens_details |
| **流式响应** | 基础流 | delta 内容递增 |
| | 工具流 | tool_calls 增量 |
| | 推理流 | reasoning_content 增量 |
| | 使用量事件 | 结束时 usage 事件（G6 判据） |
| **边界情况** | 畸形 JSON | 不崩溃，返回错误 |
| | 空消息 | 验证处理 |
| | Vision 格式 | MessageContent 数组解析 |
| | 并发请求 | 线程安全 |
| | 超长内容 | 不截断 |

### Anthropic 协议测试用例

| 分类 | 用例 | 验证点 |
|------|------|--------|
| **请求解析** | 基础字段 | model, max_tokens, messages, temperature, top_p, system |
| | 工具调用 | tools, tool_choice |
| | 思考模式 | thinking (type, budget_tokens) |
| | 元数据 | metadata.user_id |
| | 流选项 | stream_options |
| | RawBody | map 类型，保留未知字段 |
| | Tool 转换 | Anthropic → 内部 OpenAI 格式 |
| | tool_result | tool_use_id 回环 |
| **响应构建** | 使用量 | input_tokens, output_tokens |
| | 缓存令牌 | cache_creation_input_tokens, cache_read_input_tokens |
| | 停止序列 | stop_sequence |
| | 内容块 | text, tool_use |
| | 错误格式 | G2 修复：error 为对象 |
| | 默认 stop_reason | end_turn |
| **流式响应** | 事件流 | message_start → content_block_delta → message_stop |
| | 思考流 | thinking 事件 (index=1) |
| | 工具流 | tool_use 事件 |
| | stop_reason 映射 | tool_calls → tool_use |
| **边界情况** | 畸形 JSON | 不崩溃 |
| | thinking 禁用 | 不设置 Reasoning |
| | 空内容块 | 返回空文本块 |
| | 多种 content 类型 | text, image, tool_result |
| | 并发请求 | 线程安全 |
| | tool_use_id 映射 | 正确提取 |

## 测试流程

### Step 1: 执行协议测试

```bash
# 运行 OpenAI 协议端到端测试
go test ./plugins/protocol/openai/ -run TestOpenAIProtocolE2E -v -count=1

# 运行 Anthropic 协议端到端测试
go test ./plugins/protocol/anthropic/ -run TestAnthropicProtocolE2E -v -count=1

# 运行所有协议测试（包括现有单元测试）
go test ./plugins/protocol/... -v -count=1
```

### Step 2: 生成覆盖率报告

```bash
# 生成覆盖率
go test ./plugins/protocol/... -coverprofile=coverage.out

# 查看覆盖率
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html
```

### Step 3: 生成测试报告

运行测试并生成 HTML 报告：

```bash
# 运行测试并输出 JSON 格式结果
go test ./plugins/protocol/... -v -json -count=1 > test_results.json

# 生成 HTML 报告（使用项目内置脚本）
go run scripts/generate-protocol-report.go
```

### Step 4: 验证测试结果

```bash
# 检查测试通过率
grep -E "^(ok|FAIL)" test_output.txt

# 检查覆盖率
grep "total:" coverage.txt
```

## 测试报告格式

HTML 报告包含以下部分：

1. **测试概览**：总用例数、通过数、失败数、跳过数
2. **协议覆盖**：OpenAI / Anthropic 分别统计
3. **分类统计**：请求解析、响应构建、流式响应、边界情况
4. **详细结果**：每个用例的执行时间和状态
5. **覆盖率**：行覆盖、分支覆盖、函数覆盖
6. **问题诊断**：失败用例的详细错误信息

## Mock 后端实现

测试使用 `plugins/protocol/shared/test_helpers.go` 中的 `MockBackend`：

```go
type MockBackend struct {
    CapturedRequest *plugin.ProxyRequest  // 捕获的请求
    CapturedRawBody map[string]interface{} // 捕获的 RawBody
    Response        *plugin.ProxyResponse  // 预设响应
    StreamChunks    []*plugin.StreamChunk  // 流式 chunks
    Error           error                  // 预设错误
    CallCount       int                    // 调用计数
}
```

## 通过判定

单次请求判定通过需同时满足：

1. `ParseRequest` 无错误
2. 请求字段完整传递到 Mock 后端
3. RawBody 保留所有原始字段
4. `HandleResponse` 无错误
5. 响应格式符合协议标准
6. 流式响应格式正确

整组测试通过建议：

- 所有用例通过
- 覆盖率 ≥ 80%
- 无并发安全问题

## 测试文件结构

```
plugins/protocol/
├── shared/
│   └── test_helpers.go          # 共享测试工具和 Mock
├── openai/
│   ├── protocol_test.go         # 现有单元测试
│   └── protocol_e2e_test.go     # 端到端协议转换测试
└── anthropic/
    ├── protocol_test.go         # 现有单元测试
    └── protocol_e2e_test.go     # 端到端协议转换测试
```

## 常见问题诊断

### 问题1: ParseRequest 失败
**症状**：测试在 ParseRequest 阶段报错
**原因**：JSON 格式错误、字段类型不匹配
**修复**：检查请求 JSON 格式，确认字段类型

### 问题2: RawBody 为空
**症状**：Mock 后端未捕获 RawBody
**原因**：ParseRequest 未正确存储 RawBody
**修复**：检查协议插件的 ParseRequest 实现

### 问题3: 响应格式不符
**症状**：HandleResponse 输出不符合协议标准
**原因**：字段映射错误、缺少必要字段
**修复**：检查协议插件的 HandleResponse 实现

### 问题4: 流式事件缺失
**症状**：流式测试缺少预期事件
**原因**：FormatStreamChunk 逻辑错误
**修复**：检查流式事件生成逻辑
