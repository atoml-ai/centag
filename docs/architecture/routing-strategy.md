# LLM Proxy 模型调度策略 - 详细技术实施方案

## 一、需求确认

基于需求讨论，确认以下设计原则：

| 序号 | 需求 | 确认方案 |
|------|------|----------|
| 1 | 模型兼容性判断 | 提供多种策略，支持模型名称+参数量双维度匹配 |
| 2 | 缓存键生成 | 使用后端实际使用的模型名作为缓存键 |
| 3 | 模型转换通知 | 通过扩展字段返回（不破坏标准协议） |
| 4 | 模型能力评估 | 先手动配置，后续支持自动探测 |
| 5 | 多模型请求 | 暂不支持，按单模型处理 |
| 6 | 监控和告警 | 详细记录匹配决策过程 |

---

## 二、架构设计

### 2.1 整体架构图

```
用户请求
    ↓
[请求解析] - 提取 model, parameters
    ↓
[模型调度器] ←── 后端配置（模型映射、兼容性策略）
    ↓
    ├─→ [精确匹配] → 找到 exact_match? → 选择后端
    │                                        ↓
    │                                    [记录决策]
    │
    ├─→ [兼容性计算] ←── 模型兼容性策略（名称+参数量）
    │       ↓
    │   [评分排序]
    │       ↓
    │   [阈值过滤] ←── 严格度配置（weight 0-100）
    │       ↓
    │   [选择最佳后端]
    │
    └─→ [回退处理] → 无匹配后端 → 返回错误/降级处理
        ↓
[请求转换]（如有模型转换）
    ↓
[缓存查询] ←── 使用实际模型名生成缓存键
    ↓
    ├─→ [命中] → 返回缓存 + 扩展字段
    └─→ [未命中] → 调用后端
                    ↓
                [记录指标]
                    ↓
                [返回响应 + 扩展字段]
```

### 2.2 核心模块划分

| 模块 | 职责 | 文件位置 |
|------|------|----------|
| **ModelMatcher** | 模型兼容性计算、评分 | `internal/backend/model_matcher.go` |
| **BackendSelector** | 后端选择决策引擎 | `internal/backend/selector.go` |
| **CacheKeyGenerator** | 缓存键生成（模型感知） | `internal/cache/key_generator.go` |
| **ResponseDecorator** | 响应扩展字段装饰 | `internal/middleware/response_decorator.go` |
| **MetricsCollector** | 决策指标收集 | `internal/backend/metrics.go` |

---

## 三、详细设计

### 3.1 模型兼容性策略

#### 3.1.1 策略类型

```go
// 模型匹配策略类型
type ModelMatchStrategy string

const (
    // StrategyExact 精确匹配：模型名完全相同
    StrategyExact ModelMatchStrategy = "exact"

    // StrategyFamily 家族匹配：同系列模型（如 gpt-4, gpt-4-turbo）
    StrategyFamily ModelMatchStrategy = "family"

    // StrategyCapacity 参数量匹配：基于模型参数量匹配
    StrategyCapacity ModelMatchStrategy = "capacity"

    // StrategyHybrid 混合匹配：名称+参数量综合评分
    StrategyHybrid ModelMatchStrategy = "hybrid"

    // StrategyCustom 自定义规则：管理员配置的规则引擎
    StrategyCustom ModelMatchStrategy = "custom"
)
```

#### 3.1.2 评分算法

**混合评分公式**：
```
TotalScore = NameSimilarity × W_name + CapacityMatch × W_capacity

其中：
- NameSimilarity: 模型名称相似度（0-1）
- CapacityMatch: 参数量匹配度（0-1）
- W_name + W_capacity = 1
```

**示例配置**：
```yaml
model_matching:
  strategy: "hybrid"
  hybrid_weights:
    name_similarity: 0.6
    capacity_match: 0.4
  capacity_tolerance: 0.2  # 参数量允许偏差20%
```

#### 3.1.3 模型名称解析

定义模型名称的规范化解析规则：

