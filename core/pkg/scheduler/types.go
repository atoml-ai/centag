package scheduler

// TaskType 任务类型枚举
type TaskType string

const (
	TaskCodeGeneration   TaskType = "code_generation"   // 代码生成、修改、调试
	TaskSimpleChat       TaskType = "simple_chat"       // 简单问答、闲聊
	TaskComplexReasoning TaskType = "complex_reasoning" // 复杂推理、数学问题、逻辑分析
	TaskLongText         TaskType = "long_text"         // 长文档分析、总结
	TaskEmbedding        TaskType = "embedding"         // 向量生成、语义搜索
	TaskTranslation      TaskType = "translation"       // 翻译任务
	TaskCreative         TaskType = "creative"          // 创意写作、故事生成
	TaskAnalysis         TaskType = "analysis"          // 数据分析、图表解读
	TaskUnknown          TaskType = "unknown"           // 未知类型
)

// ComplexityLevel 复杂度等级
type ComplexityLevel string

const (
	ComplexityLow    ComplexityLevel = "low"     // <1K tokens
	ComplexityMedium ComplexityLevel = "medium"  // 1K-10K tokens
	ComplexityHigh   ComplexityLevel = "high"    // >10K tokens
)

// SensitivityLevel 敏感度等级
type SensitivityLevel string

const (
	SensitivityPublic     SensitivityLevel = "public"     // 公开数据
	SensitivityInternal   SensitivityLevel = "internal"   // 内部数据
	SensitivityConfidential SensitivityLevel = "confidential" // 敏感数据
)

// UrgencyLevel 时效等级
type UrgencyLevel string

const (
	UrgencyLow    UrgencyLevel = "low"    // 可接受延迟
	UrgencyMedium UrgencyLevel = "medium" // 正常响应
	UrgencyHigh   UrgencyLevel = "high"   // 实时响应
)

// ClassificationResult 意图分类结果
type ClassificationResult struct {
	TaskType        TaskType         `json:"task_type"`         // 任务类型
	Confidence      float64          `json:"confidence"`        // 置信度 0-1
	Complexity      ComplexityLevel  `json:"complexity"`        // 复杂度
	Sensitivity     SensitivityLevel `json:"sensitivity"`       // 敏感度
	Urgency         UrgencyLevel     `json:"urgency"`           // 时效要求
	EstimatedTokens int              `json:"estimated_tokens"`  // 预估 token 数
	Reasoning       string           `json:"reasoning"`         // 分类理由
	RawResponse     string           `json:"raw_response"`      // 原始响应（用于调试）
}

// IntentClassifierConfig 意图分类器配置
type IntentClassifierConfig struct {
	Enabled      bool   `json:"enabled"`       // 是否启用
	LocalModel   string `json:"local_model"`   // 本地小模型 (如 qwen2.5:1.5b)
	OllamaAddr   string `json:"ollama_addr"`   // Ollama 地址 (如 http://localhost:21434)
	CacheEnabled bool   `json:"cache_enabled"` // 是否启用缓存
	CacheTTL     int    `json:"cache_ttl"`     // 缓存 TTL (秒)
	Timeout      int    `json:"timeout"`       // 请求超时 (秒)
}

// DefaultIntentClassifierConfig 返回默认配置
func DefaultIntentClassifierConfig() IntentClassifierConfig {
	return IntentClassifierConfig{
		Enabled:      true,
		LocalModel:   "qwen2.5:1.5b",
		OllamaAddr:   "http://localhost:21434",
		CacheEnabled: true,
		CacheTTL:     300, // 5 分钟
		Timeout:      10,
	}
}

// String 返回任务类型的中文描述
func (t TaskType) String() string {
	switch t {
	case TaskCodeGeneration:
		return "代码生成"
	case TaskSimpleChat:
		return "简单对话"
	case TaskComplexReasoning:
		return "复杂推理"
	case TaskLongText:
		return "长文本处理"
	case TaskEmbedding:
		return "向量嵌入"
	case TaskTranslation:
		return "翻译"
	case TaskCreative:
		return "创意写作"
	case TaskAnalysis:
		return "数据分析"
	default:
		return "未知"
	}
}

