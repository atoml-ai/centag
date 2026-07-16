package pipeline

import (
	"sort"
	"sync"
	"time"
)

// PluginMetrics 插件指标收集器
type PluginMetrics struct {
	mu sync.RWMutex

	// 调用次数
	CallCount int64 `json:"call_count"`

	// 成功/失败次数
	SuccessCount int64 `json:"success_count"`
	ErrorCount   int64 `json:"error_count"`

	// 延迟记录（纳秒），用于计算 P50/P95/P99
	latencies []time.Duration

	// 错误分布
	ErrorDistribution map[string]int64 `json:"error_distribution"`

	// 按实现统计
	ImplementationStats map[string]*PluginImplMetrics `json:"implementation_stats"`
}

// PluginImplMetrics 单个插件实现的指标
type PluginImplMetrics struct {
	CallCount         int64          `json:"call_count"`
	SuccessCount      int64          `json:"success_count"`
	ErrorCount        int64          `json:"error_count"`
	Latencies        []time.Duration `json:"-"` // 不序列化原始数据
	P95LatencyMs     int64          `json:"p95_latency_ms"`
	ErrorDistribution map[string]int64 `json:"error_distribution"`
	// 健康状态
	HealthyCount     int64          `json:"healthy_count"`
	UnhealthyCount   int64          `json:"unhealthy_count"`
	CurrentStatus    string         `json:"current_status"` // "healthy", "unhealthy", "unknown"
	LastHealthCheck  time.Time     `json:"last_health_check"`
}

// GlobalPluginMetrics 全局插件指标
var GlobalPluginMetrics *PluginMetrics

func init() {
	GlobalPluginMetrics = NewPluginMetrics()
}

// RecordCircuitState 记录熔断状态变化
func (m *PluginMetrics) RecordCircuitState(implementation string, newState string, oldState string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 按实现统计
	if implementation == "" {
		implementation = "unknown"
	}
	implStats, exists := m.ImplementationStats[implementation]
	if !exists {
		implStats = &PluginImplMetrics{}
		m.ImplementationStats[implementation] = implStats
	}

	// 记录状态变化
	if oldState != "" && oldState != newState {
		implStats.ErrorDistribution["circuit_"+oldState+"_to_"+newState]++
	}

	// 更新当前状态
	implStats.CurrentStatus = newState
}

// RecordHealthCheck 记录健康检查
func (m *PluginMetrics) RecordHealthCheck(implementation string, newStatus string, oldStatus string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++ // 复用 call count 作为健康检查计数

	// 按实现统计
	if implementation == "" {
		implementation = "unknown"
	}
	implStats, exists := m.ImplementationStats[implementation]
	if !exists {
		implStats = &PluginImplMetrics{
			Latencies:        make([]time.Duration, 0),
			ErrorDistribution: make(map[string]int64),
		}
		m.ImplementationStats[implementation] = implStats
	}

	implStats.CurrentStatus = newStatus
	implStats.LastHealthCheck = time.Now()

	if newStatus == "healthy" {
		implStats.HealthyCount++
	} else if newStatus == "unhealthy" {
		implStats.UnhealthyCount++
	}

	// 如果状态发生变化，记录到错误分布
	if oldStatus != "" && oldStatus != newStatus {
		implStats.ErrorDistribution["status_change_"+oldStatus+"_to_"+newStatus]++
	}
}

// NewPluginMetrics 创建插件指标收集器
func NewPluginMetrics() *PluginMetrics {
	return &PluginMetrics{
		latencies:          make([]time.Duration, 0),
		ErrorDistribution: make(map[string]int64),
		ImplementationStats: make(map[string]*PluginImplMetrics),
	}
}

// RecordCall 记录一次插件调用
func (m *PluginMetrics) RecordCall(implementation string, success bool, latency time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++
	m.latencies = append(m.latencies, latency)

	if success {
		m.SuccessCount++
	} else {
		m.ErrorCount++
		// 记录错误分布
		errKey := "unknown"
		if err != nil {
			errKey = err.Error()
			// 简化错误消息，避免过多唯一值
			if len(errKey) > 50 {
				errKey = errKey[:50] + "..."
			}
		}
		m.ErrorDistribution[errKey]++
	}

	// 按实现统计
	if implementation == "" {
		implementation = "unknown"
	}
	implStats, exists := m.ImplementationStats[implementation]
	if !exists {
		implStats = &PluginImplMetrics{
			Latencies:        make([]time.Duration, 0),
			ErrorDistribution: make(map[string]int64),
		}
		m.ImplementationStats[implementation] = implStats
	}
	implStats.CallCount++
	implStats.Latencies = append(implStats.Latencies, latency)
	if success {
		implStats.SuccessCount++
	} else {
		implStats.ErrorCount++
		if err != nil {
			errKey := err.Error()
			if len(errKey) > 50 {
				errKey = errKey[:50] + "..."
			}
			implStats.ErrorDistribution[errKey]++
		}
	}
}

