package scheduler

import (
	"sync"
	"time"
)

// OptimizationStrategy 优化策略类型
type OptimizationStrategy string

const (
	StrategyCost      OptimizationStrategy = "cost"       // 成本优先
	StrategyQuality   OptimizationStrategy = "quality"    // 质量优先
	StrategyLatency   OptimizationStrategy = "latency"    // 延迟优先
	StrategyBalance   OptimizationStrategy = "balance"    // 平衡模式
	StrategyPrivacy   OptimizationStrategy = "privacy"    // 隐私优先
	StrategyDynamic   OptimizationStrategy = "dynamic"    // 动态优化
)

// OptimizationConfig 优化配置
type OptimizationConfig struct {
	Strategy          OptimizationStrategy `json:"strategy"`            // 优化策略
	BudgetLimit       float64              `json:"budget_limit"`        // 日预算限制（元）
	MaxLatencyMs      int64                `json:"max_latency_ms"`      // 最大延迟（ms）
	MinQualityScore   float64              `json:"min_quality_score"`   // 最低质量要求
	EnableFallback    bool                 `json:"enable_fallback"`     // 是否启用降级
	EnableCircuitBreaker bool              `json:"enable_circuit_breaker"` // 是否启用熔断
	EnableLoadBalance bool                `json:"enable_load_balance"`  // 是否启用负载均衡
}

// DefaultOptimizationConfig 默认优化配置
func DefaultOptimizationConfig() OptimizationConfig {
	return OptimizationConfig{
		Strategy:          StrategyBalance,
		BudgetLimit:       100.0, // 100 元/天
		MaxLatencyMs:      3000,  // 3 秒
		MinQualityScore:   0.6,
		EnableFallback:    true,
		EnableCircuitBreaker: true,
		EnableLoadBalance: true,
	}
}

// DecisionOptimizer 决策优化器
type DecisionOptimizer struct {
	mu            sync.RWMutex
	config        OptimizationConfig
	budgetUsed    float64
	budgetResetAt time.Time
	strategyStats map[OptimizationStrategy]*StrategyStats
}

// StrategyStats 策略统计
type StrategyStats struct {
	TotalDecisions   int64   `json:"total_decisions"`
	AvgCost          float64 `json:"avg_cost"`
	AvgLatencyMs     int64   `json:"avg_latency_ms"`
	AvgQualityScore  float64 `json:"avg_quality_score"`
	UserSatisfaction float64 `json:"user_satisfaction"` // 用户满意度 0-1
}

// OptimizedDecision 优化后的决策
type OptimizedDecision struct {
	BackendID        string              `json:"backend_id"`
	BackendName      string              `json:"backend_name"`
	Model            string              `json:"model"`
	Score            float64             `json:"score"`
	Strategy         OptimizationStrategy `json:"strategy"`
	EstimatedCost    float64             `json:"estimated_cost"`
	EstimatedLatencyMs int64             `json:"estimated_latency_ms"`
	IsFallback       bool                `json:"is_fallback"` // 是否为降级选择
	Reason           string              `json:"reason"`
	Alternatives     []BackendAlternative `json:"alternatives"`
}

// NewDecisionOptimizer 创建决策优化器
func NewDecisionOptimizer(config OptimizationConfig) *DecisionOptimizer {
	if config.Strategy == "" {
		config.Strategy = StrategyBalance
	}
	
	return &DecisionOptimizer{
		config: config,
		budgetResetAt: time.Now().Add(24 * time.Hour),
		strategyStats: make(map[OptimizationStrategy]*StrategyStats),
	}
}

// Optimize 优化决策
func (o *DecisionOptimizer) Optimize(
	scores []*BackendScore,
	intent *ClassificationResult,
	currentStrategy OptimizationStrategy,
) *OptimizedDecision {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 1. 应用约束条件过滤
	filtered := o.applyConstraints(scores)
	if len(filtered) == 0 {
		// 所有后端都被过滤，返回降级决策
		return o.createFallbackDecision()
	}

	// 2. 根据策略选择最佳后端
	selected := o.selectByStrategy(filtered, currentStrategy)

	// 3. 检查预算
	if o.isOverBudget(selected.EstimatedCost) {
		// 超出预算，选择最便宜的
		selected = o.selectCheapest(filtered)
		if selected == nil {
			return o.createFallbackDecision()
		}
	}

	// 4. 更新预算使用
	o.budgetUsed += selected.EstimatedCost

	// 5. 生成优化决策
	decision := &OptimizedDecision{
		BackendID:        selected.BackendID,
		BackendName:      selected.BackendName,
		Model:            "", // 从 score 中获取
		Score:            selected.TotalScore,
		Strategy:         currentStrategy,
		EstimatedCost:    selected.EstimatedCost,
		EstimatedLatencyMs: o.estimateLatency(selected.BackendID),
		IsFallback:       false,
		Reason:           o.generateOptimizationReason(selected, currentStrategy),
	}

	// 6. 更新策略统计
	o.updateStrategyStats(currentStrategy, decision)

	return decision
}

