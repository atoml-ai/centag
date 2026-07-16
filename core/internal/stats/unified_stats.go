package stats

import (
	"sync"
	"sync/atomic"
	"time"
)

// UnifiedStats 统一统计数据
// 确保命中+未命中+跳过=总请求数
type UnifiedStats struct {
	mu sync.RWMutex

	// 请求总数
	totalRequests int64

	// 缓存统计
	cacheHitExact     int64 // 精确匹配命中
	cacheHitSemantic  int64 // 语义匹配命中
	cacheMiss         int64 // 未命中
	cacheBypass       int64 // 跳过缓存的请求(如错误请求)

	// 时间窗口统计(最近1分钟)
	recentRequests    []RequestRecord
	windowSize        time.Duration
	windowLimit       int

	// 创建时间
	startTime         time.Time
}

// RequestRecord 请求记录
type RequestRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	CacheStatus  string    `json:"cache_status"` // HIT-EXACT, HIT-SEMANTIC, MISS, BYPASS
	Model        string    `json:"model"`
	IsStream     bool      `json:"is_stream"`
	StatusCode   int       `json:"status_code"`
}

// NewUnifiedStats 创建统一统计
func NewUnifiedStats() *UnifiedStats {
	return &UnifiedStats{
		recentRequests: make([]RequestRecord, 0),
		windowSize:     time.Minute,
		windowLimit:    1000,
		startTime:      time.Now(),
	}
}

// RecordRequest 记录请求
// cacheStatus: HIT-EXACT, HIT-SEMANTIC, MISS, BYPASS
// isStream: 是否为流式请求
// statusCode: HTTP状态码
func (s *UnifiedStats) RecordRequest(cacheStatus string, isStream bool, statusCode int, model string) {
	// 记录总请求数
	atomic.AddInt64(&s.totalRequests, 1)

	// 记录缓存状态
	switch cacheStatus {
	case "HIT-EXACT":
		atomic.AddInt64(&s.cacheHitExact, 1)
	case "HIT-SEMANTIC":
		atomic.AddInt64(&s.cacheHitSemantic, 1)
	case "MISS":
		atomic.AddInt64(&s.cacheMiss, 1)
	case "BYPASS":
		atomic.AddInt64(&s.cacheBypass, 1)
	}

	// 添加到时间窗口
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	record := RequestRecord{
		Timestamp:   now,
		CacheStatus: cacheStatus,
		Model:       model,
		IsStream:    isStream,
		StatusCode:  statusCode,
	}

	s.recentRequests = append(s.recentRequests, record)
	cutoff := now.Add(-s.windowSize)

	// 清理过期记录
	var valid []RequestRecord
	for _, r := range s.recentRequests {
		if r.Timestamp.After(cutoff) {
			valid = append(valid, r)
		}
	}
	s.recentRequests = valid

	// 限制记录数量
	if len(s.recentRequests) > s.windowLimit {
		s.recentRequests = s.recentRequests[len(s.recentRequests)-s.windowLimit:]
	}
}

// GetStats 获取统计信息
func (s *UnifiedStats) GetStats() *StatsSnapshot {
	total := atomic.LoadInt64(&s.totalRequests)
	hitExact := atomic.LoadInt64(&s.cacheHitExact)
	hitSemantic := atomic.LoadInt64(&s.cacheHitSemantic)
	miss := atomic.LoadInt64(&s.cacheMiss)
	bypass := atomic.LoadInt64(&s.cacheBypass)

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 计算命中率
	hitCount := hitExact + hitSemantic
	hitRate := float64(0)
	if hitCount+miss > 0 {
		hitRate = float64(hitCount) / float64(hitCount+miss) * 100
	}

	// 计算QPS
	recentQPS := 0.0
	if len(s.recentRequests) > 0 {
		duration := time.Since(s.recentRequests[0].Timestamp).Seconds()
		if duration > 0 {
			recentQPS = float64(len(s.recentRequests)) / duration
		}
	}

	return &StatsSnapshot{
		// 请求统计
		TotalRequests:    total,
		HitExact:         hitExact,
		HitSemantic:      hitSemantic,
		Miss:             miss,
		Bypass:           bypass,

		// 验证: HitExact + HitSemantic + Miss + Bypass == TotalRequests
		VerifiedSum:      hitExact + hitSemantic + miss + bypass,
		IsConsistent:     (hitExact + hitSemantic + miss + bypass) == total,

		// 命中率
		HitRate:          hitRate,

		// QPS
		RecentQPS:        recentQPS,

		// 时间窗口统计
		RecentCount:      int64(len(s.recentRequests)),

		// 运行时间
		Uptime:           time.Since(s.startTime).Milliseconds(),
	}
}

// StatsSnapshot 统计快照
type StatsSnapshot struct {
	// 请求统计
	TotalRequests int64   `json:"total_requests"`
	HitExact      int64   `json:"hit_exact"`
	HitSemantic   int64   `json:"hit_semantic"`
	Miss          int64   `json:"miss"`
	Bypass        int64   `json:"bypass"`

	// 验证
	VerifiedSum   int64   `json:"verified_sum"`
	IsConsistent  bool    `json:"is_consistent"`

	// 命中率
	HitRate       float64 `json:"hit_rate_percent"`

	// QPS
	RecentQPS     float64 `json:"recent_qps"`

	// 时间窗口统计
	RecentCount   int64   `json:"recent_count"`

	// 运行时间
	Uptime        int64   `json:"uptime_ms"`
}

// Reset 重置统计
func (s *UnifiedStats) Reset() {
	atomic.StoreInt64(&s.totalRequests, 0)
	atomic.StoreInt64(&s.cacheHitExact, 0)
	atomic.StoreInt64(&s.cacheHitSemantic, 0)
	atomic.StoreInt64(&s.cacheMiss, 0)
	atomic.StoreInt64(&s.cacheBypass, 0)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.recentRequests = make([]RequestRecord, 0)
	s.startTime = time.Now()
}
