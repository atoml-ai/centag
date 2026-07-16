package metrics

import (
	"sync"
	"time"
)

// Metrics 指标收集器
type Metrics struct {
	mu sync.RWMutex

	// 请求统计
	totalRequests    int64
	successRequests  int64
	errorRequests    int64
	totalLatency     time.Duration // 总延迟（纳秒）
	lastRequestTime  time.Time

	// QPS 计算
	requestsInWindow []time.Time // 滑动窗口
	windowSize       time.Duration
	windowLimit      int

	// 按模型统计
	modelStats map[string]*ModelStats

	// 按状态码统计
	statusCodes map[int]int64

	// 创建时间
	startTime time.Time
}

// ModelStats 模型统计
type ModelStats struct {
	TotalRequests int64
	TotalLatency  time.Duration
	ErrorRequests int64
	CacheHits     int64
	CacheMisses   int64
}

// GlobalMetrics 全局指标实例
var GlobalMetrics *Metrics

// Init 初始化全局指标
func Init() {
	GlobalMetrics = &Metrics{
		modelStats:       make(map[string]*ModelStats),
		statusCodes:      make(map[int]int64),
		requestsInWindow: make([]time.Time, 0),
		windowSize:       time.Minute,
		windowLimit:      1000,
		startTime:        time.Now(),
		lastRequestTime:  time.Now(),
	}
	GlobalRouteBackendMetrics = newRouteBackendMetricsCollector()
}



// RecordRequest 记录请求
func (m *Metrics) RecordRequest(model string, statusCode int, latency time.Duration, cached bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.totalRequests++
	m.lastRequestTime = now

	// 记录延迟
	m.totalLatency += latency

	// 记录状态码
	m.statusCodes[statusCode]++

	// 统计成功/失败
	if statusCode >= 200 && statusCode < 300 {
		m.successRequests++
	} else {
		m.errorRequests++
	}

	// 添加到滑动窗口
	m.requestsInWindow = append(m.requestsInWindow, now)
	// 清理窗口外的请求
	cutoff := now.Add(-m.windowSize)
	var validRequests []time.Time
	for _, t := range m.requestsInWindow {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	m.requestsInWindow = validRequests

	// 限制窗口大小
	if len(m.requestsInWindow) > m.windowLimit {
		m.requestsInWindow = m.requestsInWindow[len(m.requestsInWindow)-m.windowLimit:]
	}

	// 模型统计
	if model == "" {
		model = "unknown"
	}
	stats, exists := m.modelStats[model]
	if !exists {
		stats = &ModelStats{}
		m.modelStats[model] = stats
	}
	stats.TotalRequests++
	stats.TotalLatency += latency
	if statusCode >= 200 && statusCode < 300 {
		// 成功
	} else {
		stats.ErrorRequests++
	}

	// 缓存统计
	if cached {
		stats.CacheHits++
	} else {
		stats.CacheMisses++
	}
}

// GetQPS 获取当前 QPS
func (m *Metrics) GetQPS() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.requestsInWindow) < 2 {
		return 0
	}

	duration := m.requestsInWindow[len(m.requestsInWindow)-1].Sub(m.requestsInWindow[0])
	if duration <= 0 {
		return 0
	}

	return float64(len(m.requestsInWindow)) / duration.Seconds()
}

// GetStats 获取统计信息
func (m *Metrics) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgLatency := time.Duration(0)
	if m.totalRequests > 0 {
		avgLatency = m.totalLatency / time.Duration(m.totalRequests)
	}

	errorRate := float64(0)
	if m.totalRequests > 0 {
		errorRate = float64(m.errorRequests) / float64(m.totalRequests) * 100
	}

	uptime := time.Since(m.startTime)

	stats := &Stats{
		TotalRequests:   m.totalRequests,
		SuccessRequests: m.successRequests,
		ErrorRequests:   m.errorRequests,
		QPS:             m.GetQPS(),
		AvgLatency:      avgLatency.Milliseconds(),
		ErrorRate:       errorRate,
		Uptime:          uptime.Milliseconds(),
		LastRequestTime: m.lastRequestTime,
		StatusCodes:     make(map[int]int64),
		ModelStats:      make(map[string]*ModelStatsSnapshot),
	}

	// 复制状态码统计
	for code, count := range m.statusCodes {
		stats.StatusCodes[code] = count
	}

	// 复制模型统计
	for model, s := range m.modelStats {
		avgModelLatency := time.Duration(0)
		if s.TotalRequests > 0 {
			avgModelLatency = s.TotalLatency / time.Duration(s.TotalRequests)
		}

		cacheHitRate := float64(0)
		if s.CacheHits+s.CacheMisses > 0 {
			cacheHitRate = float64(s.CacheHits) / float64(s.CacheHits+s.CacheMisses) * 100
		}

		stats.ModelStats[model] = &ModelStatsSnapshot{
			TotalRequests:  s.TotalRequests,
			AvgLatency:     avgModelLatency.Milliseconds(),
			ErrorRequests:  s.ErrorRequests,
			CacheHits:      s.CacheHits,
			CacheMisses:    s.CacheMisses,
			CacheHitRate:   cacheHitRate,
		}
	}

	return stats
}

// Reset 重置统计
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRequests = 0
	m.successRequests = 0
	m.errorRequests = 0
	m.totalLatency = 0
	m.requestsInWindow = make([]time.Time, 0)
	m.modelStats = make(map[string]*ModelStats)
	m.statusCodes = make(map[int]int64)
	m.startTime = time.Now()
	m.lastRequestTime = time.Now()
}

// Stats 统计信息快照
type Stats struct {
	TotalRequests   int64                       `json:"total_requests"`
	SuccessRequests  int64                       `json:"success_requests"`
	ErrorRequests    int64                       `json:"error_requests"`
	QPS              float64                     `json:"qps"`
	AvgLatency       int64                       `json:"avg_latency_ms"`
	ErrorRate        float64                     `json:"error_rate_percent"`
	Uptime           int64                       `json:"uptime_ms"`
	LastRequestTime  time.Time                   `json:"last_request_time"`
	StatusCodes      map[int]int64               `json:"status_codes"`
	ModelStats       map[string]*ModelStatsSnapshot `json:"model_stats"`
}

// ModelStatsSnapshot 模型统计快照
type ModelStatsSnapshot struct {
	TotalRequests int64   `json:"total_requests"`
	AvgLatency    int64   `json:"avg_latency_ms"`
	ErrorRequests int64   `json:"error_requests"`
	CacheHits     int64   `json:"cache_hits"`
	CacheMisses   int64   `json:"cache_misses"`
	CacheHitRate  float64 `json:"cache_hit_rate_percent"`
}
