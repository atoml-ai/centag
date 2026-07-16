package backend

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector 指标收集器
type MetricsCollector struct {
	// 总体指标（使用原子操作）
	totalRequests     atomic.Int64
	totalExactMatches atomic.Int64
	totalConversions  atomic.Int64
	totalNoMatches    atomic.Int64

	// 模型级别指标（使用sync.Map提升并发性能）
	modelMetrics sync.Map // map[string]*ModelMetrics

	// 后端级别指标（使用sync.Map提升并发性能）
	backendMetrics sync.Map // map[string]*BackendMetrics

	// 策略使用统计（低频写，使用RWMutex）
	strategyMu    sync.RWMutex
	strategyUsage map[ModelMatchStrategy]int64

	// 开始时间
	startTime time.Time
}

// ModelMetrics 模型级别指标
type ModelMetrics struct {
	// 请求统计（使用原子操作）
	requestCount    atomic.Int64
	exactMatchCount atomic.Int64
	conversionCount atomic.Int64
	noMatchCount    atomic.Int64

	// 兼容性评分统计（低频读写，使用mutex）
	scoreMu     sync.RWMutex
	totalScore  float64
	minScore    float64
	maxScore    float64
	scoreCount  int64

	// 置信度统计（使用原子操作）
	highConfidence   atomic.Int64
	mediumConfidence atomic.Int64
	lowConfidence    atomic.Int64

	// 延迟统计（低频读写，使用mutex）
	latencyMu    sync.RWMutex
	totalLatency int64
	minLatency   int64
	maxLatency   int64
	latencyCount int64
}

// BackendMetrics 后端级别指标
type BackendMetrics struct {
	// 选择统计（使用原子操作）
	selectedCount atomic.Int64

	// 请求统计（使用原子操作）
	totalRequests atomic.Int64
	errorCount    atomic.Int64

	// 延迟统计（低频读写，使用mutex）
	latencyMu    sync.RWMutex
	totalLatency int64
	minLatency   int64
	maxLatency   int64
	latencyCount int64
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		strategyUsage: make(map[ModelMatchStrategy]int64),
		startTime:     time.Now(),
	}
}

// RecordSelection 记录选择决策
func (mc *MetricsCollector) RecordSelection(
	requestedModel string,
	backendID string,
	strategy ModelMatchStrategy,
	isExact bool,
	score float64,
	latency time.Duration,
) {
	// 使用原子操作更新总体计数
	mc.totalRequests.Add(1)

	// 更新策略使用统计
	mc.strategyMu.Lock()
	mc.strategyUsage[strategy]++
	mc.strategyMu.Unlock()

	// 更新模型指标
	modelMetrics := mc.getOrCreateModelMetrics(requestedModel)
	modelMetrics.RecordSelection(isExact, score, latency)

	// 更新后端指标
	backendMetrics := mc.getOrCreateBackendMetrics(backendID)
	backendMetrics.RecordSelection(latency)

	// 更新总体指标
	if isExact {
		mc.totalExactMatches.Add(1)
	} else if score > 0 {
		mc.totalConversions.Add(1)
	}

	log.Printf("[Metrics] Recorded selection: model=%s, backend=%s, strategy=%s, exact=%v, score=%.3f",
		requestedModel, backendID, strategy, isExact, score)
}

// RecordExactMatch 记录精确匹配
func (mc *MetricsCollector) RecordExactMatch(
	requestedModel string,
	backendID string,
) {
	mc.totalExactMatches.Add(1)

	modelMetrics := mc.getOrCreateModelMetrics(requestedModel)
	modelMetrics.exactMatchCount.Add(1)

	log.Printf("[Metrics] Recorded exact match: model=%s, backend=%s",
		requestedModel, backendID)
}

// RecordConversion 记录模型转换
func (mc *MetricsCollector) RecordConversion(
	requestedModel string,
	actualModel string,
	backendID string,
	score float64,
) {
	mc.totalConversions.Add(1)

	modelMetrics := mc.getOrCreateModelMetrics(requestedModel)
	modelMetrics.conversionCount.Add(1)

	log.Printf("[Metrics] Recorded conversion: requested=%s, actual=%s, backend=%s, score=%.3f",
		requestedModel, actualModel, backendID, score)
}

// RecordNoMatch 记录无匹配
func (mc *MetricsCollector) RecordNoMatch(
	requestedModel string,
) {
	mc.totalNoMatches.Add(1)

	modelMetrics := mc.getOrCreateModelMetrics(requestedModel)
	modelMetrics.noMatchCount.Add(1)

	log.Printf("[Metrics] Recorded no match: model=%s", requestedModel)
}

// RecordBackendError 记录后端错误
func (mc *MetricsCollector) RecordBackendError(
	backendID string,
) {
	backendMetrics := mc.getOrCreateBackendMetrics(backendID)
	backendMetrics.errorCount.Add(1)

	log.Printf("[Metrics] Recorded backend error: backend=%s", backendID)
}

// GetModelMetrics 获取模型指标
func (mc *MetricsCollector) GetModelMetrics(model string) *ModelMetrics {
	if value, ok := mc.modelMetrics.Load(model); ok {
		return value.(*ModelMetrics)
	}
	return &ModelMetrics{}
}

// GetBackendMetrics 获取后端指标
func (mc *MetricsCollector) GetBackendMetrics(backendID string) *BackendMetrics {
	if value, ok := mc.backendMetrics.Load(backendID); ok {
		return value.(*BackendMetrics)
	}
	return &BackendMetrics{}
}

