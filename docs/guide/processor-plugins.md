# Processor 插件使用指南

> 本文档介绍 Centag 中问题拆分（Question Splitter）、答案合成（Answer Synthesizer）和任务类型检测（TaskType Detector）三个 BusinessPlugin 的使用方法。
>
> 适用版本：2026-05-21 起，与 optimizer / translator / reviewer / summarizer 同属 BusinessPlugin 体系。

---

## 1. 插件列表

| 插件 | Implementation | BusinessType | 功能 |
|------|---------------|-------------|------|
| `question_splitter` | `business.question_splitter` | `question_split` | 问题拆分（rule / semantic / hybrid） |
| `answer_synthesizer` | `business.answer_synthesizer` | `answer_synthesize` | 答案合成（concat / template / llm / hybrid） |
| `tasktype_detector` | `business.tasktype_detector` | `tasktype_detect` | 任务类型检测（关键字 + 自定义规则） |

---

## 2. 自动调用（Handler 层）

Handler 已自动适配，**插件优先，不可用时回退到 `internal/processor` 内置实现**：

```
getQuestionSplitter()
  → 优先: BusinessPluginRegistry.GetByImplementation("business.question_splitter")
  → 回退: internal/processor.RuleBasedSplitter

synthesizeSubAnswers()
  → 优先: BusinessPluginRegistry.GetByImplementation("business.answer_synthesizer")
  → 回退: synthesizeSubAnswersBuiltin()

detectTaskTypeFromContent()
  → 优先: BusinessPluginRegistry.GetByImplementation("business.tasktype_detector")
  → 回退: detectTaskTypeFromKeywords()
```

**无需手动配置**，只要插件在 `server.go` 中注册即可生效。

---

## 3. Pipeline 模板中使用

### 3.1 问题拆分 + 答案合成节点

```json
{
  "nodes": [
    {
      "type": "processor",
      "implementation": "business.question_splitter",
      "config": {
        "strategy": "rule",
        "complexity_threshold": 0.5,
        "max_split_count": 5,
        "min_split_length": 10
      }
    },
    {
      "type": "processor",
      "implementation": "business.answer_synthesizer",
      "config": {
        "strategy": "template",
        "enable_citation": true,
        "preserve_order": true
      }
    }
  ]
}
```

### 3.2 任务类型检测节点

```json
{
  "nodes": [
    {
      "type": "processor",
      "implementation": "business.tasktype_detector",
      "config": {
        "default_type": "simple_chat",
        "custom_rules": [
          {
            "task_type": "math_problem",
            "keywords": ["微积分", "方程", "数学题"]
          }
        ]
      }
    }
  ]
}
```

---

## 4. 独立调用（Go 代码）

### 4.1 任务类型检测

```go
import tasktype_detector "centag/plugins/business/tasktype_detector"

// 无需 pipeline，直接检测
taskType := tasktype_detector.DetectTaskType("帮我写一段快速排序代码")
// taskType == "code_generation"
```

### 4.2 通过 BusinessPluginRegistry 调用

```go
import (
    "context"
    "centag/internal/pipeline"
    question_splitter "centag/plugins/business/question_splitter"
)

// 1. 创建注册表
nodeReg := pipeline.NewNodeRegistry()
bizReg := pipeline.NewBusinessPluginRegistry()
nodeReg.SetBusinessRegistry(bizReg)

// 2. 注册插件
_ = question_splitter.Register(nodeReg, bizReg)

// 3. 获取插件并执行
plugin, _ := bizReg.GetByImplementation("business.question_splitter")
output, _ := plugin.Execute(ctx, &pipeline.BusinessPluginInput{
    TaskType: "question_split",
    Content:  "什么是机器学习？它有什么应用？",
})
```

---

## 5. 配置参数

### 5.1 Question Splitter

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `strategy` | string | `rule` | 拆分策略：`rule` / `semantic` / `hybrid` |
| `complexity_threshold` | float64 | `0.5` | 复杂度阈值（0-1），超过才拆分 |
| `max_split_count` | int | `5` | 最大拆分子问题数 |
| `min_split_length` | int | `10` | 最小拆分长度（字符数） |

