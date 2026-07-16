# 流水线变量参考

本文档是 Pipeline 模板变量的权威参考，覆盖各节点类型的内置变量、可绑定路径和使用规则。  
**代码真源**：`internal/pipeline/builtin_nodes.go`、`internal/pipeline/template_resolver.go`。

---

## 1. 变量语法

所有模板字段（`system_prompt`、`prompt_template`）均使用 Go `text/template` 语法：

```
{{.变量名}}         ← 访问字段
{{range .criteria}}-{{.}}\n{{end}}  ← 遍历数组
```

> **注意**：`{{input}}` 写法（无点）在 Go template 中是函数调用，**不是**字段访问，会导致渲染失败。请始终使用 `{{.input}}`。

---

## 2. 各节点类型可用变量

### 2.1 GeneratorNode（生成节点）

> 适用字段：`config.system_prompt`（`config.prompt_template` 由 generator 忽略，无效）

| 变量名 | 类型 | 说明 | 来源 |
|--------|------|------|------|
| `{{.question}}` | string | 用户发送的原始输入，全链路自动传递 | execCtx 变量 `input` |
| `{{.input}}` | string | 同 `question`（别名，当前节点主输入内容） | `input.Content` |
| `{{.timestamp}}` | string | 当前执行时间，RFC3339 格式 | 运行时注入 |
| `{{.{nodeID}_content}}` | string | 上游节点 `{nodeID}` 的输出文本（自动注入，无需绑定） | `UpstreamResults` |

**示例**：

```
你是专业的技术助手，请用中文回答用户的问题。用户问：{{.question}}
```

---

### 2.2 ProcessorNode（处理节点）

> 适用字段：`config.prompt_template`

| 变量名 | 类型 | 说明 | 来源 |
|--------|------|------|------|
| `{{.question}}` | string | 用户原始输入 | execCtx 变量 `input` |
| `{{.input}}` | string | 当前节点主输入内容（上游传入的待处理文本） | `input.Content` |
| `{{.target_lang}}` | string | 目标语言（仅在 `operation=translate` 时有意义） | `custom_config.target_lang` |
| `{{.metadata}}` | map | 上游节点的完整元数据对象 | `input.Metadata` |
| `{{.timestamp}}` | string | 当前执行时间，RFC3339 格式 | 运行时注入 |
| `{{.{nodeID}_content}}` | string | 上游节点 `{nodeID}` 的输出文本（自动注入，无需绑定） | `UpstreamResults` |

**示例**：

```
请将以下内容翻译成{{.target_lang}}：

{{.input}}

直接返回翻译结果，不要解释。
```

---

### 2.3 ReviewerNode（审核节点）

> 适用字段：`config.prompt_template`

| 变量名 | 类型 | 说明 | 来源 |
|--------|------|------|------|
| `{{.question}}` | string | 用户原始输入 | execCtx 变量 `input` |
| `{{.answer}}` | string | 待审核内容（上游传入的文本，通常是 generator 的输出） | `input.Content` |
| `{{.criteria}}` | []string | 审核维度列表，来自 `custom_config.criteria` | `ReviewerNode.Criteria` |
| `{{.timestamp}}` | string | 当前执行时间，RFC3339 格式 | 运行时注入 |
| `{{.{nodeID}_content}}` | string | 上游节点 `{nodeID}` 的输出文本（自动注入，无需绑定） | `UpstreamResults` |

**示例**（`01-audit-mode.json` 中 auditor 节点的 prompt_template）：

```
你是一名资深内容审核专家。请根据下列维度审核回答质量，并严格输出 JSON。

## 用户问题
{{.question}}

## 待审核回答
{{.generator_content}}

## 审核维度
{{range .criteria}}- {{.}}
{{end}}

## 输出要求
1. 仅输出 JSON 对象，不要添加 markdown 或解释
2. score 范围 0~1，建议保留两位小数
3. 当 score < 0.8 时，passed 必须为 false

{
  "passed": true/false,
  "score": 0.0,
  "feedback": "一句话说明",
  "suggestions": ["建议1", "建议2"]
}
```

---

## 3. 自定义变量绑定（template_vars）

对于内置变量无法覆盖的场景（如引用上游节点的评分、会话 ID 等），通过 `config.template_vars` 将数据路径绑定为自定义变量名。

### 3.1 路径语法

