# 模式行为差异矩阵

> **位置说明**：本矩阵为**用户/实现对照**真源之一，与 `internal/proxy/pipeline_mode.go` 中的模板 ID 变更需同步更新。原位于 `docs/exec-plans/active/`，已迁入 `docs/guide/` 以减少执行计划目录冗余。

本文档描述 Centag 支持的**常用代理模式**在**输入预处理**、**节点执行顺序**、**输出包装**和**错误处理策略**四个维度的差异。

---

## 1. 模式总览

> **重要更新 (2026-05-14)**: 所有代理模式现已统一使用流水线模板执行路径 (pipeline-only)。每个请求会通过 `X-{Mode}-Mode-Path: pipeline-template` 响应头标识执行路径。

| 模式标识 | 快捷码 | 中文名 | 模式常量 | 流水线模板 | 执行路径 |
|---------|-------|--------|---------|------------|----------|
| smart-scheduling | #s | 智能调度 | `ModeSmartScheduling` | smart-scheduling | **pipeline-only** ✅ |
| direct-backend | #d | 直连后端（注入 system prompt） | `ModeDirectBackend` | direct-backend | **pipeline-only** ✅ |
| transparent-proxy | #t | 透明模式（不注入 system prompt） | `ModeTransparentProxy` | transparent-proxy | **pipeline-only** ✅ |
| transparent-fast | #tf | 透明模式（快，同 #t） | `ModeTransparentFast` | transparent-fast | **pipeline-only** ✅ |
| fixed-egress | #j | 跳板模式（固定出站） | `ModeFixedEgress` | fixed-egress | **pipeline-only** ✅ |
| model-matching | #m | 模型匹配 | `ModeModelMatching` | model-matching | **pipeline-only** ✅ |
| intent-classification | #c | 意图分类（已合并→router-mode） | `ModeIntentClassification` | router-mode | **pipeline-only** ✅（等同于#r） |
| audit-mode | #a | 审核模式 | `ModeAuditMode` | audit-mode | **pipeline-only** ✅ |
| optimize-mode | #o | 优化模式 | `ModeOptimizeMode` | optimize-mode | **pipeline-only** ✅ |
| fallback | #f | 降级模式（**已不再预置模板**；降级能力见 #d/#t/#j） | `ModeFallback` | — | 兼容保留快捷码 |
| pipeline | #p | 流水线模式 | `ModePipeline` | pipeline-mode | **pipeline-only** ✅ |
| aggregator | #ag | 聚合模式 | `ModeAggregator` | aggregator-mode | **pipeline-only** ✅ |
| router | #r | 路由模式 | `ModeRouter` | router-mode | **pipeline-only** ✅ |
|| translate | #l | 翻译模式 | `ModeTranslate` | translate-mode | **pipeline-only** ✅ |
|| mem0-memory | #mem0 | Mem0记忆存储 | `ModeMem0` | mem0-memory | **pipeline-only** ✅ |
|| rag-mode | #rag | RAG 知识库 | `ModeRAG` | rag-mode | **pipeline-only** ✅ |
|| security-mode | #sec | 安全审核防火墙 | `ModeSecurity` | security-mode | **pipeline-only** ✅ |
|| multilingual-support | #cs | 多语言客服 | `ModeMultilingualSupport` | multilingual-support | **pipeline-only** ✅ |
|| geo-routing-mode | #geo | 地理路由 | `ModeGeoRouting` | geo-routing-mode | **pipeline-only** ✅ |

---

## 2. 输入预处理差异

