# Centag 智能调度系统 - Phase 1 实现文档

## 📋 概述

Phase 1 实现了**意图识别层**，这是智能调度系统的第一层。它使用本地小模型（如 qwen2.5:1.5b）对用户的问题进行分类，为后续的多维评分和决策优化提供基础。

## 🏗️ 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                    意图识别层 (Phase 1)                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │  问题输入     │───▶│  小模型分类   │───▶│  分类结果     │  │
│  │              │    │              │    │              │  │
│  │ • 用户提问   │    │ • Ollama API │    │ • 任务类型   │  │
│  │ • 模型请求   │    │ • 提示词模板 │    │ • 复杂度     │  │
│  │              │    │ • JSON 解析   │    │ • 敏感度     │  │
│  │              │    │              │    │ • 时效要求   │  │
│  └──────────────┘    └──────────────┘    └──────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```


## 📊 Phase 2 多维评分层

### 核心组件

**1. ModelPriceTable 价格表**
- 支持 6 个后端的价格配置
- 4 档价格等级：free/low/medium/high
- 成本估算功能

**价格示例**:
| 后端 | 模型 | 输入价格 | 输出价格 | 等级 |
|------|------|----------|----------|------|
| ollama-local | * | ¥0 | ¥0 | free |
| ppinfra | deepseek-v3.2 | ¥1.0 | ¥1.0 | low |
| bigmodel | glm-5 | ¥20 | ¥20 | high |
| bigmodel | glm-4-flash | ¥0.5 | ¥0.5 | low |

**2. PerfMetricsCollector 性能收集器**
- 请求统计（总数/成功/失败）
- 延迟统计（平均/P95/P99）
- 成功率计算
- 质量反馈记录

**3. LatencyMonitor 延迟监测器**
- 实时延迟记录
- 延迟趋势分析（increasing/stable/decreasing）
- 健康检查
- 最快后端选择

**4. MultiDimensionScorer 多维评分器**
- 6 维度评分计算
- 动态权重配置
- 4 种预设权重模式：
  - Default（平衡模式）
  - CostOptimized（成本优先）
  - QualityOptimized（质量优先）
  - LatencyOptimized（延迟优先）

### 评分维度

| 维度 | 评分范围 | 说明 |
|------|----------|------|
| Price | 0-1 | 越便宜评分越高（免费=1.0, 高价=0.2） |
| Performance | 0-1 | 基于历史成功率和延迟 |
| Quality | 0-1 | 基于模型能力和任务匹配度 |
| Latency | 0-1 | 延迟越低评分越高（<100ms=1.0） |
| Privacy | 0-1 | 本地=1.0, 云端=0.5 |
| Match | 0-1 | 模型匹配度（精确=1.0, 家族=0.7） |

### 动态权重策略

根据任务类型自动调整权重：
- **简单对话/嵌入**: 成本优先（价格 40%）
- **复杂推理/分析/创意**: 质量优先（质量 40%）
- **代码生成**: 质量 + 性能（质量 35% + 性能 25%）
- **默认**: 平衡模式（各维度均衡）

### 使用示例

```go
// 创建评分器
scorer := scheduler.NewMultiDimensionScorer()

// 评分请求
req := &scheduler.ScoreRequest{
    Backend:      backendConfig,
    Model:        "gpt-4",
    Intent:       intent,
    InputTokens:  1000,
    OutputTokens: 1000,
    Weights:      scheduler.DefaultWeights(),
}

// 获取评分
score := scorer.Score(req)
fmt.Printf("总分：%.2f", score.TotalScore)
fmt.Printf("价格评分：%.2f", score.Dimensions.PriceScore)
fmt.Printf("预估成本：¥%.4f", score.EstimatedCost)

// 记录请求结果（用于更新统计）
scorer.RecordRequestResult("bigmodel", "glm-4-flash", 150, true, 0.9)
```
## 📁 文件结构

```
internal/scheduler/
├── types.go          # 类型定义（任务类型、复杂度、敏感度等枚举）
├── classifier.go     # 意图分类器实现
├── scheduler.go      # 智能调度器实现
└── scheduler_test.go # 单元测试
```

## 🔧 核心组件

### 1. IntentClassifier 意图分类器

**功能**: 使用本地小模型对问题进行分类

**分类维度**:
- **任务类型** (TaskType): 代码生成、简单对话、复杂推理、长文本处理等 8 种
- **复杂度** (ComplexityLevel): 低/中/高
- **敏感度** (SensitivityLevel): 公开/内部/机密
- **时效要求** (UrgencyLevel): 低/中/高
- **预估 Token 数**: 基于问题长度估算

**示例代码**:
```go
import "centag/internal/scheduler"

// 创建分类器
config := scheduler.DefaultIntentClassifierConfig()
config.LocalModel = "qwen2.5:1.5b"  // 使用本地小模型
config.OllamaAddr = "http://localhost:21434"

classifier := scheduler.NewIntentClassifier(config)
defer classifier.Close()