// applyConstraints 应用约束条件过滤
func (o *DecisionOptimizer) applyConstraints(scores []*BackendScore) []*BackendScore {
	filtered := make([]*BackendScore, 0)

	for _, score := range scores {
		// 质量约束
		if score.Dimensions.QualityScore < o.config.MinQualityScore {
			continue
		}

		// 延迟约束（如果有历史数据）
		// 这里简化处理，实际应该从 latencyMonitor 获取

		filtered = append(filtered, score)
	}

	return filtered
}

// selectByStrategy 根据策略选择
func (o *DecisionOptimizer) selectByStrategy(scores []*BackendScore, strategy OptimizationStrategy) *BackendScore {
	if len(scores) == 0 {
		return nil
	}

	// 动态策略：根据实时情况调整
	if strategy == StrategyDynamic {
		strategy = o.determineDynamicStrategy()
	}

	switch strategy {
	case StrategyCost:
		return o.selectByPrice(scores)
	case StrategyQuality:
		return o.selectByQuality(scores)
	case StrategyLatency:
		return o.selectByLatency(scores)
	case StrategyPrivacy:
		return o.selectByPrivacy(scores)
	default: // StrategyBalance
		return scores[0] // 已按总分排序
	}
}

// selectByPrice 选择最便宜的
func (o *DecisionOptimizer) selectByPrice(scores []*BackendScore) *BackendScore {
	if len(scores) == 0 {
		return nil
	}

	best := scores[0]
	for _, score := range scores {
		if score.EstimatedCost < best.EstimatedCost {
			best = score
		}
	}
	return best
}

// selectByQuality 选择质量最好的
func (o *DecisionOptimizer) selectByQuality(scores []*BackendScore) *BackendScore {
	if len(scores) == 0 {
		return nil
	}

	best := scores[0]
	for _, score := range scores {
		if score.Dimensions.QualityScore > best.Dimensions.QualityScore {
			best = score
		}
	}
	return best
}

// selectByLatency 选择延迟最低的
func (o *DecisionOptimizer) selectByLatency(scores []*BackendScore) *BackendScore {
	if len(scores) == 0 {
		return nil
	}

	best := scores[0]
	bestLatency := o.estimateLatency(best.BackendID)

	for _, score := range scores[1:] {
		latency := o.estimateLatency(score.BackendID)
		if latency < bestLatency {
			best = score
			bestLatency = latency
		}
	}
	return best
}

// selectByPrivacy 选择隐私保护最好的
func (o *DecisionOptimizer) selectByPrivacy(scores []*BackendScore) *BackendScore {
	if len(scores) == 0 {
		return nil
	}

	best := scores[0]
	for _, score := range scores {
		if score.Dimensions.PrivacyScore > best.Dimensions.PrivacyScore {
			best = score
		}
	}
	return best
}

// selectCheapest 选择最便宜的（预算超限时使用）
func (o *DecisionOptimizer) selectCheapest(scores []*BackendScore) *BackendScore {
	return o.selectByPrice(scores)
}

// isOverBudget 检查是否超出预算
func (o *DecisionOptimizer) isOverBudget(cost float64) bool {
	// 检查是否需要重置预算（新的一天）
	if time.Now().After(o.budgetResetAt) {
		o.budgetUsed = 0
		o.budgetResetAt = time.Now().Add(24 * time.Hour)
	}

	return o.budgetUsed+cost > o.config.BudgetLimit
}

// determineDynamicStrategy 动态决定策略
func (o *DecisionOptimizer) determineDynamicStrategy() OptimizationStrategy {
	// 根据时间段调整策略
	hour := time.Now().Hour()

	// 工作时间（9-18 点）：质量优先
	if hour >= 9 && hour <= 18 {
		return StrategyQuality
	}

	// 非工作时间：成本优先
	return StrategyCost
}