```go
type ModelInfo struct {
    Provider    string  // gpt, claude, qwen, deepseek 等
    Family      string  // 4, 3.5, 2.5 等
    Variant     string  // turbo, pro, mini 等
    Size        string  // 7b, 13b, 70b 等
    Precision   string  // fp16, fp8, int4 等
    Version     string  // v1, v2 等
}
```

**解析示例**：
| 模型名 | Provider | Family | Variant | Size | Precision |
|--------|----------|--------|---------|------|-----------|
| `gpt-4-turbo` | gpt | 4 | turbo | - | - |
| `qwen2.5:7b-fp8` | qwen | 2.5 | - | 7b | fp8 |
| `deepseek-chat` | deepseek | chat | - | - | - |

#### 3.1.4 参数量映射表

```yaml
# 模型参数量基准表（单位：B - billion parameters）
model_capacity_reference:
  gpt:
    "4": 1.8
    "4-turbo": 1.8
    "3.5-turbo": 175  # 175B (实际175B)
  qwen:
    "2.5": 7.0
    "2": 7.0
  deepseek:
    "r1": 671
    "v3": 685
  claude:
    "3-opus": 400
    "3-sonnet": 200
    "3-haiku": 20
```

### 3.2 后端配置扩展

#### 3.2.1 BackendConfig 扩展

```go
type BackendConfig struct {
    // ... 原有字段

    // 模型支持列表
    SupportedModels []ModelMapping `json:"supported_models"`

    // 模型能力限制
    Capabilities ModelCapabilities `json:"capabilities"`

    // 优先级（默认使用 weight）
    Priority int `json:"priority"`
}

// ModelMapping 模型映射配置
type ModelMapping struct {
    // 用户请求的模型名（精确匹配或别名）
    RequestedModel string `json:"requested_model"`

    // 后端实际使用的模型名
    ActualModel string `json:"actual_model"`

    // 兼容性评分（0-1），用于自定义策略
    CompatibilityScore float64 `json:"compatibility_score"`

    // 是否为精确匹配
    IsExact bool `json:"is_exact"`
}

// ModelCapabilities 模型能力
type ModelCapabilities struct {
    // 最大上下文长度（tokens）
    MaxContextTokens int `json:"max_context_tokens"`

    // 支持的功能
    Features []string `json:"features"` // function_calling, streaming, json_mode 等

    // 是否支持图像输入
    SupportsImages bool `json:"supports_images"`

    // 是否支持工具调用
    SupportsTools bool `json:"supports_tools"`
}
```

#### 3.2.2 配置示例

```yaml
backends:
  - id: ollama-Ollama
    name: Ollama
    type: ollama
    base_url: http://localhost:21434
    weight: 100
    enabled: true
    priority: 1

    # 模型映射配置
    supported_models:
      # 精确匹配
      - requested_model: "qwen2.5:7b"
        actual_model: "qwen2.5:7b"
        is_exact: true

      # GPT-4 映射到 Qwen
      - requested_model: "gpt-4"
        actual_model: "qwen2.5:7b"
        is_exact: false
        compatibility_score: 0.85

      - requested_model: "gpt-4-turbo"
        actual_model: "qwen2.5:7b"
        is_exact: false
        compatibility_score: 0.80

      # GPT-3.5 映射到 Qwen 小模型
      - requested_model: "gpt-3.5-turbo"
        actual_model: "qwen2:1.5b"
        is_exact: false
        compatibility_score: 0.75

    # 模型能力
    capabilities:
      max_context_tokens: 32768
      features:
        - streaming
        - function_calling
      supports_images: false
      supports_tools: true

  - id: ppinfra-deepseek
    name: PPInfra DeepSeek
    type: openai
    base_url: https://api.ppio.com/openai
    weight: 50
    enabled: true
    priority: 2

    supported_models:
      - requested_model: "deepseek-chat"
        actual_model: "deepseek-chat"
        is_exact: true

      - requested_model: "gpt-4"
        actual_model: "deepseek-chat"
        is_exact: false
        compatibility_score: 0.90

    capabilities:
      max_context_tokens: 128000
      features:
        - streaming
        - json_mode
        - function_calling
      supports_images: false
      supports_tools: true
```

