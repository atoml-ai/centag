package metrics

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// RouteBackendMetricsCollector collects metrics grouped by selected_route + backend_id.
type RouteBackendMetricsCollector struct {
	mu    sync.RWMutex
	stats map[string]*RouteBackendStat
}

// RouteBackendStat stores aggregate stats for one route-backend pair.
type RouteBackendStat struct {
	SelectedRoute string        `json:"selected_route"`
	BackendID     string        `json:"backend_id"`
	TotalRequests int64         `json:"total_requests"`
	SuccessCount  int64         `json:"success_count"`
	ErrorCount    int64         `json:"error_count"`
	TotalLatency  time.Duration `json:"-"`
	LastUpdatedAt time.Time     `json:"last_updated_at"`
}

// RouteBackendStatsSnapshot is a serializable view for API responses.
type RouteBackendStatsSnapshot struct {
	SelectedRoute string  `json:"selected_route"`
	BackendID     string  `json:"backend_id"`
	TotalRequests int64   `json:"total_requests"`
	SuccessCount  int64   `json:"success_count"`
	ErrorCount    int64   `json:"error_count"`
	SuccessRate   float64 `json:"success_rate_percent"`
	AvgLatencyMs  int64   `json:"avg_latency_ms"`
	LastUpdatedAt string  `json:"last_updated_at"`
}

// GlobalRouteBackendMetrics is the global singleton collector.
var GlobalRouteBackendMetrics *RouteBackendMetricsCollector

func newRouteBackendMetricsCollector() *RouteBackendMetricsCollector {
	return &RouteBackendMetricsCollector{
		stats: make(map[string]*RouteBackendStat),
	}
}

// Record registers one request outcome.
func (c *RouteBackendMetricsCollector) Record(selectedRoute, backendID string, success bool, latencyMs int64) {
	if c == nil {
		return
	}
	route := selectedRoute
	if route == "" {
		route = "__unknown_route__"
	}
	backend := backendID
	if backend == "" {
		backend = "__unknown_backend__"
	}
	key := fmt.Sprintf("%s::%s", route, backend)

	c.mu.Lock()
	defer c.mu.Unlock()

	stat, ok := c.stats[key]
	if !ok {
		stat = &RouteBackendStat{
			SelectedRoute: route,
			BackendID:     backend,
		}
		c.stats[key] = stat
	}
	stat.TotalRequests++
	if success {
		stat.SuccessCount++
	} else {
		stat.ErrorCount++
	}
	if latencyMs > 0 {
		stat.TotalLatency += time.Duration(latencyMs) * time.Millisecond
	}
	stat.LastUpdatedAt = time.Now()
}

// GetStats returns sorted snapshots.
func (c *RouteBackendMetricsCollector) GetStats() []RouteBackendStatsSnapshot {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]RouteBackendStatsSnapshot, 0, len(c.stats))
	for _, s := range c.stats {
		successRate := float64(0)
		avgLatencyMs := int64(0)
		if s.TotalRequests > 0 {
			successRate = float64(s.SuccessCount) / float64(s.TotalRequests) * 100
			avgLatencyMs = (s.TotalLatency / time.Duration(s.TotalRequests)).Milliseconds()
		}
		out = append(out, RouteBackendStatsSnapshot{
			SelectedRoute: s.SelectedRoute,
			BackendID:     s.BackendID,
			TotalRequests: s.TotalRequests,
			SuccessCount:  s.SuccessCount,
			ErrorCount:    s.ErrorCount,
			SuccessRate:   successRate,
			AvgLatencyMs:  avgLatencyMs,
			LastUpdatedAt: s.LastUpdatedAt.Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalRequests != out[j].TotalRequests {
			return out[i].TotalRequests > out[j].TotalRequests
		}
		if out[i].SelectedRoute != out[j].SelectedRoute {
			return out[i].SelectedRoute < out[j].SelectedRoute
		}
		return out[i].BackendID < out[j].BackendID
	})
	return out
}

// Reset clears all accumulated stats.
func (c *RouteBackendMetricsCollector) Reset() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats = make(map[string]*RouteBackendStat)
}