// GetTotalMetrics 获取总体指标
func (mc *MetricsCollector) GetTotalMetrics() (
	totalRequests int64,
	exactMatches int64,
	conversions int64,
	noMatches int64,
) {
	return mc.totalRequests.Load(),
		mc.totalExactMatches.Load(),
		mc.totalConversions.Load(),
		mc.totalNoMatches.Load()
}

// GetStrategyUsage 获取策略使用统计
func (mc *MetricsCollector) GetStrategyUsage() map[ModelMatchStrategy]int64 {
	mc.strategyMu.RLock()
	defer mc.strategyMu.RUnlock()

	result := make(map[ModelMatchStrategy]int64)
	for k, v := range mc.strategyUsage {
		result[k] = v
	}
	return result
}

// ResetMetrics 重置所有指标
func (mc *MetricsCollector) ResetMetrics() {
	mc.totalRequests.Store(0)
	mc.totalExactMatches.Store(0)
	mc.totalConversions.Store(0)
	mc.totalNoMatches.Store(0)
	
	// 重置sync.Map
	mc.modelMetrics = sync.Map{}
	mc.backendMetrics = sync.Map{}
	
	mc.strategyMu.Lock()
	mc.strategyUsage = make(map[ModelMatchStrategy]int64)
	mc.strategyMu.Unlock()
	
	mc.startTime = time.Now()

	log.Printf("[Metrics] All metrics reset")
}

// RecordSelection ModelMetrics 记录选择
func (mm *ModelMetrics) RecordSelection(
	isExact bool,
	score float64,
	latency time.Duration,
) {
	mm.requestCount.Add(1)

	// 更新评分统计
	mm.scoreMu.Lock()
	mm.totalScore += score
	mm.scoreCount++
	if score < mm.minScore || mm.scoreCount == 1 {
		mm.minScore = score
	}
	if score > mm.maxScore {
		mm.maxScore = score
	}
	mm.scoreMu.Unlock()

	// 更新置信度（使用原子操作）
	if score >= 0.8 || isExact {
		mm.highConfidence.Add(1)
	} else if score >= 0.5 {
		mm.mediumConfidence.Add(1)
	} else {
		mm.lowConfidence.Add(1)
	}

	// 更新延迟统计
	latencyNs := latency.Nanoseconds()
	mm.latencyMu.Lock()
	mm.totalLatency += latencyNs
	mm.latencyCount++
	if latencyNs < mm.minLatency || mm.latencyCount == 1 {
		mm.minLatency = latencyNs
	}
	if latencyNs > mm.maxLatency {
		mm.maxLatency = latencyNs
	}
	mm.latencyMu.Unlock()
}

// GetAverageScore 获取平均评分
func (mm *ModelMetrics) GetAverageScore() float64 {
	mm.scoreMu.RLock()
	defer mm.scoreMu.RUnlock()

	if mm.scoreCount == 0 {
		return 0
	}
	return mm.totalScore / float64(mm.scoreCount)
}

// GetAverageLatency 获取平均延迟
func (mm *ModelMetrics) GetAverageLatency() time.Duration {
	mm.latencyMu.RLock()
	defer mm.latencyMu.RUnlock()

	if mm.latencyCount == 0 {
		return 0
	}
	return time.Duration(mm.totalLatency / mm.latencyCount)
}

// GetMatchRate 获取匹配率
func (mm *ModelMetrics) GetMatchRate() float64 {
	requests := mm.requestCount.Load()
	noMatch := mm.noMatchCount.Load()

	if requests == 0 {
		return 0
	}
	return float64(requests-noMatch) / float64(requests)
}

// RecordSelection BackendMetrics 记录选择
func (bm *BackendMetrics) RecordSelection(latency time.Duration) {
	bm.selectedCount.Add(1)

	// 更新延迟统计
	latencyNs := latency.Nanoseconds()
	bm.latencyMu.Lock()
	bm.totalLatency += latencyNs
	bm.latencyCount++
	if latencyNs < bm.minLatency || bm.latencyCount == 1 {
		bm.minLatency = latencyNs
	}
	if latencyNs > bm.maxLatency {
		bm.maxLatency = latencyNs
	}
	bm.latencyMu.Unlock()
}

// GetAverageLatency 获取平均延迟
func (bm *BackendMetrics) GetAverageLatency() time.Duration {
	bm.latencyMu.RLock()
	defer bm.latencyMu.RUnlock()

	if bm.latencyCount == 0 {
		return 0
	}
	return time.Duration(bm.totalLatency / bm.latencyCount)
}

// GetSelectionRate 获取选择率
func (bm *BackendMetrics) GetSelectionRate() float64 {
	total := bm.totalRequests.Load()
	selected := bm.selectedCount.Load()

	if total == 0 {
		return 0
	}
	return float64(selected) / float64(total)
}

// getOrCreateModelMetrics 获取或创建模型指标
func (mc *MetricsCollector) getOrCreateModelMetrics(model string) *ModelMetrics {
	if value, ok := mc.modelMetrics.Load(model); ok {
		return value.(*ModelMetrics)
	}

	metrics := &ModelMetrics{}
	metrics.minLatency = int64(^uint64(0) >> 1) // Max int64
	
	actual, _ := mc.modelMetrics.LoadOrStore(model, metrics)
	return actual.(*ModelMetrics)
}

// getOrCreateBackendMetrics 获取或创建后端指标
func (mc *MetricsCollector) getOrCreateBackendMetrics(backendID string) *BackendMetrics {
	if value, ok := mc.backendMetrics.Load(backendID); ok {
		return value.(*BackendMetrics)
	}

	metrics := &BackendMetrics{}
	metrics.minLatency = int64(^uint64(0) >> 1) // Max int64
	
	actual, _ := mc.backendMetrics.LoadOrStore(backendID, metrics)
	return actual.(*BackendMetrics)
}
