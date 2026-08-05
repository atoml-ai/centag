package scheduler

import (
	"strings"
	"sync"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/logger"
)

// ScoringDimensions 评分维度
type ScoringDimensions struct {
	PriceScore       float64 `json:"price_score"`        // 价格评分 (0-1, 越便宜越高)
	PerformanceScore float64 `json:"performance_score"`  // 性能评分 (0-1, 基于历史统计)
	QualityScore     float64 `json:"quality_score"`      // 质量评分 (0-1, 基于模型能力)
	LatencyScore     float64 `json:"latency_score"`      // 延迟评分 (0-1, 越低延迟越高)
	PrivacyScore     float64 `json:"privacy_score"`      // 隐私评分 (0-1, 本地=1, 云端=0.5)
	MatchScore       float64 `json:"match_score"`        // 匹配度评分 (0-1, 现有 ModelMatcher 评分)
}

// DimensionWeights 维度权重
type DimensionWeights struct {
	Price       float64 `json:"price"`
	Performance float64 `json:"performance"`
	Quality     float64 `json:"quality"`
	Latency     float64 `json:"latency"`
	Privacy     float64 `json:"privacy"`
	Match       float64 `json:"match"`
}

// DefaultWeights 默认权重（平衡模式）
func DefaultWeights() DimensionWeights {
	return DimensionWeights{
		Price:       0.20,
		Performance: 0.20,
		Quality:     0.25,
		Latency:     0.15,
		Privacy:     0.10,
		Match:       0.10,
	}
}

// CostOptimizedWeights 成本优先权重
func CostOptimizedWeights() DimensionWeights {
	return DimensionWeights{
		Price:       0.40,
		Performance: 0.15,
		Quality:     0.15,
		Latency:     0.15,
		Privacy:     0.10,
		Match:       0.05,
	}
}

// QualityOptimizedWeights 质量优先权重
func QualityOptimizedWeights() DimensionWeights {
	return DimensionWeights{
		Price:       0.10,
		Performance: 0.20,
		Quality:     0.40,
		Latency:     0.10,
		Privacy:     0.10,
		Match:       0.10,
	}
}

// LatencyOptimizedWeights 延迟优先权重
func LatencyOptimizedWeights() DimensionWeights {
	return DimensionWeights{
		Price:       0.15,
		Performance: 0.20,
		Quality:     0.15,
		Latency:     0.40,
		Privacy:     0.05,
		Match:       0.05,
	}
}

// BackendScore 后端综合评分
type BackendScore struct {
	BackendID      string             `json:"backend_id"`
	BackendName    string             `json:"backend_name"`
	TotalScore     float64            `json:"total_score"`
	Dimensions     ScoringDimensions  `json:"dimensions"`
	Weights        DimensionWeights   `json:"weights"`
	EstimatedCost  float64            `json:"estimated_cost"`  // 预估成本（元）
	Reason         string             `json:"reason"`          // 评分理由
}

// MultiDimensionScorer 多维评分器
type MultiDimensionScorer struct {
	mu             sync.RWMutex
	priceTable     *ModelPriceTable
	perfCollector  *PerfMetricsCollector
	latencyMonitor *LatencyMonitor
	defaultWeights DimensionWeights
}

// NewMultiDimensionScorer 创建多维评分器
func NewMultiDimensionScorer() *MultiDimensionScorer {
	return &MultiDimensionScorer{
		priceTable:     NewModelPriceTable(),
		perfCollector:  NewPerfMetricsCollector(),
		latencyMonitor: NewLatencyMonitor(100),
		defaultWeights: DefaultWeights(),
	}
}

// ScoreRequest 评分请求
type ScoreRequest struct {
	Backend         *backend.BackendConfig
	Model           string
	Intent          *ClassificationResult
	InputTokens     int
	OutputTokens    int
	Weights         DimensionWeights
}

