package scheduler

import (
	"sync"
	"time"
)

// LatencyMonitor 延迟监测器
type LatencyMonitor struct {
	mu            sync.RWMutex
	measurements  map[string]*latencyData
	windowSize    int
	checkInterval time.Duration
}

type latencyData struct {
	recentLatencies []int64
	lastCheck       time.Time
	avgLatency      int64
	trend           LatencyTrend // increasing/stable/decreasing
}

// LatencyTrend 延迟趋势
type LatencyTrend string

const (
	TrendIncreasing  LatencyTrend = "increasing"  // 延迟上升
	TrendStable      LatencyTrend = "stable"      // 延迟稳定
	TrendDecreasing  LatencyTrend = "decreasing"  // 延迟下降
)

// NewLatencyMonitor 创建延迟监测器
func NewLatencyMonitor(windowSize int) *LatencyMonitor {
	if windowSize <= 0 {
		windowSize = 100
	}
	return &LatencyMonitor{
		measurements:  make(map[string]*latencyData),
		windowSize:    windowSize,
		checkInterval: 10 * time.Second,
	}
}

// RecordLatency 记录延迟
func (m *LatencyMonitor) RecordLatency(backendID string, latencyMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, ok := m.measurements[backendID]
	if !ok {
		data = &latencyData{
			recentLatencies: make([]int64, 0, m.windowSize),
			lastCheck:       time.Now(),
		}
		m.measurements[backendID] = data
	}

	// 添加新的延迟测量
	data.recentLatencies = append(data.recentLatencies, latencyMs)
	if len(data.recentLatencies) > m.windowSize {
		data.recentLatencies = data.recentLatencies[1:]
	}

	// 更新平均延迟
	var sum int64
	for _, l := range data.recentLatencies {
		sum += l
	}
	data.avgLatency = sum / int64(len(data.recentLatencies))

	// 分析趋势
	data.trend = m.analyzeTrend(data.recentLatencies)
}

// analyzeTrend 分析延迟趋势
func (m *LatencyMonitor) analyzeTrend(latencies []int64) LatencyTrend {
	if len(latencies) < 10 {
		return TrendStable
	}

	// 比较最近 10 次和之前 10 次的平均延迟
	mid := len(latencies) / 2
	recent := latencies[mid:]
	older := latencies[:mid]

	var recentAvg, olderAvg int64
	for _, l := range recent {
		recentAvg += l
	}
	for _, l := range older {
		olderAvg += l
	}
	recentAvg /= int64(len(recent))
	olderAvg /= int64(len(older))

	// 差异超过 20% 认为有趋势
	diff := float64(recentAvg-olderAvg) / float64(olderAvg)
	if diff > 0.2 {
		return TrendIncreasing
	} else if diff < -0.2 {
		return TrendDecreasing
	}
	return TrendStable
}

// GetLatency 获取延迟信息
func (m *LatencyMonitor) GetLatency(backendID string) (avgMs int64, trend LatencyTrend, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.measurements[backendID]
	if !ok {
		return 0, TrendStable, false
	}

	return data.avgLatency, data.trend, true
}

// GetLatencyScore 获取延迟评分 (0-1, 越低延迟评分越高)
func (m *LatencyMonitor) GetLatencyScore(backendID string) float64 {
	avgMs, _, ok := m.GetLatency(backendID)
	if !ok {
		return 0.5 // 默认中等评分
	}

	// 评分标准：<100ms=1, 100-500ms=0.8-0.6, 500-1000ms=0.6-0.4, >1000ms=0.2
	if avgMs < 100 {
		return 1.0
	} else if avgMs < 500 {
		return 0.8 - float64(avgMs-100)/400*0.2
	} else if avgMs < 1000 {
		return 0.6 - float64(avgMs-500)/500*0.2
	} else if avgMs < 2000 {
		return 0.4 - float64(avgMs-1000)/1000*0.2
	}
	return 0.2
}

// GetAllLatencies 获取所有后端的延迟信息
func (m *LatencyMonitor) GetAllLatencies() map[string]LatencyInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]LatencyInfo)
	for backendID, data := range m.measurements {
		result[backendID] = LatencyInfo{
			BackendID:     backendID,
			AvgLatencyMs:  data.avgLatency,
			Trend:         data.trend,
			MeasurementCount: len(data.recentLatencies),
		}
	}
	return result
}

// LatencyInfo 延迟信息
type LatencyInfo struct {
	BackendID        string       `json:"backend_id"`
	AvgLatencyMs     int64        `json:"avg_latency_ms"`
	Trend            LatencyTrend `json:"trend"`
	MeasurementCount int          `json:"measurement_count"`
}

// IsHealthy 检查后端是否健康（延迟是否正常）
func (m *LatencyMonitor) IsHealthy(backendID string) bool {
	avgMs, trend, ok := m.GetLatency(backendID)
	if !ok {
		return true // 无数据时假设健康
	}

	// 延迟过高或持续上升认为不健康
	if avgMs > 5000 {
		return false
	}
	if trend == TrendIncreasing && avgMs > 2000 {
		return false
	}

	return true
}

// GetFastestBackend 获取延迟最低的后端
func (m *LatencyMonitor) GetFastestBackend(backendIDs []string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var fastestID string
	var fastestLatency int64 = -1

	for _, backendID := range backendIDs {
		data, ok := m.measurements[backendID]
		if !ok {
			continue
		}
		if fastestLatency < 0 || data.avgLatency < fastestLatency {
			fastestLatency = data.avgLatency
			fastestID = backendID
		}
	}

	return fastestID
}

// Reset 重置指定后端的延迟数据
func (m *LatencyMonitor) Reset(backendID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if backendID == "" {
		m.measurements = make(map[string]*latencyData)
	} else {
		delete(m.measurements, backendID)
	}
}
