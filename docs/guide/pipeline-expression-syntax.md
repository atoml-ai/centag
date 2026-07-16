# Pipeline Expression Syntax

本文档描述 Pipeline Inputs 表达式语法规范，用于在模板配置中表达复杂绑定逻辑。

## 概述

表达式语法允许在节点的 `inputs` 字段中引用数据源、指定默认值、标记必填字段，并支持严格失败模式。

## 语法规范

### 基本语法

```
{{expression}}
```

- 包裹在 `{{` 和 `}}` 中间
- 表达式内不允许有空格（可选外层）

### 支持的数据路径

| 路径类型 | 示例 | 说明 |
|----------|------|------|
| 原始内容 | `{{.content}}` | PipelineInput.Content |
| 消息列表 | `{{.messages}}` | PipelineInput.Messages 数组 |
| 元数据 | `{{.question}}` | PipelineInput.Metadata 中的字段 |
| 数组访问 | `{{.messages[0].content}}` | 访问 messages 第 0 条的 content |
| 上游结果 | `{{.node.generator.content}}` | 上游节点的输出内容 |
| 上下文 | `{{context.pipeline_id}}` | 执行上下文中的变量 |
| 字面量 | `{{literal:hello}}` | 固定字符串值 |

### 过滤器 (Filters)

过滤器附加在表达式末尾，使用 `|` 分隔，支持链式组合。

#### 1. default - 默认值

当字段缺失或为空时，返回指定的默认值。

```yaml
inputs:
  content: "{{.missing_field | default \"fallback\"}}"
  count: "{{.missing_count | default 100}}"
  flag: "{{.missing_flag | default true}}"
```

类型支持：
- 字符串: `"value"` 或 `value`
- 数字: `123`
- 布尔: `true` / `false`

#### 2. required - 必填字段

当字段缺失时，返回错误而非空值。

```yaml
inputs:
  user_id: "{{.user_id | required}}"
```

#### 3. strict - 严格模式

当字段缺失或值为 nil 时，中止整个流水线执行。

```yaml
inputs:
  answer: "{{.answer | strict}}"
```

### 过滤器组合

可以组合多个过滤器，按从左到右顺序执行：

```yaml
inputs:
  field: "{{.field | default \"fallback\" | required}}"
  critical: "{{.critical | default \"default\" | strict}}"
```

## 错误码

| 错误码 | 说明 |
|--------|------|
| EXPR_INVALID_EXPRESSION | 表达式格式错误 |
| EXPR_MISSING_FIELD | 必填字段缺失 |
| EXPR_INVALID_PATH | 数据路径无效 |
| EXPR_TYPE_ERROR | 类型转换错误 |
| EXPR_STRICT_FAILED | 严格模式失败 |

## 使用示例

### 示例 1: 基本字段绑定

```yaml
nodes:
  - id: process
    type: processor
    inputs:
      content: "{{.content}}"
      question: "{{.question}}"
```

### 示例 2: 数组元素访问

```yaml
nodes:
  - id: extract
    type: processor
    inputs:
      first_message: "{{.messages[0].content}}"
```

### 示例 3: 默认值回退

```yaml
nodes:
  - id: summary
    type: generator
    inputs:
      language: "{{.metadata.language | default \"en\"}}"
      max_length: "{{.metadata.max_length | default 500}}"
```

### 示例 4: 严格模式确保数据存在

```yaml
nodes:
  - id: verify
    type: reviewer
    inputs:
      answer: "{{.generator.answer | strict}}"
```

### 示例 5: 上游节点结果

```yaml
nodes:
  - id: aggregate
    type: aggregator
    inputs:
      draft1: "{{.node.draft1.content}}"
      draft2: "{{.node.draft2.content}}"
```

### 示例 6: 上下文变量

```yaml
nodes:
  - id: log
    type: processor
    inputs:
      session_id: "{{context.session_id}}"
      pipeline_id: "{{context.pipeline_id}}"
      timestamp: "{{context.timestamp}}"
```

### 示例 7: 字面量

```yaml
nodes:
  - id: translate
    type: processor
    inputs:
      target_language: "{{literal:zh-CN}}"
      template_version: "{{literal:v1}}"
```

## 在 Engine 中的集成

表达式解析器在 `PrepareNodeInput` 阶段被调用：

```go
// 使用 strict 模式：任何输入绑定失败都会导致整个流水线失败
result, err := ProcessNodeInputs(nodeInput, execCtx, config.inputs, true)

// 使用非 strict 模式：绑定失败会跳过该字段
result, err := ProcessNodeInputs(nodeInput, execCtx, config.inputs, false)
```

## 与 TemplateVars 的区别

| 特性 | TemplateVars | 表达式 |
|------|-------------|---------|
| 语法 | `input.content` | `{{.content}}` |
| 默认值 | 不支持 | `{{.field | default "value"}}` |
| 必填校验 | 不支持 | `{{.field | required}}` |
| 严格模式 | 不支持 | `{{.field | strict}}` |
| 数组访问 | 有限 | `{{.messages[0].content}}` |

推荐：
- 简单路径绑定使用 TemplateVars（性能更好）
- 复杂逻辑使用表达式（功能更全）