### 3.3 缓存键生成策略

#### 3.3.1 新的缓存键结构

```go
// CacheKeyOptions 缓存键生成选项
type CacheKeyOptions struct {
    // 使用实际模型名（而非请求模型名）
    UseActualModel bool

    // 参数容忍度
    TemperatureTolerance float64
    MaxTokensTolerance   int

    // 是否包含模型能力限制
    IncludeConstraints bool
}

// 生成缓存键
func GenerateCacheKey(req *LLMRequest, actualModel string, opts *CacheKeyOptions) (string, error) {
    // 使用实际模型名
    model := actualModel

    // 对温度参数进行归一化处理
    temperature := normalizeTemperature(req.Temperature, opts.TemperatureTolerance)

    // 对 max_tokens 进行分段处理
    maxTokens := discretizeMaxTokens(req.MaxTokens, opts.MaxTokensTolerance)

    // 组合生成键
    key := fmt.Sprintf("%s:%s:%d:%v",
        model,
        hashMessages(req.Messages),
        temperature,
        maxTokens,
    )

    return key, nil
}
```

#### 3.3.2 参数归一化策略

| 参数 | 策略 | 示例 |
|------|------|------|
| temperature | 容忍度范围 | 配置 tolerance=0.1，则 0.7 和 0.75 视为相同 |
| max_tokens | 分段 | [0, 100], [101, 500], [501, 2000], [2001, ∞) |
| top_p | 容忍度 | 同 temperature |
| presence_penalty | 容忍度 | 同 temperature |

#### 3.3.3 配置项

```yaml
cache:
  enabled: true

  # 模型感知缓存配置
  model_aware:
    # 缓存键使用实际模型名
    use_actual_model: true

    # 参数容忍度配置
    parameter_tolerance:
      temperature: 0.1      # ±0.1 视为相同
      top_p: 0.05
      max_tokens_bucket: 100  # 分段粒度

    # 模型族缓存共享
    share_within_family: false  # 同族模型不共享缓存
```

### 3.4 模型转换通知机制

#### 3.4.1 扩展字段设计

```go
// ResponseDecoration 响应装饰器
type ResponseDecoration struct {
    // 原始请求模型
    RequestedModel string `json:"requested_model"`

    // 实际使用模型
    ActualModel string `json:"actual_model"`

    // 模型是否转换
    ModelConverted bool `json:"model_converted"`

    // 后端ID
    BackendID string `json:"backend_id"`

    // 后端名称
    BackendName string `json:"backend_name"`

    // 兼容性评分
    CompatibilityScore float64 `json:"compatibility_score"`

    // 匹配策略
    MatchStrategy string `json:"match_strategy"`
}
```

#### 3.4.2 响应格式

**方式1：通过响应头返回（推荐）**
```
HTTP/1.1 200 OK
Content-Type: application/json
X-Proxy-Backend: ollama-Ollama
X-Proxy-Requested-Model: gpt-4
X-Proxy-Actual-Model: qwen2.5:7b
X-Proxy-Model-Converted: true
X-Proxy-Compatibility-Score: 0.85
```

**方式2：响应体扩展字段**
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-4",  // 保留原始模型名
  "choices": [...],
  "usage": {...},
  "_proxy_info": {
    "requested_model": "gpt-4",
    "actual_model": "qwen2.5:7b",
    "backend_id": "ollama-Ollama",
    "model_converted": true,
    "compatibility_score": 0.85
  }
}
```

**推荐组合使用**：响应头用于快速查看，响应体用于详细信息记录。

#### 3.4.3 配置选项

```yaml
proxy:
  # 响应装饰配置
  response_decoration:
    # 是否在响应头中添加代理信息
    add_headers: true

    # 是否在响应体中添加扩展字段
    add_body_fields: true

    # 字段前缀（避免冲突）
    field_prefix: "_proxy"