// Score 对单个后端进行评分
func (s *MultiDimensionScorer) Score(req *ScoreRequest) *BackendScore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	score := &BackendScore{
		BackendID:   req.Backend.ID,
		BackendName: req.Backend.Name,
		Weights:     req.Weights,
	}

	// 1. 价格评分
	score.Dimensions.PriceScore = s.calculatePriceScore(req.Backend.ID, req.Model)
	score.EstimatedCost = s.priceTable.EstimateCost(req.Backend.ID, req.Model, req.InputTokens, req.OutputTokens)

	// 2. 性能评分
	score.Dimensions.PerformanceScore = s.perfCollector.GetPerformanceScore(req.Backend.ID)

	// 3. 质量评分
	score.Dimensions.QualityScore = s.calculateQualityScore(req.Backend, req.Intent)

	// 4. 延迟评分
	score.Dimensions.LatencyScore = s.latencyMonitor.GetLatencyScore(req.Backend.ID)

	// 5. 隐私评分
	score.Dimensions.PrivacyScore = s.calculatePrivacyScore(req.Backend)

	// 6. 匹配度评分
	score.Dimensions.MatchScore = s.calculateMatchScore(req.Backend, req.Model)

	// 计算加权总分
	score.TotalScore = s.calculateWeightedTotal(score.Dimensions, req.Weights)

	// 生成评分理由
	score.Reason = s.generateReason(score)

	return score
}

// calculatePriceScore 计算价格评分
func (s *MultiDimensionScorer) calculatePriceScore(backendID, model string) float64 {
	price := s.priceTable.GetPrice(backendID, model)

	// 免费=1, 低价=0.8, 中价=0.5, 高价=0.2
	switch price.Tier {
	case "free":
		return 1.0
	case "low":
		return 0.8
	case "medium":
		return 0.5
	case "high":
		return 0.2
	default:
		return 0.5
	}
}

// calculateQualityScore 计算质量评分
func (s *MultiDimensionScorer) calculateQualityScore(backend *backend.BackendConfig, intent *ClassificationResult) float64 {
	var baseScore float64

	// 根据后端类型调整
	switch backend.ID {
	case "bigmodel":
		// 综合能力强：代码、推理、分析、创意
		if intent.TaskType == TaskCodeGeneration || intent.TaskType == TaskComplexReasoning || intent.TaskType == TaskAnalysis {
			baseScore = 0.95
		} else if intent.TaskType == TaskCreative {
			baseScore = 0.85
		} else {
			baseScore = 0.75
		}
	case "ppinfra":
		// 模型丰富，性价比高
		if intent.TaskType == TaskLongText {
			baseScore = 0.90 // Kimi
		} else {
			baseScore = 0.75
		}
	case "ollama-local":
		// 本地模型，质量一般
		if intent.Complexity == ComplexityLow {
			baseScore = 0.70
		} else {
			baseScore = 0.50
		}
	default:
		baseScore = 0.60
	}

	// 根据复杂度调整
	if intent.Complexity == ComplexityHigh && backend.ID != "bigmodel" {
		baseScore -= 0.15
	}

	return baseScore
}

// calculatePrivacyScore 计算隐私评分
func (s *MultiDimensionScorer) calculatePrivacyScore(backend *backend.BackendConfig) float64 {
	// 本地后端隐私评分高
	if backend.ID == "ollama-local" {
		return 1.0
	}

	// 云端后端根据敏感度调整
	return 0.5
}

// calculateMatchScore 计算匹配度评分
func (s *MultiDimensionScorer) calculateMatchScore(backend *backend.BackendConfig, requestedModel string) float64 {
	if requestedModel == "" {
		return 0.5
	}

	// 检查是否有精确匹配
	for _, mapping := range backend.SupportedModels {
		if mapping.RequestedModel == requestedModel {
			return 1.0
		}
	}

	// 检查是否有家族匹配（简化版本）
	for _, mapping := range backend.SupportedModels {
		if normalizeModelName(requestedModel) == normalizeModelName(mapping.RequestedModel) {
			return 0.7
		}
	}

	return 0.3
}

// calculateWeightedTotal 计算加权总分
func (s *MultiDimensionScorer) calculateWeightedTotal(dimensions ScoringDimensions, weights DimensionWeights) float64 {
	total := dimensions.PriceScore*weights.Price +
		dimensions.PerformanceScore*weights.Performance +
		dimensions.QualityScore*weights.Quality +
		dimensions.LatencyScore*weights.Latency +
		dimensions.PrivacyScore*weights.Privacy +
		dimensions.MatchScore*weights.Match

	// 归一化（确保权重和为 1）
	weightSum := weights.Price + weights.Performance + weights.Quality +
		weights.Latency + weights.Privacy + weights.Match

	if weightSum > 0 && weightSum != 1.0 {
		total /= weightSum
	}

	return total
}

