package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

// CacheMetrics 缓存统计
type CacheMetrics struct {
	mu sync.RWMutex

	// 基础统计
	hits        int64
	misses      int64
	sets        int64
	evictions   int64

	// 延迟统计
	totalLatency int64

	// 按策略统计
	exactHits     int64
	semanticHits  int64

	// 时间窗口统计（最近1分钟）
	recentHits    []time.Time
	recentMisses  []time.Time
	windowSize    time.Duration
	windowLimit   int

	// 创建时间
	startTime     time.Time
}

// NewCacheMetrics 创建缓存统计
func NewCacheMetrics() *CacheMetrics {
	return &CacheMetrics{
		recentHits:  make([]time.Time, 0),
		recentMisses: make([]time.Time, 0),
		windowSize:   time.Minute,
		windowLimit:  1000,
		startTime:    time.Now(),
	}
}

// RecordHit 记录命中
func (m *CacheMetrics) RecordHit(strategy string, latency time.Duration) {
	atomic.AddInt64(&m.hits, 1)
	atomic.AddInt64(&m.totalLatency, latency.Nanoseconds())

	m.mu.Lock()
	defer m.mu.Unlock()

	// 按策略统计
	if strategy == string(CacheStrategyExact) {
		m.exactHits++
	} else if strategy == string(CacheStrategySemantic) {
		m.semanticHits++
	}

	// 添加到最近窗口
	now := time.Now()
	m.recentHits = append(m.recentHits, now)
	cutoff := now.Add(-m.windowSize)
	var valid []time.Time
	for _, t := range m.recentHits {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	m.recentHits = valid

	if len(m.recentHits) > m.windowLimit {
		m.recentHits = m.recentHits[len(m.recentHits)-m.windowLimit:]
	}
}

// RecordMiss 记录未命中
func (m *CacheMetrics) RecordMiss(latency time.Duration) {
	atomic.AddInt64(&m.misses, 1)
	atomic.AddInt64(&m.totalLatency, latency.Nanoseconds())

	m.mu.Lock()
	defer m.mu.Unlock()

	// 添加到最近窗口
	now := time.Now()
	m.recentMisses = append(m.recentMisses, now)
	cutoff := now.Add(-m.windowSize)
	var valid []time.Time
	for _, t := range m.recentMisses {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	m.recentMisses = valid

	if len(m.recentMisses) > m.windowLimit {
		m.recentMisses = m.recentMisses[len(m.recentMisses)-m.windowLimit:]
	}
}

// RecordSet 记录设置
func (m *CacheMetrics) RecordSet() {
	atomic.AddInt64(&m.sets, 1)
}

// RecordEviction 记录淘汰
func (m *CacheMetrics) RecordEviction() {
	atomic.AddInt64(&m.evictions, 1)
}

// GetStats 获取统计信息
func (m *CacheMetrics) GetStats() *CacheStats {
	hits := atomic.LoadInt64(&m.hits)
	misses := atomic.LoadInt64(&m.misses)
	sets := atomic.LoadInt64(&m.sets)
	evictions := atomic.LoadInt64(&m.evictions)
	totalLatency := atomic.LoadInt64(&m.totalLatency)

	m.mu.RLock()
	defer m.mu.RUnlock()

	total := hits + misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	avgLatency := int64(0)
	if total > 0 {
		avgLatency = totalLatency / total
	}

	// 计算最近 QPS
	recentHits := len(m.recentHits)
	recentMisses := len(m.recentMisses)
	recentQPS := 0.0
	if recentHits+recentMisses > 0 && m.windowSize > 0 {
		recentQPS = float64(recentHits+recentMisses) / m.windowSize.Seconds()
	}

	strategyHitRate := make(map[string]float64)
	if hits > 0 {
		strategyHitRate[string(CacheStrategyExact)] = float64(m.exactHits) / float64(hits) * 100
		strategyHitRate[string(CacheStrategySemantic)] = float64(m.semanticHits) / float64(hits) * 100
	}

	return &CacheStats{
		Hits:              hits,
		Misses:            misses,
		Sets:              sets,
		Evictions:         evictions,
		HitRate:           hitRate,
		MissRate:          100 - hitRate,
		AvgLatency:        avgLatency,
		RecentQPS:         recentQPS,
		StrategyHitRate:   strategyHitRate,
		ExactHits:         m.exactHits,
		SemanticHits:      m.semanticHits,
		Uptime:            time.Since(m.startTime).Milliseconds(),
		TotalRequests:     total,
	}
}

// Reset 重置统计
func (m *CacheMetrics) Reset() {
	atomic.StoreInt64(&m.hits, 0)
	atomic.StoreInt64(&m.misses, 0)
	atomic.StoreInt64(&m.sets, 0)
	atomic.StoreInt64(&m.evictions, 0)
	atomic.StoreInt64(&m.totalLatency, 0)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.exactHits = 0
	m.semanticHits = 0
	m.recentHits = make([]time.Time, 0)
	m.recentMisses = make([]time.Time, 0)
	m.startTime = time.Now()
}

// CacheStats 缓存统计信息
type CacheStats struct {
	Hits            int64             `json:"hits"`
	Misses          int64             `json:"misses"`
	Sets            int64             `json:"sets"`
	Evictions       int64             `json:"evictions"`
	HitRate         float64           `json:"hit_rate_percent"`
	MissRate        float64           `json:"miss_rate_percent"`
	AvgLatency      int64             `json:"avg_latency_ns"`
	RecentQPS       float64           `json:"recent_qps"`
	StrategyHitRate map[string]float64 `json:"strategy_hit_rate_percent"`
	ExactHits       int64             `json:"exact_hits"`
	SemanticHits    int64             `json:"semantic_hits"`
	Uptime          int64             `json:"uptime_ms"`
	TotalEntries    int64             `json:"total_entries"`
	TotalRequests   int64             `json:"total_requests"`
}