```

### 3.5 严格度控制逻辑

#### 3.5.1 Weight 到严格度的映射

| Weight | 模式 | 行为描述 |
|--------|------|----------|
| 0 | 严格模式 | 仅精确匹配，不允许模型转换 |
| 1-30 | 保守模式 | 优先精确匹配，仅在完全无匹配时转换 |
| 31-70 | 平衡模式 | 允许兼容性 > 0.8 的模型转换 |
| 71-90 | 宽松模式 | 允许兼容性 > 0.6 的模型转换 |
| 91-100 | 极度宽松 | 允许兼容性 > 0.4 的模型转换 |

#### 3.5.2 选择算法

```go
type SelectionCriteria struct {
    Strictness      int     // 0-100
    MinCompatibility float64  // 根据严格度计算的最小兼容性
    AllowConversion bool     // 是否允许模型转换
    PreferExact    bool     // 是否优先精确匹配
}

func (s *BackendSelector) SelectBackend(reqModel string, criteria *SelectionCriteria) (*BackendSelection, error) {
    // Step 1: 查找精确匹配
    exactMatches := s.findExactMatches(reqModel)
    if len(exactMatches) > 0 && (criteria.Strictness == 0 || criteria.PreferExact) {
        return s.selectBestByWeight(exactMatches), nil
    }

    // Step 2: 如果不允许转换，返回错误
    if !criteria.AllowConversion {
        return nil, fmt.Errorf("no backend supports model: %s (strict mode)", reqModel)
    }

    // Step 3: 查找兼容模型
    candidates := s.findAllCompatibleModels(reqModel, criteria.MinCompatibility)

    // Step 4: 过滤低于阈值的候选
    candidates = s.filterByMinScore(candidates, criteria.MinCompatibility)

    // Step 5: 选择最佳候选
    if len(candidates) == 0 {
        return nil, fmt.Errorf("no compatible backend found for model: %s", reqModel)
    }

    selected := s.selectBestCandidate(candidates)

    // Step 6: 记录决策
    s.recordDecision(reqModel, selected, criteria)

    return selected, nil
}
```

### 3.6 监控指标设计

#### 3.6.1 决策指标

```go
// BackendSelectionMetrics 后端选择指标
type BackendSelectionMetrics struct {
    // 基础指标
    TotalRequests        int64 `json:"total_requests"`
    ExactMatches         int64 `json:"exact_matches"`
    FamilyMatches        int64 `json:"family_matches"`
    CapacityMatches      int64 `json:"capacity_matches"`
    ConvertedRequests    int64 `json:"converted_requests"`

    // 失败指标
    NoMatchRequests      int64 `json:"no_match_requests"`
    CapacityExceeded     int64 `json:"capacity_exceeded"`

    // 性能指标
    SelectionLatencyMs   int64 `json:"selection_latency_ms"`

    // 转换率
    ConversionRate      float64 `json:"conversion_rate"`

    // 按模型统计
    ByModel             map[string]ModelMetrics `json:"by_model"`

    // 按后端统计
    ByBackend           map[string]BackendMetrics `json:"by_backend"`
}

type ModelMetrics struct {
    RequestCount      int     `json:"request_count"`
    ExactMatchCount   int     `json:"exact_match_count"`
    ConvertedCount    int     `json:"converted_count"`
    AvgCompatibility  float64 `json:"avg_compatibility"`
}