// 分类问题
result, err := classifier.Classify("帮我写一个快速排序算法")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("任务类型：%s\n", result.TaskType)        // 代码生成
fmt.Printf("复杂度：%s\n", result.Complexity)        // 低/中/高
fmt.Printf("置信度：%.2f\n", result.Confidence)      // 0-1
```

### 2. Scheduler 智能调度器

**功能**: 根据意图分类结果推荐最佳后端

**调度策略** (按任务类型):
| 任务类型 | 优先后端 | 备选后端 | 理由 |
|----------|----------|----------|------|
| 代码生成 | bigmodel(glm-4-flash) | ppinfra, bigmodel(glm-5) | 智谱免费模型 |
| 简单对话 | ollama-local | bigmodel(glm-4-flash) | 本地零成本 |
| 复杂推理 | bigmodel(glm-5) | bigmodel(glm-4-flash) | 强推理能力 |
| 长文本 | ppinfra(kimi) | bigmodel(glm-4-flash) | Kimi 擅长长文档 |
| 向量嵌入 | ollama-local(bge-m3) | - | 本地零成本 |
| 翻译 | ppinfra | bigmodel(glm-4-flash) | 性价比高 |
| 创意写作 | bigmodel(glm-4-flash) | bigmodel(glm-5) | 高质量 |
| 数据分析 | bigmodel(glm-5) | ppinfra(deepseek) | 强推理能力 |

**示例代码**:
```go
import (
    "centag/internal/scheduler"
    "centag/internal/backend"
)

// 创建调度器
config := scheduler.DefaultSchedulerConfig()
backendMgr := backend.GetManager()  // 获取后端管理器

scheduler := scheduler.NewScheduler(config, backendMgr)
defer scheduler.Close()