// GetSnapshot 获取指标快照
func (m *PluginMetrics) GetSnapshot() *PluginMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := &PluginMetricsSnapshot{
		CallCount:         m.CallCount,
		SuccessCount:      m.SuccessCount,
		ErrorCount:        m.ErrorCount,
		SuccessRate:       m.calcSuccessRate(),
		P95LatencyMs:      m.calcP95LatencyMs(),
		ErrorDistribution: make(map[string]int64),
		ImplementationStats: make(map[string]*PluginImplMetricsSnapshot),
		// 健康状态汇总
		HealthyCount:    m.calcHealthyCount(),
		UnhealthyCount:  m.calcUnhealthyCount(),
	}

	// 复制错误分布
	for k, v := range m.ErrorDistribution {
		snapshot.ErrorDistribution[k] = v
	}

	// 计算各实现的快照
	for impl, stats := range m.ImplementationStats {
		snapshot.ImplementationStats[impl] = &PluginImplMetricsSnapshot{
			CallCount:         stats.CallCount,
			SuccessCount:      stats.SuccessCount,
			ErrorCount:        stats.ErrorCount,
			SuccessRate:       stats.calcSuccessRate(),
			P95LatencyMs:      stats.calcP95LatencyMs(),
			ErrorDistribution: make(map[string]int64),
			// 健康状态
			HealthyCount:    stats.HealthyCount,
			UnhealthyCount:  stats.UnhealthyCount,
			CurrentStatus:   stats.CurrentStatus,
			LastHealthCheck: stats.LastHealthCheck.UnixMilli(),
		}
		for k, v := range stats.ErrorDistribution {
			snapshot.ImplementationStats[impl].ErrorDistribution[k] = v
		}
	}

	return snapshot
}

// calcHealthyCount 计算总健康次数
func (m *PluginMetrics) calcHealthyCount() int64 {
	var total int64
	for _, stats := range m.ImplementationStats {
		total += stats.HealthyCount
	}
	return total
}

// calcUnhealthyCount 计算总不健康次数
func (m *PluginMetrics) calcUnhealthyCount() int64 {
	var total int64
	for _, stats := range m.ImplementationStats {
		total += stats.UnhealthyCount
	}
	return total
}

// calcSuccessRate 计算成功率
func (m *PluginMetrics) calcSuccessRate() float64 {
	if m.CallCount == 0 {
		return 0
	}
	return float64(m.SuccessCount) / float64(m.CallCount) * 100
}

// calcP95LatencyMs 计算 P95 延迟（毫秒）
func (m *PluginMetrics) calcP95LatencyMs() int64 {
	if len(m.latencies) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(m.latencies))
	copy(sorted, m.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	p95Index := int(float64(len(sorted)) * 0.95)
	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}
	return sorted[p95Index].Milliseconds()
}

// calcSuccessRate 计算单个实现的成功率
func (s *PluginImplMetrics) calcSuccessRate() float64 {
	if s.CallCount == 0 {
		return 0
	}
	return float64(s.SuccessCount) / float64(s.CallCount) * 100
}

// calcP95LatencyMs 计算单个实现的 P95 延迟
func (s *PluginImplMetrics) calcP95LatencyMs() int64 {
	if len(s.Latencies) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(s.Latencies))
	copy(sorted, s.Latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	p95Index := int(float64(len(sorted)) * 0.95)
	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}
	return sorted[p95Index].Milliseconds()
}

// PluginMetricsSnapshot 指标快照
type PluginMetricsSnapshot struct {
	CallCount          int64                           `json:"call_count"`
	SuccessCount       int64                           `json:"success_count"`
	ErrorCount         int64                           `json:"error_count"`
	SuccessRate        float64                         `json:"success_rate_percent"`
	P95LatencyMs      int64                           `json:"p95_latency_ms"`
	ErrorDistribution  map[string]int64               `json:"error_distribution"`
	ImplementationStats map[string]*PluginImplMetricsSnapshot `json:"implementation_stats"`
	// 健康状态汇总
	HealthyCount    int64 `json:"healthy_count"`
	UnhealthyCount  int64 `json:"unhealthy_count"`
}

// PluginImplMetricsSnapshot 单个实现指标快照
type PluginImplMetricsSnapshot struct {
	CallCount          int64           `json:"call_count"`
	SuccessCount       int64           `json:"success_count"`
	ErrorCount         int64           `json:"error_count"`
	SuccessRate        float64         `json:"success_rate_percent"`
	P95LatencyMs      int64           `json:"p95_latency_ms"`
	ErrorDistribution  map[string]int64 `json:"error_distribution"`
	// 健康状态
	HealthyCount    int64  `json:"healthy_count"`
	UnhealthyCount  int64  `json:"unhealthy_count"`
	CurrentStatus   string `json:"current_status"`
	LastHealthCheck int64  `json:"last_health_check_ms"` // Unix milliseconds
}
