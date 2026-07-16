package backend

import (
	"fmt"
)

// BackendSelection 后端选择结果
type BackendSelection struct {
	// 后端ID
	BackendID string
	// 后端名称
	BackendName string
	// 后端类型
	BackendType string
	// 请求的模型
	RequestedModel string
	// 实际使用的模型
	ActualModel string
	// 是否精确匹配
	IsExactMatch bool
	// 兼容性评分（0-1）
	CompatibilityScore float64
	// 使用的匹配策略
	Strategy ModelMatchStrategy
	// 选择标准
	Criteria SelectionCriteria
	// 选择指标
	Metrics SelectionMetrics
	// 后端配置引用
	BackendConfig *BackendConfig
}

// SelectionCriteria 选择标准
type SelectionCriteria struct {
	// 匹配策略
	Strategy ModelMatchStrategy
	// 严格度
	Strictness int
	// 最小兼容性阈值
	MinCompatibility float64
	// 是否允许转换
	AllowConversion bool
	// 是否优先精确匹配
	PreferExact bool
}

// SelectionMetrics 选择指标
type SelectionMetrics struct {
	// 候选后端数量
	CandidateCount int
	// 精确匹配数量
	ExactMatchCount int
	// 兼容匹配数量
	CompatibleMatchCount int
	// 选择耗时（毫秒）
	SelectionTimeMs int64
	// 模型转换评分
	ConversionScore float64
	// 优先级评分
	PriorityScore float64
	// 权重评分
	WeightScore float64
}

// BackendSelectionMetrics 后端选择指标集合
type BackendSelectionMetrics struct {
	// 总选择次数
	TotalSelections int64
	// 精确匹配次数
	ExactMatches int64
	// 模型转换次数
	ConversionMatches int64
	// 无匹配次数
	NoMatches int64
	// 按后端分组的指标
	ByBackend map[string]*BackendMetricStats
}

// BackendMetricStats 单个后端的指标统计
type BackendMetricStats struct {
	// 被选中次数
	SelectionCount int64
	// 精确匹配次数
	ExactMatchCount int64
	// 模型转换次数
	ConversionCount int64
	// 平均兼容性评分
	AverageCompatibility float64
}

// NewBackendSelection 创建后端选择结果
func NewBackendSelection(
	backendID, backendName, backendType string,
	requestedModel, actualModel string,
	isExact bool,
	score float64,
	strategy ModelMatchStrategy,
	backend *BackendConfig,
) *BackendSelection {
	return &BackendSelection{
		BackendID:          backendID,
		BackendName:        backendName,
		BackendType:        backendType,
		RequestedModel:     requestedModel,
		ActualModel:        actualModel,
		IsExactMatch:       isExact,
		CompatibilityScore: score,
		Strategy:           strategy,
		BackendConfig:      backend,
		Criteria: SelectionCriteria{
			Strategy:        strategy,
			AllowConversion: !isExact,
		},
		Metrics: SelectionMetrics{
			ExactMatchCount:        0,
			CompatibleMatchCount:   0,
			PriorityScore:          float64(backend.Priority),
			WeightScore:            float64(backend.Weight),
		},
	}
}

// GetBackendConfig 获取后端配置
func (s *BackendSelection) GetBackendConfig() *BackendConfig {
	return s.BackendConfig
}

// IsModelConverted 判断是否发生了模型转换
func (s *BackendSelection) IsModelConverted() bool {
	return !s.IsExactMatch
}

// GetConfidenceLevel 获取置信度等级
func (s *BackendSelection) GetConfidenceLevel() string {
	if s.IsExactMatch {
		return "high"
	}
	if s.CompatibilityScore >= 0.8 {
		return "high"
	}
	if s.CompatibilityScore >= 0.6 {
		return "medium"
	}
	return "low"
}

// Clone 克隆选择结果
func (s *BackendSelection) Clone() *BackendSelection {
	clone := *s
	clone.BackendConfig = s.BackendConfig
	return &clone
}

// String 返回选择结果的字符串表示
func (s *BackendSelection) String() string {
	return fmt.Sprintf("BackendSelection{BackendID=%s, Model=%s->%s, Score=%.2f, Strategy=%s}",
		s.BackendID, s.RequestedModel, s.ActualModel, s.CompatibilityScore, s.Strategy)
}
