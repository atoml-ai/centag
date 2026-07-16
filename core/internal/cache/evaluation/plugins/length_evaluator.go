package plugins

import (
	"context"
	"fmt"
	"math"
	"time"
	"unicode/utf8"

	"centag/core/internal/cache/evaluation/plugin"
)

// LengthEvaluatorPlugin 长度评估插件
type LengthEvaluatorPlugin struct {
	config *LengthConfig
}

// LengthConfig 长度评估配置
type LengthConfig struct {
	Enabled    bool `json:"enabled" default:"true"`
	MinLength  int  `json:"min_length" default:"50"`
	OptimalMin int  `json:"optimal_min" default:"200"`
	OptimalMax int  `json:"optimal_max" default:"2000"`
	MaxLength  int  `json:"max_length" default:"5000"`
}

// NewLengthEvaluatorPlugin 创建长度评估插件
func NewLengthEvaluatorPlugin() plugin.EvaluatorPlugin {
	return &LengthEvaluatorPlugin{
		config: &LengthConfig{},
	}
}

// Name 返回插件名称
func (p *LengthEvaluatorPlugin) Name() string {
	return "length_evaluator"
}

// Version 返回插件版本
func (p *LengthEvaluatorPlugin) Version() string {
	return "1.0.0"
}

// Type 返回插件类型
func (p *LengthEvaluatorPlugin) Type() plugin.PluginType {
	return plugin.PluginTypeProcess
}

// Description 返回插件描述
func (p *LengthEvaluatorPlugin) Description() string {
	return "基于答案长度评估缓存价值，太短或太长都不适合缓存"
}

// Init 初始化插件
func (p *LengthEvaluatorPlugin) Init() error {
	// 设置默认值
	if p.config.MinLength == 0 {
		p.config.MinLength = 50
	}
	if p.config.OptimalMin == 0 {
		p.config.OptimalMin = 200
	}
	if p.config.OptimalMax == 0 {
		p.config.OptimalMax = 2000
	}
	if p.config.MaxLength == 0 {
		p.config.MaxLength = 5000
	}
	return nil
}

// Close 关闭插件
func (p *LengthEvaluatorPlugin) Close() error {
	return nil
}

// HealthCheck 健康检查
func (p *LengthEvaluatorPlugin) HealthCheck() error {
	return nil
}

// Evaluate 执行评估
func (p *LengthEvaluatorPlugin) Evaluate(
	ctx context.Context,
	input *plugin.EvalInput,
) (*plugin.EvalOutput, error) {
	start := time.Now()

	answer := input.Answer
	length := utf8.RuneCountInString(answer)
	score := 0.0
	passed := true
	label := ""

	switch {
	case length < p.config.MinLength:
		// 太短，不值得缓存
		score = float64(length) / float64(p.config.MinLength) * 30
		passed = false
		label = "too_short"

	case length < p.config.OptimalMin:
		// 偏短但可接受
		ratio := float64(length-p.config.MinLength) /
			float64(p.config.OptimalMin-p.config.MinLength)
		score = 30 + ratio*40
		label = "short_but_acceptable"

	case length <= p.config.OptimalMax:
		// 最佳区间
		// 在最佳区间内，1000字符为最高分点
		optimalCenter := (p.config.OptimalMin + p.config.OptimalMax) / 2
		distance := math.Abs(float64(length-int(optimalCenter)))
		maxDistance := float64(p.config.OptimalMax - int(optimalCenter))
		score = 70 + 30*(1-distance/maxDistance)
		label = "optimal_length"

	case length < p.config.MaxLength:
		// 偏长但仍可接受
		ratio := float64(p.config.MaxLength-length) /
			float64(p.config.MaxLength-p.config.OptimalMax)
		score = 70 * ratio
		label = "long_but_acceptable"

	default:
		// 太长，可能包含过多细节或噪声
		score = 20
		passed = false
		label = "too_long"
	}

	// 确保分数在0-100范围内
	score = clamp(score, 0, 100)

	return &plugin.EvalOutput{
		Score:         score,
		Passed:        passed,
		Labels:        []string{label},
		ProcessTimeMs: time.Since(start).Milliseconds(),
		Details: map[string]interface{}{
			"answer_length":   length,
			"min_length":      p.config.MinLength,
			"optimal_min":     p.config.OptimalMin,
			"optimal_max":     p.config.OptimalMax,
			"max_length":      p.config.MaxLength,
			"character_count": length,
			"byte_count":      len(answer),
		},
	}, nil
}

