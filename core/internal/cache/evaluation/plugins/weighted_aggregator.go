package plugins

import (
	"context"
	"fmt"
	"time"

	"centag/core/internal/cache/evaluation/plugin"
)

// WeightedAggregatorPlugin 加权聚合插件
type WeightedAggregatorPlugin struct {
	config *AggregatorConfig
}

// AggregatorConfig 聚合器配置
type AggregatorConfig struct {
	Enabled  bool              `json:"enabled" default:"true"`
	MinScore float64           `json:"min_score" default:"50"`
	Strategy string            `json:"strategy" default:"weighted_sum"` // weighted_sum | min | max | average
	Weights  map[string]float64 `json:"weights"`
}

// NewWeightedAggregatorPlugin 创建加权聚合插件
func NewWeightedAggregatorPlugin() plugin.EvaluatorPlugin {
	return &WeightedAggregatorPlugin{
		config: &AggregatorConfig{
			Weights: make(map[string]float64),
		},
	}
}

// Name 返回插件名称
func (p *WeightedAggregatorPlugin) Name() string {
	return "weighted_aggregator"
}

// Version 返回插件版本
func (p *WeightedAggregatorPlugin) Version() string {
	return "1.0.0"
}

// Type 返回插件类型
func (p *WeightedAggregatorPlugin) Type() plugin.PluginType {
	return plugin.PluginTypeOutput
}

// Description 返回插件描述
func (p *WeightedAggregatorPlugin) Description() string {
	return "聚合前面所有插件的评分，计算最终缓存决策"
}

// Init 初始化插件
func (p *WeightedAggregatorPlugin) Init() error {
	// 设置默认权重
	if len(p.config.Weights) == 0 {
		p.config.Weights = defaultWeights()
	}
	// 设置默认策略
	if p.config.Strategy == "" {
		p.config.Strategy = "weighted_sum"
	}
	return nil
}

// Close 关闭插件
func (p *WeightedAggregatorPlugin) Close() error {
	return nil
}

// HealthCheck 健康检查
func (p *WeightedAggregatorPlugin) HealthCheck() error {
	return nil
}

// Evaluate 执行评估
func (p *WeightedAggregatorPlugin) Evaluate(
	ctx context.Context,
	input *plugin.EvalInput,
) (*plugin.EvalOutput, error) {
	start := time.Now()

	results := input.PreviousResults

	if len(results) == 0 {
		return &plugin.EvalOutput{
			Score:         50,
			Passed:        true,
			Labels:        []string{"no_inputs"},
			Details:       map[string]interface{}{"reason": "no_previous_results"},
			ProcessTimeMs: time.Since(start).Milliseconds(),
		}, nil
	}

	var finalScore float64
	var calculationDetail string

	switch p.config.Strategy {
	case "weighted_sum":
		finalScore = p.weightedSum(results)
		calculationDetail = "weighted_sum"
	case "min":
		finalScore = p.minScore(results)
		calculationDetail = "minimum_score"
	case "max":
		finalScore = p.maxScore(results)
		calculationDetail = "maximum_score"
	case "average":
		finalScore = p.averageScore(results)
		calculationDetail = "average_score"
	default:
		finalScore = p.weightedSum(results)
		calculationDetail = "weighted_sum(default)"
	}

	// 确保分数在有效范围内
	finalScore = clamp(finalScore, 0, 100)

	passed := finalScore >= p.config.MinScore

	// 构建详细信息
	scoreBreakdown := make(map[string]float64)
	for name, result := range results {
		scoreBreakdown[name] = result.Score
	}

	return &plugin.EvalOutput{
		Score:         finalScore,
		Passed:        passed,
		Labels:        []string{"aggregated"},
		ProcessTimeMs: time.Since(start).Milliseconds(),
		Details: map[string]interface{}{
			"strategy":          calculationDetail,
			"input_count":       len(results),
			"score_breakdown":   scoreBreakdown,
			"min_threshold":     p.config.MinScore,
			"applied_weights":   p.getAppliedWeights(results),
		},
	}, nil
}

// weightedSum 加权求和
func (p *WeightedAggregatorPlugin) weightedSum(results map[string]*plugin.EvalOutput) float64 {
	totalScore := 0.0
	totalWeight := 0.0

	for name, result := range results {
		weight := p.config.Weights[name]
		if weight == 0 {
			weight = 1.0 // 默认权重
		}
		totalScore += result.Score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 50
	}

	return totalScore / totalWeight
}