type BackendMetrics struct {
    SelectedCount     int     `json:"selected_count"`
    SelectedAsExact   int     `json:"selected_as_exact"`
    SelectedAsFallback int    `json:"selected_as_fallback"`
    AvgCompatibility  float64 `json:"avg_compatibility"`
}
```

#### 3.6.2 Prometheus 指标

```go
var (
    // 后端选择指标
    backendSelectionTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "llm_proxy_backend_selection_total",
            Help: "Total number of backend selections",
        },
        []string{"requested_model", "actual_model", "backend", "strategy"},
    )

    backendSelectionLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "llm_proxy_backend_selection_duration_seconds",
            Help:    "Backend selection latency",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
        },
        []string{"strategy"},
    )

    modelCompatibilityScore = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "llm_proxy_model_compatibility_score",
            Help: "Model compatibility score",
        },
        []string{"requested_model", "actual_model", "backend"},
    )
)
```

#### 3.6.3 日志结构

```json
{
  "level": "info",
  "timestamp": "2024-01-01T12:00:00Z",
  "event": "backend_selection",
  "request_id": "req-abc123",

  "requested_model": "gpt-4",
  "actual_model": "qwen2.5:7b",
  "backend": {
    "id": "ollama-Ollama",
    "name": "Ollama",
    "type": "ollama"
  },

  "selection": {
    "strategy": "hybrid",
    "strictness": 70,
    "exact_match": false,
    "compatibility_score": 0.85,
    "name_similarity": 0.6,
    "capacity_match": 0.9,
    "decision_time_ms": 2.3
  },

  "criteria": {
    "min_compatibility": 0.6,
    "allow_conversion": true
  }
}
```

#### 3.6.4 告警规则

```yaml
groups:
  - name: llm_proxy_backend_selection
    rules:
      # 模型转换率过高告警
      - alert: HighModelConversionRate
        expr: |
          rate(llm_proxy_backend_selection_total{strategy!="exact"}[5m]) /
          rate(llm_proxy_backend_selection_total[5m]) > 0.5
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "模型转换率过高: {{ $value }}"
          description: "最近5分钟内超过50%的请求发生了模型转换"

      # 无匹配请求过多
      - alert: HighNoMatchRate
        expr: |
          rate(llm_proxy_backend_selection_total{strategy="no_match"}[5m]) > 10
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "无匹配请求过多"
          description: "最近5分钟内无匹配请求超过10次/分钟"

      # 选择延迟过高
      - alert: SlowBackendSelection
        expr: |
          histogram_quantile(0.99,
            rate(llm_proxy_backend_selection_duration_seconds_bucket[5m])
          ) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "后端选择延迟过高: {{ $value }}s"
```

---

## 四、实施计划

### 4.1 Phase 1: 基础架构（预计5天）

#### Day 1: 数据结构设计
- [ ] 扩展 `BackendConfig` 结构
- [ ] 定义 `ModelInfo`、`ModelMapping`、`ModelCapabilities`
- [ ] 更新配置加载逻辑
- [ ] 编写单元测试

#### Day 2-3: 模型匹配器
- [ ] 实现 `ModelMatcher` 核心结构
- [ ] 实现模型名称解析
- [ ] 实现各匹配策略（exact, family, capacity, hybrid）
- [ ] 编写单元测试

#### Day 4: 后端选择器
- [ ] 实现 `BackendSelector`
- [ ] 实现严格度到选择标准的转换
- [ ] 实现候选评分和排序算法
- [ ] 编写集成测试

#### Day 5: 配置和文档
- [ ] 更新 `config.yaml` 示例
- [ ] 编写配置说明文档
- [ ] 准备测试数据

### 4.2 Phase 2: 缓存增强（预计3天）

#### Day 1: 缓存键生成器
- [ ] 实现 `CacheKeyGenerator`
- [ ] 支持参数归一化
- [ ] 使用实际模型名生成键
- [ ] 编写单元测试

#### Day 2: 缓存查询逻辑
- [ ] 修改 `HandleOpenAIRequest` 中的缓存查询
- [ ] 传入实际模型名
- [ ] 添加缓存命中日志

#### Day 3: 配置和测试
- [ ] 添加缓存配置项
- [ ] 编写缓存测试用例
- [ ] 性能测试

### 4.3 Phase 3: 响应装饰（预计2天）

#### Day 1: 响应装饰器
- [ ] 实现 `ResponseDecorator`
- [ ] 支持响应头扩展
- [ ] 支持响应体扩展
- [ ] 编写单元测试

#### Day 2: 集成和测试
- [ ] 在代理流程中集成装饰器
- [ ] 测试不同协议的兼容性
- [ ] 编写使用示例

### 4.4 Phase 4: 监控和指标（预计2天）

#### Day 1: 指标收集
- [ ] 实现指标收集器
- [ ] 集成 Prometheus
- [ ] 记录决策日志

#### Day 2: 告警和可视化
- [ ] 定义告警规则
- [ ] 创建 Grafana 仪表盘
- [ ] 编写监控文档

### 4.5 Phase 5: 测试和优化（预计3天）

#### Day 1: 单元测试
- [ ] 补充所有模块的单元测试
- [ ] 目标覆盖率 80%+

#### Day 2: 集成测试
- [ ] 端到端测试
- [ ] 压力测试
- [ ] 边界条件测试

#### Day 3: 优化和文档
- [ ] 性能优化
- [ ] 完善文档
- [ ] 准备发布

---

## 五、代码结构

```
internal/
├── backend/
│   ├── backend.go              # 现有文件，扩展配置结构
│   ├── model_matcher.go        # 新增：模型匹配器
│   ├── selector.go             # 新增：后端选择器
│   ├── model_info.go           # 新增：模型信息解析
│   ├── metrics.go              # 新增：指标收集
│   └── strategies.go           # 新增：匹配策略实现
│
├── middleware/
│   ├── proxy.go                # 现有文件，修改后端选择逻辑
│   ├── response_decorator.go   # 新增：响应装饰器
│   └── selection_logger.go     # 新增：选择日志记录
│
├── cache/
│   ├── cache.go                # 现有文件，保持不变
│   ├── key_generator.go        # 新增：缓存键生成器
│   └── normalizer.go           # 新增：参数归一化
│
└── config/
    ├── config.go               # 现有文件，添加新配置项
    └── model_matching.go       # 新增：模型匹配配置

