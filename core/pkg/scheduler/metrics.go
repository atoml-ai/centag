package scheduler

import (
	"sync"
	"time"
)

// BackendPerfStats 后端性能统计
type BackendPerfStats struct {
	BackendID       string    `json:"backend_id"`
	TotalRequests   int64     `json:"total_requests"`    // 总请求数
	SuccessCount    int64     `json:"success_count"`     // 成功次数
	FailureCount    int64     `json:"failure_count"`     // 失败次数
	AvgLatencyMs    int64     `json:"avg_latency_ms"`    // 平均延迟 (ms)
	P95LatencyMs    int64     `json:"p95_latency_ms"`    // P95 延迟 (ms)
	P99LatencyMs    int64     `json:"p99_latency_ms"`    // P99 延迟 (ms)
	SuccessRate     float64   `json:"success_rate"`      // 成功率
	QualityScore    float64   `json:"quality_score"`     // 质量评分 (0-1)
	LastUpdated     time.Time `json:"last_updated"`      // 最后更新时间
	latencyHistory  []int64   // 延迟历史（用于计算百分位数）
	totalLatency    int64     // 总延迟（用于计算平均）
}

// RequestRecord 请求记录
type RequestRecord struct {
	BackendID   string
	Model       string
	LatencyMs   int64
	Success     bool
	TokensUsed  int
	Timestamp   time.Time
}

// PerfMetricsCollector 性能指标收集器
type PerfMetricsCollector struct {
	mu              sync.RWMutex
	stats           map[string]*BackendPerfStats
	maxHistorySize  int
	windowDuration  time.Duration
}

// NewPerfMetricsCollector 创建性能指标收集器
func NewPerfMetricsCollector() *PerfMetricsCollector {
	return &PerfMetricsCollector{
		stats:          make(map[string]*BackendPerfStats),
		maxHistorySize: 1000,
		windowDuration: 1 * time.Hour, // 1 小时窗口
	}
}

// RecordRequest 记录请求
func (c *PerfMetricsCollector) RecordRequest(record RequestRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats, ok := c.stats[record.BackendID]
	if !ok {
		stats = &BackendPerfStats{
			BackendID: record.BackendID,
		}
		c.stats[record.BackendID] = stats
	}

	// 更新计数
	stats.TotalRequests++
	if record.Success {
		stats.SuccessCount++
	} else {
		stats.FailureCount++
	}

	// 更新延迟统计
	stats.totalLatency += record.LatencyMs
	stats.AvgLatencyMs = stats.totalLatency / stats.TotalRequests

	// 更新延迟历史
	stats.latencyHistory = append(stats.latencyHistory, record.LatencyMs)
	if len(stats.latencyHistory) > c.maxHistorySize {
		stats.latencyHistory = stats.latencyHistory[1:]
	}

	// 计算 P95 和 P99
	stats.P95LatencyMs = c.calculatePercentile(stats.latencyHistory, 0.95)
	stats.P99LatencyMs = c.calculatePercentile(stats.latencyHistory, 0.99)

	// 更新成功率
	stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalRequests)

	// 更新时间
	stats.LastUpdated = time.Now()
}

// RecordQualityFeedback 记录质量反馈
func (c *PerfMetricsCollector) RecordQualityFeedback(backendID string, score float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats, ok := c.stats[backendID]
	if !ok {
		stats = &BackendPerfStats{
			BackendID: backendID,
		}
		c.stats[backendID] = stats
	}

	// 指数移动平均更新质量评分
	if stats.QualityScore == 0 {
		stats.QualityScore = score
	} else {
		// 新评分权重 0.1，历史权重 0.9
		stats.QualityScore = stats.QualityScore*0.9 + score*0.1
	}

	stats.LastUpdated = time.Now()
}

// GetStats 获取统计信息
func (c *PerfMetricsCollector) GetStats(backendID string) *BackendPerfStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats, ok := c.stats[backendID]
	if !ok {
		return nil
	}

	// 返回副本
	statsCopy := *stats
	return &statsCopy
}

// GetAllStats 获取所有统计信息
func (c *PerfMetricsCollector) GetAllStats() map[string]*BackendPerfStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*BackendPerfStats)
	for k, v := range c.stats {
		statsCopy := *v
		result[k] = &statsCopy
	}
	return result
}

// calculatePercentile 计算百分位数
func (c *PerfMetricsCollector) calculatePercentile(data []int64, percentile float64) int64 {
	if len(data) == 0 {
		return 0
	}

	// 排序
	sorted := make([]int64, len(data))
	copy(sorted, data)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// 计算百分位数索引
	index := int(float64(len(sorted)) * percentile)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// GetPerformanceScore 获取性能评分 (0-1)
func (c *PerfMetricsCollector) GetPerformanceScore(backendID string) float64 {
	stats := c.GetStats(backendID)
	if stats == nil || stats.TotalRequests == 0 {
		return 0.5 // 默认中等评分
	}

	// 综合评分：成功率 40% + 延迟 30% + 质量 30%
	successScore := stats.SuccessRate

	// 延迟评分（越低越好，<100ms=1, >2000ms=0）
	latencyScore := 1.0 - float64(stats.AvgLatencyMs)/2000.0
	if latencyScore < 0 {
		latencyScore = 0
	}
	if latencyScore > 1 {
		latencyScore = 1
	}

	qualityScore := stats.QualityScore
	if qualityScore == 0 {
		qualityScore = 0.5 // 无反馈时默认中等
	}

	performanceScore := successScore*0.4 + latencyScore*0.3 + qualityScore*0.3

	return performanceScore
}

// Reset 重置统计
func (c *PerfMetricsCollector) Reset(backendID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if backendID == "" {
		c.stats = make(map[string]*BackendPerfStats)
	} else {
		delete(c.stats, backendID)
	}
}

// Cleanup 清理过期数据
func (c *PerfMetricsCollector) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for backendID, stats := range c.stats {
		if now.Sub(stats.LastUpdated) > c.windowDuration*2 {
			delete(c.stats, backendID)
		}
	}
}
