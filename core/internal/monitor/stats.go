package monitor

import (
	"sync"
	"sync/atomic"
	"time"
)

// Stats 统计信息
type Stats struct {
	TotalRequests uint64
	CacheHits     uint64
	TotalLatency  uint64 // 纳秒
}

// Monitor 监控器
type Monitor struct {
	stats Stats
}

var (
	instance *Monitor
	once     sync.Once
)

// GetMonitor 获取监控器单例
func GetMonitor() *Monitor {
	once.Do(func() {
		instance = &Monitor{
			stats: Stats{},
		}
	})
	return instance
}

// RecordRequest 记录请求
func (m *Monitor) RecordRequest(latency time.Duration) {
	atomic.AddUint64(&m.stats.TotalRequests, 1)
	atomic.AddUint64(&m.stats.TotalLatency, uint64(latency))
}

// RecordCacheHit 记录缓存命中
func (m *Monitor) RecordCacheHit() {
	atomic.AddUint64(&m.stats.CacheHits, 1)
}

// GetStats 获取统计信息
func (m *Monitor) GetStats() Stats {
	return Stats{
		TotalRequests: atomic.LoadUint64(&m.stats.TotalRequests),
		CacheHits:     atomic.LoadUint64(&m.stats.CacheHits),
		TotalLatency:  atomic.LoadUint64(&m.stats.TotalLatency),
	}
}

// GetAvgLatency 获取平均延迟(毫秒)
func (m *Monitor) GetAvgLatency() float64 {
	total := atomic.LoadUint64(&m.stats.TotalRequests)
	if total == 0 {
		return 0
	}
	latencyNs := atomic.LoadUint64(&m.stats.TotalLatency)
	return float64(latencyNs) / float64(total) / 1e6
}

// GetCacheHitRate 获取缓存命中率
func (m *Monitor) GetCacheHitRate() float64 {
	total := atomic.LoadUint64(&m.stats.TotalRequests)
	if total == 0 {
		return 0
	}
	hits := atomic.LoadUint64(&m.stats.CacheHits)
	return float64(hits) / float64(total) * 100
}

// Reset 重置统计信息
func (m *Monitor) Reset() {
	atomic.StoreUint64(&m.stats.TotalRequests, 0)
	atomic.StoreUint64(&m.stats.CacheHits, 0)
	atomic.StoreUint64(&m.stats.TotalLatency, 0)
}