| 模式 | 前缀移除 | Header 提取 | Prompt 构建 | 特殊处理 |
|------|---------|-------------|-------------|----------|
| #s (智能调度) | 无 | 无 | 直接透传 | 根据关键词选择路径 |
| #d (直连后端) | 无 | 可选 `X-Backend-*` | 注入网关 system prompt | 覆盖客户端 system |
| #t / #tf (透明) | 无 | 无（不需要 Target-URL） | 不注入 system prompt | 保留客户端 messages |
| #j (跳板模式) | 无 | 可选 `X-Backend-ID` | 固定出站，不注入 system | 不做跨后端模型匹配 |
| #m (模型匹配) | 无 | 无 | 直接透传 | 路由决策前预处理 |
| #c (意图分类) | 无 | 无 | 直接透传 | 分类决策前预处理 |
| #a (审核) | 无 | `X-Auditor-Backend-ID` | 构建审核 Prompt | executor → auditor 两阶段 |
| #o (优化) | 无 | `X-Optimizer-Backend-ID` | 构建优化 Prompt | executor → optimizer 两阶段 |
| #f (降级) | 无 | `X-Primary-Backend-ID`, `X-Fallback-Backend-ID` | 直接透传 | 失败后尝试备用 |
| #p (流水线) | 无 | 可配置 | 可配置 | 多阶段串行 |
| #ag (聚合) | 无 | `X-Aggregator-Backend-ID`, `X-Generator-Backend-IDs` | 直接透传 | 多模型并行生成 |
| #r (路由) | 无 | `X-Router-Rules`, `X-Default-Backend-ID` | 直接透传 | 关键词路由决策 |
|| #l (翻译) | 无 | `X-Generator-Backend-ID`, `X-Translator-Backend-ID` | 直接透传 | 生成 → 翻译两阶段 |
|| #mem0 (Mem0记忆) | 无 | `X-Mem0-Backend-ID` | 直接透传 | 生成 → Mem0存储两阶段 |

---

## 3. 节点执行顺序差异

| 模式 | 节点数 | 执行顺序 | 并发能力 | 依赖关系 |
|------|--------|---------|----------|----------|
| #s | 1-3 | 分支选择 → 生成 | 支持分支并发 | 条件依赖 |
| #d | 1 | transparent_forward（fixed + system replace） | 无 | 无 |
| #t / #tf | 1 | transparent_forward（match_model + system passthrough） | 无 | 无 |
| #j | 1 | transparent_forward（route_policy=fixed，无 system 注入） | 无 | 固定默认后端 |
| #m | 1+N | 路由 → 条件分支生成 | 支持分支并发 | 依赖路由器 |
| #c | 1+N | 分类 → 条件分支处理 | 支持分支并发 | 依赖分类器 |
| #a | 2 | 生成 → 审核 | 串行 | executor → auditor |
| #o | 2 | 生成 → 优化 | 串行 | executor → optimizer |
| #f | 2 | 主生成 → [失败] 备用生成 | 条件触发 | 失败条件触发 |
| #p | N | 阶段 1 → 阶段 2 → ... → 阶段 N | 可配置 | 串行依赖 |
| #ag | N | 并行生成 1, 2, ... N → 聚合 | 支持并行 | 聚合器依赖所有生成器 |
| #r | 1+N | 路由决策 → 条件分支生成 | 支持分支并发 | 依赖路由器 |
|| #l | 2 | 生成 → 翻译 | 串行 | generator → translator |
|| #mem0 | 2 | 生成 → Mem0存储 | 串行 | generator → mem0_storage |

---

## 4. 输出包装差异

> **Pipeline-Only 路径标识**: 所有模式统一通过 `X-{Mode}-Mode-Path: pipeline-template` 响应头标识当前执行路径。这是 Phase 3-9 改造的结果，所有非默认模式现已强制走流水线模板执行。
>
> **流式行为 (2026-06-05 起)**：所有模式的"流式"列均支持客户端 `req.Stream=true/false`。
> 唯一决策来源是 `req.Stream`，与流水线全局配置 (`global_config.stream_mode` 字段已废弃) 无关；
> 引擎内部所有节点统一非流式执行，由顶层 `StreamAdapter` 按 `req.Stream` 决定是否分块。
> 即使最后节点是 processor（如 `optimize-mode`、`translate-mode`），也支持 `stream=true` 输出。
> 详见 `docs/exec-plans/active/2026-06-05-pipeline-stream-decoupling.md`。