// estimateLatency 估算延迟
func (o *DecisionOptimizer) estimateLatency(backendID string) int64 {
	// 简化实现，实际应该从 latencyMonitor 获取
	switch backendID {
	case "ollama-local":
		return 100
	case "ppinfra", "bigmodel":
		return 500
	default:
		return 300
	}
}

// createFallbackDecision 创建降级决策
func (o *DecisionOptimizer) createFallbackDecision() *OptimizedDecision {
	return &OptimizedDecision{
		BackendID:   "ollama-local",
		BackendName: "Ollama (本地)",
		Model:       "qwen2.5:1.5b",
		Score:       0.5,
		Strategy:    StrategyCost,
		Reason:      "降级方案：所有后端都不满足约束条件",
		IsFallback:  true,
	}
}

// generateOptimizationReason 生成优化理由
func (o *DecisionOptimizer) generateOptimizationReason(score *BackendScore, strategy OptimizationStrategy) string {
	switch strategy {
	case StrategyCost:
		return "成本优先：选择性价比最高的后端"
	case StrategyQuality:
		return "质量优先：选择能力最强的后端"
	case StrategyLatency:
		return "延迟优先：选择响应最快的后端"
	case StrategyPrivacy:
		return "隐私优先：选择本地后端"
	default:
		return "平衡模式：综合考虑成本、质量、延迟"
	}
}

// updateStrategyStats 更新策略统计
func (o *DecisionOptimizer) updateStrategyStats(strategy OptimizationStrategy, decision *OptimizedDecision) {
	stats, ok := o.strategyStats[strategy]
	if !ok {
		stats = &StrategyStats{}
		o.strategyStats[strategy] = stats
	}

	// 指数移动平均更新
	n := float64(stats.TotalDecisions) + 1
	stats.AvgCost = (stats.AvgCost*n + decision.EstimatedCost) / (n + 1)
	stats.AvgLatencyMs = int64((float64(stats.AvgLatencyMs)*n + float64(decision.EstimatedLatencyMs)) / (n + 1))
	stats.TotalDecisions++
}

// GetBudgetUsed 获取已用预算
func (o *DecisionOptimizer) GetBudgetUsed() float64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.budgetUsed
}

// GetBudgetRemaining 获取剩余预算
func (o *DecisionOptimizer) GetBudgetRemaining() float64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.config.BudgetLimit - o.budgetUsed
}

// ResetBudget 重置预算
func (o *DecisionOptimizer) ResetBudget() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.budgetUsed = 0
	o.budgetResetAt = time.Now().Add(24 * time.Hour)
}

// GetStrategyStats 获取策略统计
func (o *DecisionOptimizer) GetStrategyStats(strategy OptimizationStrategy) *StrategyStats {
	o.mu.RLock()
	defer o.mu.RUnlock()

	stats, ok := o.strategyStats[strategy]
	if !ok {
		return nil
	}

	// 返回副本
	statsCopy := *stats
	return &statsCopy
}

// RecordUserFeedback 记录用户反馈
func (o *DecisionOptimizer) RecordUserFeedback(strategy OptimizationStrategy, satisfied bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	stats, ok := o.strategyStats[strategy]
	if !ok {
		stats = &StrategyStats{}
		o.strategyStats[strategy] = stats
	}

	// 更新满意度（指数移动平均）
	if stats.UserSatisfaction == 0 {
		stats.UserSatisfaction = map[bool]float64{true: 1.0, false: 0.0}[satisfied]
	} else {
		newScore := map[bool]float64{true: 1.0, false: 0.0}[satisfied]
		stats.UserSatisfaction = stats.UserSatisfaction*0.9 + newScore*0.1
	}
}

// GetBestStrategy 获取最佳策略（基于用户满意度）
func (o *DecisionOptimizer) GetBestStrategy() OptimizationStrategy {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var bestStrategy OptimizationStrategy
	var bestSatisfaction float64 = -1

	for strategy, stats := range o.strategyStats {
		if stats.UserSatisfaction > bestSatisfaction {
			bestSatisfaction = stats.UserSatisfaction
			bestStrategy = strategy
		}
	}

	if bestSatisfaction < 0 {
		return StrategyBalance
	}

	return bestStrategy
}

// SetConfig 更新配置
func (o *DecisionOptimizer) SetConfig(config OptimizationConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.config = config
}

// GetConfig 获取配置