| 路径格式 | 说明 | 示例 |
|----------|------|------|
| `input.content` | 当前节点主输入内容 | `input.content` |
| `input.metadata.<key>` | 输入元数据字段 | `input.metadata.language` |
| `node.<id>.content` | 上游节点输出文本 | `node.generator.content` |
| `node.<id>.score` | 上游审核节点的评分（0~1） | `node.auditor.score` |
| `node.<id>.passed` | 上游审核节点是否通过 | `node.auditor.passed` |
| `node.<id>.feedback` | 上游审核节点的反馈文本 | `node.auditor.feedback` |
| `node.<id>.metadata.<key>` | 上游节点元数据字段 | `node.generator.metadata.tokens` |
| `context.timestamp` | 当前 RFC3339 时间戳 | `context.timestamp` |
| `context.user_id` | 当前用户 ID | `context.user_id` |
| `context.session_id` | 当前会话 ID | `context.session_id` |
| `context.pipeline_id` | 当前流水线 ID | `context.pipeline_id` |
| `literal:<值>` | 字面量常量 | `literal:zh-CN` |

### 3.2 配置示例

```json
{
  "config": {
    "template_vars": {
      "audit_score":    "node.auditor.score",
      "audit_feedback": "node.auditor.feedback",
      "session":        "context.session_id"
    },
    "prompt_template": "审核评分：{{.audit_score}}，反馈：{{.audit_feedback}}"
  }
}
```

### 3.3 优先级规则

当同一变量名在多处定义时，高优先级覆盖低优先级：

```
template_vars 显式绑定（最高）
    ↑ 覆盖
input.Metadata 自动展开（中）
    ↑ 覆盖
内置变量 builtinVars（最低）
```

---

## 4. 变量可用性速查表

| 变量名 | Generator | Processor | Reviewer | 备注 |
|--------|:---------:|:---------:|:--------:|------|
| `{{.question}}` | ✅ | ✅ | ✅ | 全链路内置，无需绑定 |
| `{{.input}}` | ✅ | ✅ | — | 当前节点主输入 |
| `{{.answer}}` | — | — | ✅ | reviewer 专用，同 input.Content |
| `{{.criteria}}` | — | — | ✅ | 来自 custom_config.criteria |
| `{{.timestamp}}` | ✅ | ✅ | ✅ | generator 需自动注入，processor/reviewer 内置 |
| `{{.target_lang}}` | — | ✅ | — | processor translate 专用 |
| `{{.metadata}}` | — | ✅ | — | 上游完整元数据对象 |
| `{{.{nodeID}_content}}` | ✅ | ✅ | ✅ | **自动注入，无需 template_vars 绑定** |
| `{{.{nodeID}_score}}` | 需绑定 | 需绑定 | 需绑定 | 路径：`node.<id>.score` |
| `{{.{nodeID}_passed}}` | 需绑定 | 需绑定 | 需绑定 | 路径：`node.<id>.passed` |
| `{{.{nodeID}_feedback}}` | 需绑定 | 需绑定 | 需绑定 | 路径：`node.<id>.feedback` |
| `{{.user_id}}` | 需绑定 | 需绑定 | 需绑定 | 路径：`context.user_id` |
| `{{.session_id}}` | 需绑定 | 需绑定 | 需绑定 | 路径：`context.session_id` |

---

## 5. 常见问题

**Q：为什么 `{{.question}}` 在 reviewer 的 prompt 里显示为空？**  
A：检查 `depends_on` 是否正确填写了上游节点 ID。`question` 来自 `execCtx` 全局变量，一般不为空；但如果 generator bypass 导致 `input.Content` 为空，`{{.answer}}` 会是空字符串。

**Q：`{{.generator_content}}` 和 `{{.answer}}` 有什么区别？**  
A：两者在"generator → reviewer"这条最常见链路中值相同，但语义不同：
- `{{.answer}}` = 当前节点的 `input.Content`（即 `depends_on` 中最后一个节点的输出）
- `{{.generator_content}}` = UpstreamResults 中 id 为 `generator` 的节点的输出（无论链路拓扑如何）  
多节点链路中两者可能不同，建议优先使用 `{{.{nodeID}_content}}` 以明确语义。

**Q：`template_vars` 绑定了但变量值没生效怎么排查？**  
A：查看后端日志中是否有 `template_vars 中存在解析失败的路径` 的 WARN 级别消息，其中会列出失败的路径和具体错误原因。

**Q：AggregatorNode 的 `prompt_template` 支持变量吗？**  
A：暂不支持（`strategy=summarize` 时，`PromptTemplate` 原样传给模型，不经过 `BuildTemplateData` 渲染）。如需动态内容请使用 Processor 节点。

---

*最后更新：2026-04-29 | 代码真源：`internal/pipeline/builtin_nodes.go`、`internal/pipeline/template_resolver.go`*