docs/
├── MODEL_SCHEDULING_DESIGN.md      # 本文档
├── MODEL_SCHEDULING_GUIDE.md       # 使用指南
└── MODEL_SCHEDULING_API.md         # API文档

test/
├── model_matcher_test.go
├── selector_test.go
├── cache_key_generator_test.go
└── integration_test.go
```

---

## 六、配置示例

### 6.1 完整配置示例

```yaml
# LLM Proxy 配置
server:
  port: 20060
  host: 0.0.0.0
  mode: debug

# 模型匹配配置
model_matching:
  # 全局匹配策略
  strategy: "hybrid"

  # 混合策略权重
  hybrid_weights:
    name_similarity: 0.6
    capacity_match: 0.4

  # 参数量容忍度
  capacity_tolerance: 0.2

  # 严格度默认值（对应 weight）
  default_strictness: 70

# 后端配置
backends:
  - id: ollama-Ollama
    name: Ollama
    type: ollama
    base_url: http://localhost:21434
    api_key: ""
    enabled: true
    weight: 70        # 对应严格度
    priority: 1
    timeout: 60
    max_retries: 3

    supported_models:
      - requested_model: "qwen2.5:7b"
        actual_model: "qwen2.5:7b"
        is_exact: true

      - requested_model: "gpt-4"
        actual_model: "qwen2.5:7b"
        is_exact: false
        compatibility_score: 0.85

      - requested_model: "gpt-3.5-turbo"
        actual_model: "qwen2:1.5b"
        is_exact: false
        compatibility_score: 0.75

    capabilities:
      max_context_tokens: 32768
      features:
        - streaming
        - function_calling
      supports_images: false
      supports_tools: true

# 缓存配置
cache:
  enabled: true

  model_aware:
    use_actual_model: true
    parameter_tolerance:
      temperature: 0.1
      top_p: 0.05
      max_tokens_bucket: 100
    share_within_family: false

# 代理配置
proxy:
  timeout: 60
  max_retries: 3

  response_decoration:
    add_headers: true
    add_body_fields: true
    field_prefix: "_proxy"

# 监控配置
monitoring:
  prometheus:
    enabled: true
    port: 9090
  alerts:
    high_conversion_rate:
      enabled: true
      threshold: 0.5
    slow_selection:
      enabled: true
      threshold_ms: 100