| 模式 | 响应头 | Path 标识头 | 特殊字段 | 流式处理 |
|------|--------|-------------|----------|----------|
| #s | `X-Proxy-Mode: smart-scheduling` | `X-Smart-Scheduling-Mode-Path: pipeline-template` | 无 | `req.Stream` 驱动 |
| #d | `X-Proxy-Mode: direct-backend` | `X-Direct-Backend-Mode-Path: pipeline-template` | 无 | `req.Stream` 驱动 |
| #t / #tf | `X-Proxy-Mode: transparent-proxy` | `X-Transparent-Proxy-Mode-Path: pipeline-template` | 无 | `req.Stream` 驱动 |
| #j | `X-Proxy-Mode: fixed-egress` | `X-Fixed-Egress-Mode-Path: pipeline-template` | 无 | `req.Stream` 驱动 |
| #m | `X-Proxy-Mode: model-matching` | `X-Model-Matching-Mode-Path: pipeline-template` | `selected_model` | `req.Stream` 驱动 |
| #c | `X-Proxy-Mode: intent-classification` | `X-Intent-Classification-Mode-Path: pipeline-template` | `intent`, `confidence` | `req.Stream` 驱动 |
| #a | `X-Proxy-Mode: audit-mode` | `X-Audit-Mode-Path: pipeline-template` | `audit_feedback` | `req.Stream` 驱动 |
| #o | `X-Proxy-Mode: optimize-mode` | `X-Optimize-Mode-Path: pipeline-template` | `optimized_answer` | `req.Stream` 驱动（最后节点 processor） |
| #f | `X-Proxy-Mode: fallback` | `X-Fallback-Mode-Path: pipeline-template` | `fallback_duration_ms` | `req.Stream` 驱动 |
| #p | `X-Proxy-Mode: pipeline` | `X-Pipeline-Mode-Path: pipeline-template` | `stage_results` | `req.Stream` 驱动 |
| #ag | `X-Proxy-Mode: aggregator` | `X-Aggregator-Mode-Path: pipeline-template` | `aggregated_results` | `req.Stream` 驱动 |
| #r | `X-Proxy-Mode: router` | `X-Router-Mode-Path: pipeline-template` | `selected_route` | `req.Stream` 驱动 |
| #l | `X-Proxy-Mode: translate` | `X-Translate-Mode-Path: pipeline-template` | `translated_text` | `req.Stream` 驱动（最后节点 translator） |
| #mem0 | `X-Proxy-Mode: mem0-memory` | `X-Mem0-Mode-Path: pipeline-template` | `memory_stored` | `req.Stream` 驱动 |

---

## 5. 错误处理策略差异

| 模式 | Primary 失败策略 | 超时策略 | 错误传播 | Fallback 行为 |
|------|-----------------|----------|----------|--------------|
| #s | 降级到默认路径 | 默认路径兜底 | 错误透传 | 自动切换 |
| #d | 返回错误 | 超时返回错误 | 错误透传 | 无 |
| #t / #tf | 返回错误 | 超时返回错误 | 错误透传 | 无 |
| #j | 返回错误 | 超时返回错误 | 错误透传 | 无 |
| #m | 返回错误 | 默认路由兜底 | 错误透传 | 自动切换默认 |
| #c | 返回错误 | 默认处理器兜底 | 错误透传 | 自动切换默认 |
| #a | 跳过审核返回原结果 | 跳过审核返回原结果 | 温和失败 | bypass_on_error |
| #o | 跳过优化返回原结果 | `BypassOnTimeout` 控制 | 温和失败 | bypass_on_error |
| #f | 尝试备用后端 | 尝试备用后端 | 错误降级 | 自动切换 |
| #p | 阶段失败停止 | 超时停止 | 可配置 | 可配置 |
| #ag | 部分生成器失败继续 | 超时返回部分结果 | 温和失败 | 聚合器降级 |
| #r | 返回错误 | 默认路由兜底 | 错误透传 | 自动切换默认 |
| #l | 返回生成结果（跳过翻译） | 跳过翻译返回原结果 | 温和失败 | bypass_on_error |
| #mem0 | 返回生成结果（跳过存储） | 跳过存储返回原结果 | 温和失败 | bypass_on_error |

---

## 6. 快速参考表

### 6.1 模式选择决策树

```
请求是否需要多阶段处理?
├── 否 → 是否有特殊路由需求?
│     ├── 否 → #s (智能调度)
│     └── 是 → 需要路由到不同模型?
│           ├── 是 → 需要意图理解/关键词路由?
│           │     ├── 是 → #r (路由模式, #c已合并至#r)
│           │     └── 否 → #m (模型匹配)
│           └── 否 → #d (直接后端)
└── 是 → 是否需要聚合/fallback?
      ├── 多模型并行聚合 → #ag (聚合模式)
      ├── 降级容错 → #d/#t/#j（节点内 fallback + FallbackGroup；#f 已不再预置）
      └── 否 → 是否需要审核/优化/翻译/记忆?
            ├── 审核 → #a (审核模式)
            ├── 优化 → #o (优化模式)
            ├── 翻译 → #l (翻译模式)
            ├── Mem0记忆 → #mem0 (Mem0记忆存储)
            └── 通用多阶段 → #p (流水线模式)
```