// GetConfigSchema 获取配置schema
func (p *LengthEvaluatorPlugin) GetConfigSchema() *plugin.ConfigSchema {
	return &plugin.ConfigSchema{
		Fields: []plugin.ConfigField{
			{
				Name:        "min_length",
				Type:        "number",
				Description: "最小长度阈值（字符数），低于此值不缓存",
				Required:    false,
				Default:     50,
				Min:         ptrFloat64(10),
				Max:         ptrFloat64(500),
			},
			{
				Name:        "optimal_min",
				Type:        "number",
				Description: "最佳长度下限",
				Required:    false,
				Default:     200,
				Min:         ptrFloat64(100),
				Max:         ptrFloat64(1000),
			},
			{
				Name:        "optimal_max",
				Type:        "number",
				Description: "最佳长度上限",
				Required:    false,
				Default:     2000,
				Min:         ptrFloat64(500),
				Max:         ptrFloat64(10000),
			},
			{
				Name:        "max_length",
				Type:        "number",
				Description: "最大长度阈值，超过此值不缓存",
				Required:    false,
				Default:     5000,
				Min:         ptrFloat64(1000),
				Max:         ptrFloat64(20000),
			},
		},
	}
}

// ValidateConfig 验证配置
func (p *LengthEvaluatorPlugin) ValidateConfig(config map[string]interface{}) error {
	minLength := getIntFromConfig(config, "min_length", 50)
	optimalMin := getIntFromConfig(config, "optimal_min", 200)
	optimalMax := getIntFromConfig(config, "optimal_max", 2000)
	maxLength := getIntFromConfig(config, "max_length", 5000)

	if minLength >= optimalMin {
		return fmt.Errorf("min_length must be less than optimal_min")
	}
	if optimalMin >= optimalMax {
		return fmt.Errorf("optimal_min must be less than optimal_max")
	}
	if optimalMax >= maxLength {
		return fmt.Errorf("optimal_max must be less than max_length")
	}

	return nil
}

// SetConfig 设置配置
func (p *LengthEvaluatorPlugin) SetConfig(config map[string]interface{}) error {
	if err := p.ValidateConfig(config); err != nil {
		return err
	}

	if enabled, ok := config["enabled"].(bool); ok {
		p.config.Enabled = enabled
	}
	if v := getIntFromConfig(config, "min_length", 0); v > 0 {
		p.config.MinLength = v
	}
	if v := getIntFromConfig(config, "optimal_min", 0); v > 0 {
		p.config.OptimalMin = v
	}
	if v := getIntFromConfig(config, "optimal_max", 0); v > 0 {
		p.config.OptimalMax = v
	}
	if v := getIntFromConfig(config, "max_length", 0); v > 0 {
		p.config.MaxLength = v
	}

	return nil
}

// GetConfig 获取当前配置
func (p *LengthEvaluatorPlugin) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":     p.config.Enabled,
		"min_length":  p.config.MinLength,
		"optimal_min": p.config.OptimalMin,
		"optimal_max": p.config.OptimalMax,
		"max_length":  p.config.MaxLength,
	}
}

// getIntFromConfig 从配置中获取整数
func getIntFromConfig(config map[string]interface{}, key string, defaultVal int) int {
	if v, ok := config[key].(float64); ok {
		return int(v)
	}
	if v, ok := config[key].(int); ok {
		return v
	}
	return defaultVal
}