// String 返回复杂度等级的中文描述
func (c ComplexityLevel) String() string {
	switch c {
	case ComplexityLow:
		return "低"
	case ComplexityMedium:
		return "中"
	case ComplexityHigh:
		return "高"
	default:
		return "未知"
	}
}

// String 返回敏感度等级的中文描述
func (s SensitivityLevel) String() string {
	switch s {
	case SensitivityPublic:
		return "公开"
	case SensitivityInternal:
		return "内部"
	case SensitivityConfidential:
		return "机密"
	default:
		return "未知"
	}
}

// String 返回时效等级的中文描述
func (u UrgencyLevel) String() string {
	switch u {
	case UrgencyLow:
		return "低"
	case UrgencyMedium:
		return "中"
	case UrgencyHigh:
		return "高"
	default:
		return "未知"
	}
}

// BackendCandidate 候选后端（用于成本感知路由）
type BackendCandidate struct {
	BackendID        string  `json:"backend_id"`
	Model            string  `json:"model"`
	DynamicCostPer1k float64 `json:"dynamic_cost_per_1k"` // 动态成本（元/1K tokens）
	PriceType        string  `json:"price_type"`           // "cost" 或 "revenue"
	Tier             int     `json:"tier"`                 // 价格层级（0=免费，1=低价，2=中价，3=高价）
	Enabled          bool    `json:"enabled"`
}

// ScoreWeights 评分权重（用于成本感知路由）
type ScoreWeights struct {
	Cost      float64 `json:"cost"`       // 成本权重（0-1）
	Quality   float64 `json:"quality"`    // 质量权重（0-1）
	Latency   float64 `json:"latency"`    // 延迟权重（0-1）
	Match     float64 `json:"match"`      // 匹配度权重（0-1）
}

// RoutingPolicyType 路由策略类型
type RoutingPolicyType string

const (
	RoutingPolicyCostOptimal  RoutingPolicyType = "cost_optimal"  // 成本优先
	RoutingPolicyBalanced     RoutingPolicyType = "balanced"      // 平衡模式
	RoutingPolicyQualityFirst RoutingPolicyType = "quality_first" // 质量优先
	RoutingPolicyLatencyFirst RoutingPolicyType = "latency_first" // 延迟优先
)

// DefaultScoreWeights 返回默认权重（平衡模式）
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		Cost:    0.25,
		Quality: 0.35,
		Latency: 0.20,
		Match:   0.20,
	}
}

// CostOptimalWeights 返回成本优先权重
func CostOptimalWeights() ScoreWeights {
	return ScoreWeights{
		Cost:    0.50,
		Quality: 0.20,
		Latency: 0.15,
		Match:   0.15,
	}
}

// QualityFirstWeights 返回质量优先权重
func QualityFirstWeights() ScoreWeights {
	return ScoreWeights{
		Cost:    0.10,
		Quality: 0.55,
		Latency: 0.15,
		Match:   0.20,
	}
}

// LatencyFirstWeights 返回延迟优先权重
func LatencyFirstWeights() ScoreWeights {
	return ScoreWeights{
		Cost:    0.15,
		Quality: 0.25,
		Latency: 0.45,
		Match:   0.15,
	}
}

// GetScoreWeightsByPolicy 根据路由策略获取权重
func GetScoreWeightsByPolicy(policy RoutingPolicyType) ScoreWeights {
	switch policy {
	case RoutingPolicyCostOptimal:
		return CostOptimalWeights()
	case RoutingPolicyQualityFirst:
		return QualityFirstWeights()
	case RoutingPolicyLatencyFirst:
		return LatencyFirstWeights()
	default:
		return DefaultScoreWeights()
	}
}
