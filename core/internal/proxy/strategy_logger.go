package proxy

import (
	"centag/core/pkg/logger"

	"go.uber.org/zap"
)

// StrategyLogger 策略日志记录器
type StrategyLogger struct {
	requestID string
}

// NewStrategyLogger 创建策略日志记录器
func NewStrategyLogger(requestID string) *StrategyLogger {
	return &StrategyLogger{
		requestID: requestID,
	}
}

// LogRequestStart 记录请求开始
func (sl *StrategyLogger) LogRequestStart(model string, messageCount int) {
	logger.Debug("=== 策略决策流程开始 ===",
		zap.String("request_id", sl.requestID),
		zap.String("requested_model", model),
		zap.Int("message_count", messageCount))
}

// LogCacheCheck 记录缓存检查
func (sl *StrategyLogger) LogCacheCheck(cacheType string) {
	logger.Info("[步骤1] 检查缓存",
		zap.String("request_id", sl.requestID),
		zap.String("cache_type", cacheType))
}

// LogCacheHit 记录缓存命中
func (sl *StrategyLogger) LogCacheHit(backend string, similarity float64) {
	logger.Info("[步骤1] 缓存命中",
		zap.String("request_id", sl.requestID),
		zap.String("cached_backend", backend),
		zap.Float64("similarity", similarity),
		zap.String("decision", "直接返回缓存响应"))
}

// LogCacheMiss 记录缓存未命中
func (sl *StrategyLogger) LogCacheMiss(reason string) {
	logger.Info("[步骤1] 缓存未命中",
		zap.String("request_id", sl.requestID),
		zap.String("reason", reason),
		zap.String("decision", "继续执行后端选择"))
}

// LogBackendSelection 记录后端选择流程
func (sl *StrategyLogger) LogBackendSelection(policy string, strategy string) {
	logger.Info("[步骤2] 开始后端选择",
		zap.String("request_id", sl.requestID),
		zap.String("match_policy", policy),
		zap.String("selection_strategy", strategy))
}

// LogModelMatching 记录模型匹配
func (sl *StrategyLogger) LogModelMatching(requestedModel string, matchedModels []string, matchType string) {
	logger.Info("[步骤2.1] 模型匹配",
		zap.String("request_id", sl.requestID),
		zap.String("requested_model", requestedModel),
		zap.Strings("matched_models", matchedModels),
		zap.String("match_type", matchType))
}

// LogCandidateBackends 记录候选后端
func (sl *StrategyLogger) LogCandidateBackends(backends []string) {
	logger.Info("[步骤2.2] 候选后端列表",
		zap.String("request_id", sl.requestID),
		zap.Strings("candidates", backends),
		zap.Int("candidate_count", len(backends)))
}

// LogBackendEvaluation 记录后端评估
func (sl *StrategyLogger) LogBackendEvaluation(backend string, score float64, metrics map[string]interface{}) {
	logger.Info("[步骤2.3] 后端评估",
		zap.String("request_id", sl.requestID),
		zap.String("backend", backend),
		zap.Float64("score", score),
		zap.Any("metrics", metrics))
}

// LogSelectedBackend 记录选中的后端
func (sl *StrategyLogger) LogSelectedBackend(backend string, reason string) {
	logger.Info("[步骤2.4] 后端选择完成",
		zap.String("request_id", sl.requestID),
		zap.String("selected_backend", backend),
		zap.String("selection_reason", reason))
}

// LogRequestForward 记录请求转发
func (sl *StrategyLogger) LogRequestForward(backend string, model string, baseURL string) {
	logger.Info("[步骤3] 请求转发到后端",
		zap.String("request_id", sl.requestID),
		zap.String("backend", backend),
		zap.String("model", model),
		zap.String("base_url", baseURL))
}

// LogResponseReceived 记录响应接收
func (sl *StrategyLogger) LogResponseReceived(backend string, latency int, tokenCount int) {
	logger.Info("[步骤4] 后端响应接收",
		zap.String("request_id", sl.requestID),
		zap.String("backend", backend),
		zap.Int("latency_ms", latency),
		zap.Int("token_count", tokenCount))
}

// LogCacheStore 记录缓存存储
func (sl *StrategyLogger) LogCacheStore(success bool, reason string) {
	logger.Info("[步骤5] 缓存存储",
		zap.String("request_id", sl.requestID),
		zap.Bool("success", success),
		zap.String("reason", reason))
}

// LogStrategySummary 记录策略总结
func (sl *StrategyLogger) LogStrategySummary(summary *StrategySummary) {
	logger.Info("=== 策略决策流程结束 ===",
		zap.String("request_id", sl.requestID),
		zap.String("cache_status", summary.CacheStatus),
		zap.String("selected_backend", summary.SelectedBackend),
		zap.String("selection_reason", summary.SelectionReason),
		zap.Int("total_latency_ms", summary.TotalLatency),
		zap.Bool("from_cache", summary.FromCache))
}

// LogError 记录错误
func (sl *StrategyLogger) LogError(step string, err error) {
	logger.Error("[策略错误]",
		zap.String("request_id", sl.requestID),
		zap.String("step", step),
		zap.Error(err))
}

// LogFallback 记录降级
func (sl *StrategyLogger) LogFallback(from string, to string, reason string) {
	logger.Warn("[策略降级]",
		zap.String("request_id", sl.requestID),
		zap.String("from_backend", from),
		zap.String("to_backend", to),
		zap.String("reason", reason))
}

// StrategySummary 策略总结
type StrategySummary struct {
	CacheStatus     string `json:"cache_status"`     // 缓存状态: hit, miss
	SelectedBackend string `json:"selected_backend"` // 选中的后端
	SelectionReason string `json:"selection_reason"` // 选择原因
	TotalLatency    int    `json:"total_latency_ms"` // 总延迟
	FromCache       bool   `json:"from_cache"`       // 是否来自缓存
}