// generateReason 生成评分理由
func (s *MultiDimensionScorer) generateReason(score *BackendScore) string {
	reasons := []string{}

	if score.Dimensions.PriceScore >= 0.8 {
		reasons = append(reasons, "成本低")
	} else if score.Dimensions.PriceScore <= 0.3 {
		reasons = append(reasons, "成本较高")
	}

	if score.Dimensions.PerformanceScore >= 0.8 {
		reasons = append(reasons, "性能好")
	}

	if score.Dimensions.QualityScore >= 0.8 {
		reasons = append(reasons, "质量高")
	}

	if score.Dimensions.LatencyScore >= 0.8 {
		reasons = append(reasons, "延迟低")
	}

	if score.Dimensions.PrivacyScore >= 0.9 {
		reasons = append(reasons, "隐私保护好")
	}

	if len(reasons) == 0 {
		return "综合评分"
	}

	return "综合评分：" + joinStrings(reasons, ", ")
}

// ScoreAll 对所有后端进行评分
func (s *MultiDimensionScorer) ScoreAll(
	backends []*backend.BackendConfig,
	intent *ClassificationResult,
	requestedModel string,
	inputTokens int,
	outputTokens int,
) []*BackendScore {
	weights := s.getWeightsForIntent(intent)

	scores := make([]*BackendScore, 0, len(backends))
	for _, backend := range backends {
		if !backend.Enabled {
			continue
		}

		score := s.Score(&ScoreRequest{
			Backend:      backend,
			Model:        requestedModel,
			Intent:       intent,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			Weights:      weights,
		})
		scores = append(scores, score)
	}

	// 按总分排序
	sortScores(scores)

	return scores
}

// getWeightsForIntent 根据意图获取权重
func (s *MultiDimensionScorer) getWeightsForIntent(intent *ClassificationResult) DimensionWeights {
	if intent == nil {
		return s.defaultWeights
	}

	switch intent.TaskType {
	case TaskSimpleChat, TaskEmbedding:
		// 简单任务：成本优先
		return CostOptimizedWeights()
	case TaskComplexReasoning, TaskAnalysis, TaskCreative:
		// 复杂任务：质量优先
		return QualityOptimizedWeights()
	case TaskCodeGeneration:
		// 代码任务：质量 + 性能
		return DimensionWeights{
			Price:       0.15,
			Performance: 0.25,
			Quality:     0.35,
			Latency:     0.10,
			Privacy:     0.10,
			Match:       0.05,
		}
	default:
		return s.defaultWeights
	}
}

// RecordRequestResult 记录请求结果（用于更新统计）
func (s *MultiDimensionScorer) RecordRequestResult(backendID, model string, latencyMs int64, success bool, qualityScore float64) {
	// 记录性能指标
	s.perfCollector.RecordRequest(RequestRecord{
		BackendID: backendID,
		Model:     model,
		LatencyMs: latencyMs,
		Success:   success,
		Timestamp: time.Now(),
	})

	// 记录延迟
	s.latencyMonitor.RecordLatency(backendID, latencyMs)

	// 记录质量反馈
	if qualityScore > 0 {
		s.perfCollector.RecordQualityFeedback(backendID, qualityScore)
	}

	logger.Debugf("[Scorer] Recorded request: backend=%s, latency=%dms, success=%v, quality=%.2f",
		backendID, latencyMs, success, qualityScore)
}

// GetPriceTable 获取价格表
func (s *MultiDimensionScorer) GetPriceTable() *ModelPriceTable {
	return s.priceTable
}

// GetPerfCollector 获取性能收集器
func (s *MultiDimensionScorer) GetPerfCollector() *PerfMetricsCollector {
	return s.perfCollector
}

// GetLatencyMonitor 获取延迟监测器
func (s *MultiDimensionScorer) GetLatencyMonitor() *LatencyMonitor {
	return s.latencyMonitor
}