### 5.2 Answer Synthesizer

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `strategy` | string | `template` | 合成策略：`concat` / `template` / `llm` / `hybrid` |
| `enable_citation` | bool | `true` | 是否添加引用标记 |
| `preserve_order` | bool | `true` | 是否保持子答案原始顺序 |
| `template` | string | 内置模板 | 自定义合成模板（仅 template 策略有效） |

### 5.3 TaskType Detector

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `default_type` | string | `simple_chat` | 无匹配时的默认类型 |
| `custom_rules` | array | `[]` | 自定义规则列表（见 3.2 示例） |

**内置检测优先级**（从高到低）：
1. `translation` — 翻译、翻成英文、翻译成
2. `creative_writing` — 写故事、写诗、创意
3. `long_text` — 长文本、总结长文、文档
4. `code_generation` — 写代码、编程、函数
5. `data_analysis` — 分析数据、统计、图表
6. `complex_reasoning` — 推理、分析、证明
7. `simple_chat` — 默认回退

---

## 6. 调试与日志

启用 `debug` 级别日志可查看插件调用路径：

```
using question_splitter plugin  → 插件路径
fallback to builtin question splitter  → 回退路径
using answer_synthesizer plugin
using tasktype_detector plugin
```

---

## 7. 故障排查

| 现象 | 原因 | 解决 |
|------|------|------|
| 始终走内置实现，未使用插件 | 插件未注册 / Registry 未注入 | 检查 `server.go` 是否调用 `Register()` 并注入 Handler |
| 插件返回空结果 | 配置参数错误 / LLM 调用失败 | 检查日志，确认 `strategy` 值有效 |
| 任务类型检测不准确 | 规则优先级 / 自定义规则冲突 | 调整 `custom_rules` 顺序，更具体的规则放前面 |
| 性能下降 | 插件重复调用 | 确认启用了适配器缓存（已内置） |

---

## 8. 向后兼容

- **旧代码**：使用 `internal/processor` 直接调用的代码不受影响（通过 `aliases.go` 类型别名兼容）
- **Handler 层**：自动 fallback，插件不可用时静默回退
- **配置**：无需修改现有配置即可启用插件
- **API 变更**：
  - `processor.CreateProcessor()` 已删除，请使用 `pkg/processor.NewRuleBasedSplitter()` + `pkg/processor.NewSynthesizer()` 替代
  - `processor.NewQuestionProcessorWithLLM()` 已废弃，返回错误提示迁移到 BusinessPlugin 体系
- **示例更新**：`cmd/processor-verify/` 和 `../../archive/deprecated/examples/model-extension/` 已更新为使用 `pkg/processor` API

---

## 9. 从旧 API 迁移

### 9.1 问题拆分

```go
// 旧代码（已删除）
proc, _ := processor.CreateProcessor(config, nil)
result, _ := proc.SplitQuestion(ctx, question)

// 新代码
splitter, _ := processor.NewRuleBasedSplitter(&config.Split)
subQuestions, _ := splitter.Split(ctx, question)
```

### 9.2 答案合成

```go
// 旧代码（已删除）
proc, _ := processor.CreateProcessor(config, nil)
answer, _ := proc.SynthesizeAnswer(ctx, question, subAnswers)

// 新代码
synthesizer, _ := processor.NewSynthesizer(&config.Synthesis)
answer, _ := synthesizer.Synthesize(ctx, question, subAnswers)
```

### 9.3 完整示例

参见：
- `cmd/processor-verify/main.go` — 基础功能验证
- `../../archive/deprecated/examples/model-extension/main.go` — 模型扩展示例（已归档）
- `pkg/processor/processor_test.go` — 单元测试参考

---

## 相关文档

- [问题拆分与合成指南](../../archive/deprecated/docs/processor/Question-Processor-Guide.md) — 完整功能说明与旧版 API（已归档）
- [流水线插件标准](pipeline-plugin-standard.md) — BusinessPlugin 接口规范
- [插件与流水线执行计划](../exec-plans/active/plugin-pipeline.md) — 主台账与进度
- [迁移计划](../../archive/deprecated/docs/exec-plans/2026-05-21-processor-tasktype-plugin-migration.md) — 设计与决策记录（已归档）