```

---

## 七、向后兼容性

### 7.1 兼容性保证

1. **保留原有接口**
   - `SelectDefaultBackend()` 保持不变
   - `SelectBackend(backendType)` 保持不变

2. **新增接口**
   - `SelectBackendByModel(requestedModel, strictness)` 新增

3. **配置兼容**
   - 未配置 `supported_models` 时，使用原有逻辑
   - 未配置 `model_matching` 时，使用默认值

4. **渐进式迁移**
   - 通过环境变量控制新功能启用
   - 支持灰度发布

### 7.2 迁移路径

| 阶段 | 操作 | 预期影响 |
|------|------|----------|
| 阶段0 | 部署新版本，功能关闭 | 无影响 |
| 阶段1 | 配置少量模型映射 | 部分请求使用新逻辑 |
| 阶段2 | 扩充模型映射配置 | 大部分请求使用新逻辑 |
| 阶段3 | 完全切换到新逻辑 | 废弃旧逻辑 |

---

## 八、测试计划

### 8.1 单元测试覆盖

| 模块 | 测试场景 | 预期覆盖率 |
|------|----------|-----------|
| ModelMatcher | 模型解析、各策略评分 | 90% |
| BackendSelector | 选择逻辑、回退处理 | 85% |
| CacheKeyGenerator | 键生成、参数归一化 | 90% |
| ResponseDecorator | 头部/体装饰 | 95% |

### 8.2 集成测试场景

1. **精确匹配场景**
   - 用户请求 `gpt-4` → 后端支持 `gpt-4`
   - 验证：精确命中，无转换

2. **模型转换场景**
   - 用户请求 `gpt-4` → 后端映射到 `qwen2.5:7b`
   - 验证：转换成功，扩展字段正确

3. **无匹配场景**
   - 用户请求 `claude-3-opus` → 无后端支持
   - 验证：返回错误，记录指标

4. **缓存命中场景**
   - 相同请求二次调用
   - 验证：使用实际模型名命中缓存

5. **不同严格度场景**
   - 测试 weight = 0, 50, 100 的行为差异

---

## 九、风险评估和缓解

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| 模型匹配算法不准确 | 中 | 高 | 支持手动配置，可快速调整 |
| 性能下降 | 低 | 中 | 使用缓存，优化算法 |
| 破坏现有功能 | 低 | 高 | 保持向后兼容，灰度发布 |
| 配置复杂度增加 | 高 | 中 | 提供配置工具和模板 |
| 缓存命中率下降 | 中 | 低 | 监控指标，可调整策略 |

---

## 十、总结

本方案提供了完整的模型调度策略技术实施路线：

**核心特性**:
- ✅ 多种模型匹配策略（精确、家族、参数量、混合）
- ✅ 灵活的严格度控制（0-100）
- ✅ 模型感知的缓存机制
- ✅ 不破坏协议的转换通知
- ✅ 完善的监控和指标

**实施优先级**:
1. **P0**: 模型映射配置 + 精确匹配（Phase 1 Day 1-2）
2. **P1**: 兼容性评分 + 后端选择器（Phase 1 Day 3-4）
3. **P2**: 缓存键生成（Phase 2）
4. **P3**: 响应装饰器（Phase 3）
5. **P4**: 监控指标（Phase 4）

**预计完成时间**: 10-16 个工作日

---

## 附录

### A. 术语表

| 术语 | 说明 |
|------|------|
| requested_model | 用户请求中指定的模型名 |
| actual_model | 后端实际调用的模型名 |
| exact_match | requested_model == actual_model |
| model_conversion | requested_model != actual_model |
| compatibility_score | 模型兼容性评分（0-1） |
| strictness | 严格度（0-100），控制模型转换的宽松程度 |

### B. 参考资料

- OpenAI API 文档: https://platform.openai.com/docs/api-reference
- 模型参数量对比: https://lmsys.org/blog/2023-11-21-model-benchmarks/
- 缓存策略最佳实践: https://github.com/ai-server/cache-design

### C. 变更日志

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2024-01-06 | 1.0 | 初始版本 |