// SetDefaultWeights 设置默认权重
func (s *MultiDimensionScorer) SetDefaultWeights(weights DimensionWeights) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultWeights = weights
}

// sortScores 按总分降序排序
func sortScores(scores []*BackendScore) {
	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[i].TotalScore < scores[j].TotalScore {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
}


// normalizeModelName 简化版模型名规范化
func normalizeModelName(name string) string {
	// 简单处理：转小写，去掉版本号
	name = strings.ToLower(name)
	// 去掉 :version 后缀
	if idx := strings.Index(name, ":"); idx > 0 {
		name = name[:idx]
	}
	return name
}

// joinStrings 连接字符串
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// CalculatePriceScore 公开方法：计算价格评分（用于测试）
func (s *MultiDimensionScorer) CalculatePriceScore(backendID, model string) float64 {
	return s.calculatePriceScore(backendID, model)
}

// GetWeightsForIntent 公开方法：根据意图获取权重（用于测试）
func (s *MultiDimensionScorer) GetWeightsForIntent(intent *ClassificationResult) DimensionWeights {
	return s.getWeightsForIntent(intent)
}

// ScoreCandidate 对候选后端进行成本感知评分
func (s *MultiDimensionScorer) ScoreCandidate(candidate *BackendCandidate, intent *ClassificationResult, weights DimensionWeights) *BackendScore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	score := &BackendScore{
		BackendID:   candidate.BackendID,
		BackendName: candidate.Model,
		Weights:     weights,
	}

	// 成本评分（基于动态成本）
	if candidate.DynamicCostPer1k > 0 {
		// 根据动态成本计算价格评分
		// 假设：免费=1.0，0.01元以下=0.9，0.01-0.05元=0.8，0.05-0.1元=0.6，0.1-0.5元=0.4，0.5元以上=0.2
		switch {
		case candidate.DynamicCostPer1k <= 0:
			score.Dimensions.PriceScore = 1.0
		case candidate.DynamicCostPer1k <= 0.01:
			score.Dimensions.PriceScore = 0.9
		case candidate.DynamicCostPer1k <= 0.05:
			score.Dimensions.PriceScore = 0.8
		case candidate.DynamicCostPer1k <= 0.1:
			score.Dimensions.PriceScore = 0.6
		case candidate.DynamicCostPer1k <= 0.5:
			score.Dimensions.PriceScore = 0.4
		default:
			score.Dimensions.PriceScore = 0.2
		}
		score.EstimatedCost = candidate.DynamicCostPer1k
	} else {
		// 无动态成本，使用静态价格表
		score.Dimensions.PriceScore = s.calculatePriceScore(candidate.BackendID, candidate.Model)
		score.EstimatedCost = s.priceTable.EstimateCost(candidate.BackendID, candidate.Model, 0, 0)
	}

	// 性能评分
	score.Dimensions.PerformanceScore = s.perfCollector.GetPerformanceScore(candidate.BackendID)

	// 质量评分（基于候选后端）
	score.Dimensions.QualityScore = 0.8 // 默认质量为 0.8

	// 延迟评分
	score.Dimensions.LatencyScore = s.latencyMonitor.GetLatencyScore(candidate.BackendID)

	// 隐私评分
	score.Dimensions.PrivacyScore = 0.5 // 默认为云端

	// 匹配度评分
	score.Dimensions.MatchScore = 0.7 // 默认匹配度

	// 计算加权总分
	score.TotalScore = s.calculateWeightedTotal(score.Dimensions, weights)

	// 生成评分理由
	score.Reason = s.generateReason(score)

	return score
}

// ScoreCandidates 对多个候选后端进行评分并排序
func (s *MultiDimensionScorer) ScoreCandidates(candidates []*BackendCandidate, intent *ClassificationResult, weights DimensionWeights) []*BackendScore {
	if len(candidates) == 0 {
		return []*BackendScore{}
	}

	scores := make([]*BackendScore, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.Enabled {
			continue
		}
		score := s.ScoreCandidate(candidate, intent, weights)
		scores = append(scores, score)
	}

	// 按总分排序
	sortScores(scores)

	return scores
}