### 6.2 响应头速查

| 模式 | 必须 Header | 可选 Header |
|------|-------------|-------------|
| #d | X-Backend-Name | X-Backend-ID |
| #t / #tf | （同直连路径，可选后端头） | 无 Target-URL |
| #j | （可选）X-Backend-ID | - |
| #f | X-Primary-Backend-ID | X-Fallback-Backend-ID |
| #a | X-Auditor-Backend-ID | X-Audit-Threshold |
| #o | X-Optimizer-Backend-ID | X-BypassOnTimeout |

---

- 文档版本: 1.6.0  
- 最后更新: 2026-07-14  
 
- 更新内容: router-mode 路由策略新增 `llm_classify`（LLM 意图分类）支持，前端下拉切换；详见第 7 节。  

## 7. router-mode 路由策略（`routing_strategy`）

> 自 2026-06-02 起，`router-mode` 的 `classifier` 节点支持多种路由策略，前端下拉切换。  
> 各分支 **backend/model** 可在 Web「分配模型」面板统一配置（见 `docs/guide/proxy-modes.md` §11 路由模型分配），不必逐节点进画布。

| 策略值 | 名称 | 匹配方式 | 延迟成本 | 适用场景 |
|--------|------|----------|----------|----------|
| `keyword_contains`（默认） | 关键词包含 | 子串包含（忽略大小写） | 零成本 | 关键词覆盖充分的稳定场景 |
| `keyword_prefix` | 关键词前缀 | 前缀匹配 | 零成本 | 命令式输入（如 `/help`） |
| `ordered` | 有序规则 | 按 `route_rules` 顺序匹配 | 零成本 | 优先级明确的复杂规则 |
| `regex_only` | 正则匹配 | 按 `route_rules` 正则匹配 | 零成本 | 复杂模式（电话号码、URL 等） |
| `llm_classify` | LLM 意图分类 | 调用 LLM 语义分类 | +1 次 LLM 调用（500ms-2s） | 表达多样、关键词覆盖不全的场景 |

### 7.1 `llm_classify` 关键说明

- **额外调用**：每次路由会发起一次 LLM 调用（建议使用轻量模型如 `glm-4-flash`）
- **`routes` key 含义不同**：在 `llm_classify` 模式下，`routes` 的 key 是**类别名**（如 `code`），而非常规关键词（如 `python`）
- **配置要求**：`Validate()` 强制要求 `backend`、`model` 非空且 `routes` 非空
- **失败降级**：LLM 调用失败（网络超时、后端不可用、返回未定义类别等）时自动 fallback 到 `default_route`
- **可定制 Prompt**：`custom_config.classify_prompt` 可覆盖内置默认 Prompt（变量：`{{.input}}`）
- **响应清洗**：自动去除 markdown 代码块包裹、引号、前缀（如 "类别：code"），统一小写
- **追溯**：`NodeOutput.Metadata["llm_raw_response"]` 保留 LLM 原始响应

### 7.2 配置示例

#### 关键词分类（默认，零成本）

```yaml
- id: classifier
  type: router
  backend: bigmodel
  model: glm-4-flash
  config:
    custom_config:
      routing_strategy: keyword_contains
      default_route: chat-generator
      routes:
        python: code-generator
        javascript: code-generator
        翻译: translate-gen
        摘要: summary-gen
```

#### LLM 分类（语义准确，有延迟成本）

```yaml
- id: classifier
  type: router
  backend: bigmodel
  model: glm-4-flash
  config:
    custom_config:
      routing_strategy: llm_classify
      default_route: chat-generator
      # 可选：自定义分类 Prompt，留空使用内置默认
      # classify_prompt: "判断用户意图，只返回类别名（code/translate/summary/chat）\n输入：{{.input}}\n类别："
      routes:
        code: code-generator      # 注意：key 是类别名
        translate: translate-gen
        summary: summary-gen
        chat: chat-generator
```

### 7.3 决策建议

- 关键词覆盖充分、延迟敏感 → `keyword_contains`（默认）
- 用户表达多样、关键词维护成本高 → `llm_classify`
