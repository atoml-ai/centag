package scheduler

import (
	"time"
)

// OptimizeConfig 优化模式配置
type OptimizeConfig struct {
	ExecutorBackendID string `json:"executor_backend"` // 执行后端 ID
	ExecutorModel     string `json:"executor_model"`   // 执行模型名称
	OptimizerBackend  string `json:"optimizer_backend"` // 优化后端 ID
	OptimizerModel    string `json:"optimizer_model"`  // 优化模型名称
	OptimizePrompt    string `json:"optimize_prompt"`  // 优化 Prompt 模板
	AutoRetry         bool   `json:"auto_retry"`        // 优化失败自动重试
	MaxRetries        int    `json:"max_retries"`       // 最大重试次数
	BypassOnTimeout   bool   `json:"bypass_on_timeout"` // 优化超时是否降级返回原始答案
	OptimizeTimeoutSec int    `json:"optimize_timeout_sec"` // 优化超时时间 (秒)
}

// OptimizeResult 优化结果
type OptimizeResult struct {
	Optimized    bool     `json:"optimized"`     // 是否进行了优化
	Original     string   `json:"original"`      // 原始答案
	OptimizedText string  `json:"optimized_text"` // 优化后的答案
	Improvements []string `json:"improvements"`  // 改进点列表
	RawResponse  string   `json:"raw_response"`  // 原始响应
	DurationMs   int64    `json:"duration_ms"`   // 优化耗时
}

// OptimizeDecision 优化决策
type OptimizeDecision struct {
	ExecutorBackendID string          `json:"executor_backend_id"`
	OptimizerBackend  string          `json:"optimizer_backend"`
	ExecutorModel     string          `json:"executor_model"`
	OptimizerModel    string          `json:"optimizer_model"`
	OriginalAnswer    string          `json:"original_answer"`
	OptimizeResult    *OptimizeResult `json:"optimize_result"`
	FinalAnswer       string          `json:"final_answer"`
	RetryCount        int             `json:"retry_count"`
	Action            string          `json:"action"` // optimized/bypass/reject
	Reason            string          `json:"reason"`
}

// DefaultOptimizeConfig 返回默认优化配置
func DefaultOptimizeConfig() *OptimizeConfig {
	return &OptimizeConfig{
		ExecutorBackendID: "bigmodel",
		OptimizerBackend:  "bigmodel",
		OptimizerModel:    "glm-5",
		OptimizePrompt:    DefaultOptimizePrompt,
		AutoRetry:         true,
		MaxRetries:        2,
		BypassOnTimeout:   true,
		OptimizeTimeoutSec: 60,
	}
}

// DefaultOptimizePrompt 默认优化 Prompt 模板
const DefaultOptimizePrompt = `你是一名专业的 AI 助手优化师。请对以下回答进行优化、检验和查错。

## 用户问题
{{.question}}

## 原始回答
{{.answer}}

## 优化要求
1. **错误修正**: 修正回答中的事实错误、逻辑错误或表述不当之处
2. **表达优化**: 提升表达的清晰度、准确性和专业性
3. **信息完善**: 补充遗漏的重要信息，删除冗余内容
4. **格式优化**: 优化结构化输出格式，提升可读性

## 输出格式
请直接输出优化后的完整回答，不要包含任何解释、注释或元数据。只输出优化后的文本内容。

## 优化时间
{{.timestamp}}
`

// OptimizeAction 优化动作枚举
type OptimizeAction string

const (
	OptimizeActionOptimized OptimizeAction = "optimized" // 优化成功
	OptimizeActionRetry    OptimizeAction = "retry"     // 需要重试
	OptimizeActionBypass   OptimizeAction = "bypass"    // 降级返回原始答案
	OptimizeActionReject   OptimizeAction = "reject"     // 优化失败
)

// OptimizeStats 优化统计
type OptimizeStats struct {
	TotalOptimizations int64   `json:"total_optimizations"`
	OptimizedCount     int64   `json:"optimized_count"`
	BypassedCount      int64   `json:"bypassed_count"`
	RetryCount         int64   `json:"retry_count"`
	AvgDurationMs      float64 `json:"avg_duration_ms"`
	LastUpdated        time.Time
}

// UpdateStats 更新优化统计
func (s *OptimizeStats) UpdateStats(result *OptimizeResult, durationMs int64, action OptimizeAction) {
	s.TotalOptimizations++
	s.LastUpdated = time.Now()

	switch action {
	case OptimizeActionOptimized:
		s.OptimizedCount++
	case OptimizeActionRetry:
		s.RetryCount++
	case OptimizeActionBypass:
		s.BypassedCount++
	}

	// 更新平均耗时
	totalDuration := float64(s.TotalOptimizations-1) * s.AvgDurationMs
	s.AvgDurationMs = (totalDuration + float64(durationMs)) / float64(s.TotalOptimizations)
}