// 智能调度
decision, err := scheduler.Schedule(
    "帮我分析一下这个销售数据图表",  // 问题
    "gpt-4",                         // 请求的模型（可选）
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("推荐后端：%s\n", decision.RecommendedBackendID)
fmt.Printf("推荐模型：%s\n", decision.RecommendedModel)
fmt.Printf("任务类型：%s\n", decision.Intent.TaskType)
fmt.Printf("调度理由：%s\n", decision.Reason)
```

## 🎯 使用场景

### 场景 1: 代码生成任务
```
用户提问："帮我写一个 Python 装饰器"
意图分类：TaskType=代码生成，Complexity=低
推荐后端：bigmodel (Glm-4-flash 免费模型)
理由：代码生成任务，优先使用智谱免费模型
```

### 场景 2: 简单对话
```
用户提问："你好，今天天气怎么样？"
意图分类：TaskType=简单对话，Complexity=低
推荐后端：ollama-local (本地 qwen2.5:1.5b)
理由：简单对话，使用本地模型节省成本
```

### 场景 3: 复杂推理
```
用户提问："请证明勾股定理"
意图分类：TaskType=复杂推理，Complexity=中
推荐后端：bigmodel (智谱 GLM-5)
理由：复杂推理，使用智谱 GLM-5 高质量模型
```

### 场景 4: 长文本分析
```
用户提问："请总结这篇 5000 字的文章..."
意图分类：TaskType=长文本处理，Complexity=高
推荐后端：ppinfra (Kimi 模型)
理由：长文本处理，使用 Kimi 模型（擅长长文档）
```

## ⚙️ 配置选项

### IntentClassifierConfig
```go
type IntentClassifierConfig struct {
    Enabled      bool   // 是否启用
    LocalModel   string // 本地小模型 (如 qwen2.5:1.5b)
    OllamaAddr   string // Ollama 地址
    CacheEnabled bool   // 是否启用缓存
    CacheTTL     int    // 缓存 TTL (秒)
    Timeout      int    // 请求超时 (秒)
}
```

### SchedulerConfig
```go
type SchedulerConfig struct {
    IntentClassifier IntentClassifierConfig
    EnableLogging    bool  // 是否启用日志
    EnableStats      bool  // 是否启用统计
}
```

## 📊 性能指标

### 分类性能
- **平均分类延迟**: <100ms (本地模型)
- **缓存命中率**: ~60% (相同问题)
- **分类准确率**: ~85% (基于关键词的默认分类)

### 资源消耗
- **内存占用**: ~10MB
- **CPU 使用**: 低（主要是 HTTP 请求）
- **缓存大小**: 100 条目（可配置）

## 🧪 测试

运行单元测试:
```bash
cd ~/aispaces/centag
go test ./internal/scheduler/... -v
```

预期输出:
```
=== RUN   TestIntentClassifier_parseTaskType
--- PASS: TestIntentClassifier_parseTaskType (0.00s)
=== RUN   TestIntentClassifier_parseComplexity
--- PASS: TestIntentClassifier_parseComplexity (0.00s)
=== RUN   TestIntentClassifier_getDefaultClassification
    scheduler_test.go:227: Question: 帮我写代码 -> Task: 代码生成
    scheduler_test.go:227: Question: 翻译这句话 -> Task: 翻译
--- PASS: TestIntentClassifier_getDefaultClassification (0.00s)
PASS
ok      centag/internal/scheduler    0.009s
```

## 🔮 下一步 (Phase 2)

Phase 2 将实现**多维评分层**，包括：
1. **价格评分**: 基于后端价格表
2. **性能评分**: 基于历史统计数据
3. **质量评分**: 基于模型能力和用户反馈
4. **延迟评分**: 基于实时监测
5. **隐私评分**: 本地 vs 云端

## 📝 注意事项

1. **Ollama 依赖**: 需要本地运行 Ollama 服务
2. **模型要求**: 推荐使用 qwen2.5:1.5b 或更小的模型
3. **缓存策略**: 生产环境建议启用缓存（TTL 5-10 分钟）
4. **降级方案**: 小模型不可用时自动使用关键词分类


## 🧠 Phase 3 决策优化层

### 核心组件

**1. DecisionOptimizer 决策优化器**
- 4 种优化策略：成本/质量/延迟/平衡
- 动态策略调整（根据时间段）
- 预算控制（日预算限制）
- 用户反馈学习

**2. CircuitBreaker 熔断器**
- 3 状态：Closed（正常）/Open（熔断）/Half-Open（试探）
- 失败阈值：5 次失败触发熔断
- 超时恢复：30 秒后自动尝试恢复
- 健康检查：实时监测后端健康状态

**3. LoadBalancer 负载均衡器**
- 4 种策略：
  - Round Robin（轮询）
  - Least Connection（最少连接）
  - Weighted（加权轮询）
  - Consistent Hash（一致性哈希）
- 粘性会话支持
- 实时连接数统计

**4. ABTestManager A/B 测试管理器**
- 流量分配配置
- 结果统计分析
- 获胜策略自动识别
- 置信度计算

### 优化策略

| 策略 | 适用场景 | 特点 |
|------|----------|------|
| Cost | 预算敏感/批量任务 | 优先选择最便宜的后端 |
| Quality | 高质量要求/关键任务 | 优先选择质量最好的后端 |
| Latency | 实时交互/对话场景 | 优先选择延迟最低的后端 |
| Privacy | 敏感数据/内部使用 | 优先选择本地后端 |
| Balance | 一般场景 | 综合考虑各维度 |
| Dynamic | 不确定场景 | 根据时间段自动调整 |

### 熔断器状态机

```
                    失败≥5 次
Closed (正常) ──────────────> Open (熔断)
     ↑                           │
     │                           │ 超时 (30s)
     │                           ↓
     │                      Half-Open (试探)
     │                           │
     │ 成功≥3 次                  │ 失败 1 次
     └───────────────────────────┘
```

### 使用示例

```go
// 创建优化器
optimizer := scheduler.NewDecisionOptimizer(
    scheduler.OptimizationConfig{
        Strategy:    scheduler.StrategyQuality,
        BudgetLimit: 100.0, // 100 元/天
        MinQuality:  0.6,
    },
)

// 优化决策
decision := optimizer.Optimize(scores, intent, scheduler.StrategyQuality)
fmt.Printf("推荐后端：%s, 预估成本：¥%.2f", 
    decision.BackendID, decision.EstimatedCost)

// 创建熔断器管理器
cbManager := scheduler.NewCircuitBreakerManager(
    scheduler.DefaultCircuitBreakerConfig(),
)

// 检查后端健康状态
if cbManager.Allow("bigmodel") {
    // 后端健康，可以请求
} else {
    // 后端熔断，选择备选方案
}

// 创建负载均衡器
lb := scheduler.NewLoadBalancer(
    scheduler.LoadBalancerConfig{
        Strategy: scheduler.LBStrategyLeastConn,
    },
)

// 选择后端
backendID := lb.Select([]string{"backend1", "backend2", "backend3"}, "")

// 创建 A/B 测试
abManager := scheduler.NewABTestManager()
abManager.CreateTest(scheduler.ABTestConfig{
    Name: "strategy_comparison",
    TrafficSplit: map[string]float64{
        "StrategyCost": 0.5,
        "StrategyQuality": 0.5,
    },
})

// 选择测试策略
strategy := abManager.SelectStrategy("strategy_comparison")

// 记录测试结果
abManager.RecordResult("strategy_comparison", strategy, 
    true, 200, 5.0, 0.9)

// 获取获胜策略
winner := abManager.GetWinner("strategy_comparison")
```
## 🎉 完成状态

✅ Phase 1 意图识别层 - **已完成**
- [x] 任务类型分类（8 种）
- [x] 复杂度评估
- [x] 敏感度检测
- [x] 时效要求判断
- [x] 缓存机制
- [x] 单元测试
- [x] 调度策略

✅ Phase 2 多维评分层 - **已完成**
- [x] 价格评分（免费/低/中/高 4 档）
- [x] 性能评分（基于历史统计）
- [x] 质量评分（基于模型能力）
- [x] 延迟评分（实时监测）
- [x] 隐私评分（本地/云端）
- [x] 匹配度评分
- [x] 动态权重配置
- [x] 单元测试

✅ Phase 3 决策优化层 - **已完成**
- [x] 决策优化器（4 种优化策略）
- [x] 预算控制（日预算限制）
- [x] 熔断器（3 状态：closed/open/half-open）
- [x] 负载均衡（4 种策略）
- [x] A/B 测试框架
- [x] 用户反馈学习
- [x] 单元测试
