# 推理（Reasoning）兼容性说明

Centag 通过 `Canonical.Reasoning` 归一化层，在不同厂商的推理参数之间进行双向映射。

## 架构概览

```
客户端请求                      协议插件解析                Backend 映射
─────────                      ─────────────              ─────────────
OpenAI                 ┌┐       reasoning.Effort       ┌┐
  reasoning_effort ────┤├──►  ┌────────────────────┐  ─►├├──── reasoning_effort / effort (Azure)
Anthropic              ││     │ NormalizeEffort()   │    ││
  thinking.budget  ────┤├──►  │ EffortToBudget()    │  ─►├├──── budget_tokens (DeepSeek)
DeepSeek               ││     │ BudgetToEffort()    │    ││
  reasoning_content ◄──┤├──►  └────────────────────┘  ◄──├├──── thinking.enabled + budget (Claude)
                        └┘                               └┘
```

协议插件将各厂商的原生 thinking/reasoning 字段解析后写入统一的 `plugin.ReasoningSpec`，backend 再反序列化为目标厂商的方言参数。

## 归一化核心：ReasoningSpec

定义在 `core/pkg/plugin/protocol.go:36-54`：

| 字段 | 类型 | 说明 |
|------|------|------|
| `Specified` | `bool` | 客户端是否显式指定了推理参数（区分「未设置」与「显式 none」） |
| `Disabled` | `bool` | 客户端是否显式禁用推理 |
| `Effort` | `string` | 标准化努力级别：`none` / `minimal` / `low` / `medium` / `high` / `xhigh` |
| `BudgetTokens` | `*int` | 推理 token 预算（与 Effort 互斥，优先使用 Effort） |

## Effort 级别映射

`core/pkg/protocol/reasoning/reasoning.go` 提供双向转换：

### Effort → Budget

| Effort | Budget Tokens |
|--------|--------------|
| `none` | 0 |
| `minimal` | 1,024 |
| `low` | 2,048 |
| `medium` | 4,096 |
| `high` | 8,192 |
| `xhigh` | 16,384 |

### 厂商字段映射规则

| 厂商协议 | 原生字段 | → `ReasoningSpec` | ← Backend 映射 |
|---------|---------|-------------------|---------------|
| **OpenAI** | `reasoning.effort` | `Effort` | `reasoning.effort` |
| | `reasoning.summary` | `Metadata` | `reasoning.summary` |
| **Anthropic** | `thinking.type` | `Disabled`（`disabled`→true） | `thinking.type: enabled/disabled` |
| | `thinking.budget_tokens` | `BudgetTokens` | `thinking.budget_tokens` |
| **DeepSeek** | `reasoning_effort` | `Effort`（通过 `NormalizeEffort`） | `reasoning_effort` |
| **Gemini** | `thinkingConfig.thinkingBudget` | `BudgetTokens` | `thinkingConfig.thinkingBudget` |
| | `thinkingConfig.includeThoughts` | `Metadata` | `thinkingConfig.includeThoughts` |
| **Azure** | `reasoning_effort` (OpenAI 兼容) | `Effort` | `reasoning_effort` |

## 推理内容（Reasoning Content）

各后端/模型返回的推理内容通过 `reasoning_content` 字段在 Centag 内部透传：

- **流式响应**：`StreamChunk.ReasoningContent` 承载增量推理文本（DeepSeek thinking、Claude thinking delta 等）
- **非流式响应**：`Message.ReasoningContent` 承载完整推理文本
- **多轮对话**：自动回传上一轮 `reasoning_content`（DeepSeek 要求多轮保持 thinking 上下文）
- **Channel 收缩**：推理内容与正文分流为独立 channel，通过 `ThinkSplit` 确保 Claude Code / Gemini CLI 等工具正确展示

## 客户端集成

不同客户端设置推理参数的方式：

### Claude Code
```bash
# 设置 reasoning effort
claude --model claude-sonnet-4-20250514 -p "..." 
# 或通过 API 调用时在请求体设置 thinking.budget_tokens
```

### Codex
```toml
[models.openai]
chat_model = "gpt-4o"
# reasoning 参数由 API 请求体控制
```

### Gemini CLI
```bash
gemini --model gemini-2.5-flash -p "..."
# thinking 参数由 generationConfig 控制
```

## 约束与注意事项

1. **Effort 优先于 Budget**：当同时提供时，后端优先使用 `Effort` 进行映射
2. **未设置 ≠ 禁用**：通过 `Specified` 字段区分；未指定时由后端默认行为决定
3. **Effort 别名**：`off` → `none`，`med` → `medium`
4. **无效 Effort**：无法识别的 effort 值会被归一化为 `none`
5. **ThinkSplit 始终启用**：无需配置，自动分流推理内容到独立 channel
