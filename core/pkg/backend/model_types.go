package backend

import (
	"encoding/json"
)

// ModelMatchStrategy 模型匹配策略类型
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
	Features []string `json:"features"`
	// 是否支持图像输入
	SupportsImages bool `json:"supports_images"`
	// 是否支持工具调用
	SupportsTools bool `json:"supports_tools"`
}

// ModelInfo 模型信息解析结果
type ModelInfo struct {
	Provider  string // gpt, claude, qwen, deepseek 等
	Family    string // 4, 3.5, 2.5 等
	Variant   string // turbo, pro, mini 等
	Size      string // 7b, 13b, 70b 等
	Precision string // fp16, fp8, int4 等
	Version   string // v1, v2 等
}

// ModelMatchResult 模型匹配结果
type ModelMatchResult struct {
	// 后端ID
	BackendID string
	// 后端名称
	BackendName string
	// 请求的模型
	RequestedModel string
	// 实际使用的模型
	ActualModel string
	// 是否精确匹配
	IsExact bool
	// 兼容性评分（0-1）
	CompatibilityScore float64
	// 使用的匹配策略
	Strategy ModelMatchStrategy
	// 各维度评分详情
	Details MatchDetails
}

// MatchDetails 匹配详情
type MatchDetails struct {
	// 名称相似度（0-1）
	NameSimilarity float64
	// 参数量匹配度（0-1）
	CapacityMatch float64
	// 家族匹配度（0-1）
	FamilyMatch float64
}

// ModelMatchingConfig 模型匹配全局配置
type ModelMatchingConfig struct {
	// 全局匹配策略
	Strategy ModelMatchStrategy `json:"strategy"`
	// 混合策略权重
	HybridWeights HybridWeights `json:"hybrid_weights"`
	// 参数量容忍度
	CapacityTolerance float64 `json:"capacity_tolerance"`
	// 严格度默认值（对应 weight）
	DefaultStrictness int `json:"default_strictness"`
	// 模型转换权重 (0=严格, 100=宽松)
	ConversionWeight int `json:"conversion_weight"`
}

// HybridWeights 混合策略权重配置（三维度，总和应为 1.0）
type HybridWeights struct {
	// 名称相似度权重
	NameSimilarity float64 `json:"name_similarity"`
	// 参数量匹配权重
	CapacityMatch float64 `json:"capacity_match"`
	// 家族匹配权重
	FamilyMatch float64 `json:"family_match"`
}

// ModelCapacityReference 模型参数量基准表
type ModelCapacityReference struct {
	// 参数量单位：B (billion parameters)
	Models map[string]float64 `json:"models"`
}

// DefaultModelMatchingConfig 获取默认模型匹配配置
func DefaultModelMatchingConfig() ModelMatchingConfig {
	return ModelMatchingConfig{
		Strategy: StrategyHybrid,
		HybridWeights: DefaultHybridWeights(),
		CapacityTolerance: 0.2,
		DefaultStrictness: 70,
		ConversionWeight: 50,
	}
}

// DefaultHybridWeights 获取默认混合权重配置（名称50% + 参数量30% + 家族20%）
func DefaultHybridWeights() HybridWeights {
	return HybridWeights{
		NameSimilarity: 0.5,
		CapacityMatch:  0.3,
		FamilyMatch:    0.2,
	}
}

// MarshalJSON 自定义 JSON 序列化
func (m ModelMatchStrategy) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(m))
}

// UnmarshalJSON 自定义 JSON 反序列化
func (m *ModelMatchStrategy) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*m = ModelMatchStrategy(s)
	return nil
}

// GetMinCompatibility 根据严格度获取最小兼容性阈值
func GetMinCompatibility(strictness int) float64 {
	switch {
	case strictness == 0:
		return 1.0 // 严格模式，仅精确匹配
	case strictness <= 30:
		return 1.0 // 保守模式，仅精确匹配
	case strictness <= 70:
		return 0.8 // 平衡模式
	case strictness <= 90:
		return 0.6 // 宽松模式
	default:
		return 0.4 // 极度宽松
	}
}

// AllowConversion 根据严格度判断是否允许模型转换
func AllowConversion(strictness int) bool {
	return strictness > 0
}

// PreferExact 根据严格度判断是否优先精确匹配
func PreferExact(strictness int) bool {
	return strictness == 0 || strictness <= 30
}

// GetMinCompatibility 根据配置获取最小兼容性阈值
func (c ModelMatchingConfig) GetMinCompatibility() float64 {
	// 使用配置的严格度
	strictness := c.DefaultStrictness
	if strictness == 0 {
		return 1.0 // 严格模式，仅精确匹配
	}
	return GetMinCompatibility(strictness)
}

// AllowConversion 根据配置判断是否允许模型转换
func (c ModelMatchingConfig) AllowConversion() bool {
	return AllowConversion(c.ConversionWeight)
}

// PreferExact 根据配置判断是否优先精确匹配
func (c ModelMatchingConfig) PreferExact() bool {
	return PreferExact(c.ConversionWeight)
}