// minScore 取最小值
func (p *WeightedAggregatorPlugin) minScore(results map[string]*plugin.EvalOutput) float64 {
	minScore := 100.0
	for _, result := range results {
		if result.Score < minScore {
			minScore = result.Score
		}
	}
	return minScore
}

// maxScore 取最大值
func (p *WeightedAggregatorPlugin) maxScore(results map[string]*plugin.EvalOutput) float64 {
	maxScore := 0.0
	for _, result := range results {
		if result.Score > maxScore {
			maxScore = result.Score
		}
	}
	return maxScore
}

// averageScore 取平均值
func (p *WeightedAggregatorPlugin) averageScore(results map[string]*plugin.EvalOutput) float64 {
	totalScore := 0.0
	count := 0
	for _, result := range results {
		totalScore += result.Score
		count++
	}
	if count == 0 {
		return 50
	}
	return totalScore / float64(count)
}

// getAppliedWeights 获取实际应用的权重
func (p *WeightedAggregatorPlugin) getAppliedWeights(results map[string]*plugin.EvalOutput) map[string]float64 {
	applied := make(map[string]float64)
	for name := range results {
		weight := p.config.Weights[name]
		if weight == 0 {
			weight = 1.0
		}
		applied[name] = weight
	}
	return applied
}

// GetConfigSchema 获取配置schema
func (p *WeightedAggregatorPlugin) GetConfigSchema() *plugin.ConfigSchema {
	return &plugin.ConfigSchema{
		Fields: []plugin.ConfigField{
			{
				Name:        "min_score",
				Type:        "number",
				Description: "通过阈值，最终分数需大于等于此值才缓存",
				Required:    false,
				Default:     50,
				Min:         ptrFloat64(0),
				Max:         ptrFloat64(100),
			},
			{
				Name:        "strategy",
				Type:        "string",
				Description: "聚合策略",
				Required:    false,
				Default:     "weighted_sum",
				Options:     []string{"weighted_sum", "min", "max", "average"},
			},
			{
				Name:        "weights",
				Type:        "object",
				Description: "各插件的权重配置（插件名->权重）",
				Required:    false,
				Default:     defaultWeights(),
			},
		},
	}
}

// ValidateConfig 验证配置
func (p *WeightedAggregatorPlugin) ValidateConfig(config map[string]interface{}) error {
	if strategy, ok := config["strategy"].(string); ok {
		validStrategies := map[string]bool{
			"weighted_sum": true,
			"min":          true,
			"max":          true,
			"average":      true,
		}
		if !validStrategies[strategy] {
			return fmt.Errorf("invalid strategy: %s", strategy)
		}
	}

	if minScore, ok := config["min_score"].(float64); ok {
		if minScore < 0 || minScore > 100 {
			return fmt.Errorf("min_score must be between 0 and 100")
		}
	}

	return nil
}

// SetConfig 设置配置
func (p *WeightedAggregatorPlugin) SetConfig(config map[string]interface{}) error {
	if err := p.ValidateConfig(config); err != nil {
		return err
	}

	if enabled, ok := config["enabled"].(bool); ok {
		p.config.Enabled = enabled
	}
	if minScore, ok := config["min_score"].(float64); ok {
		p.config.MinScore = minScore
	}
	if strategy, ok := config["strategy"].(string); ok {
		p.config.Strategy = strategy
	}
	if weights, ok := config["weights"].(map[string]interface{}); ok {
		newWeights := make(map[string]float64)
		for k, v := range weights {
			if fv, ok := v.(float64); ok {
				newWeights[k] = fv
			}
		}
		p.config.Weights = newWeights
	}

	return nil
}

// GetConfig 获取当前配置
func (p *WeightedAggregatorPlugin) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":   p.config.Enabled,
		"min_score": p.config.MinScore,
		"strategy":  p.config.Strategy,
		"weights":   p.config.Weights,
	}
}

// defaultWeights 返回默认权重
func defaultWeights() map[string]float64 {
	return map[string]float64{
		"follow_up_detector": 0.15,
		"regex_matcher":      0.10,
		"length_evaluator":   0.20,
		"density_evaluator":  0.25,
		"context_dependency": 0.30,
	}
}
