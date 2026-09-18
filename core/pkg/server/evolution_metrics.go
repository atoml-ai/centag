package server

import (
	"context"
	"math"

	"centag/core/internal/agent/evolution"
	"centag/core/internal/monitor"
	"centag/core/pkg/metrics"
)

// hostMetricSource 生产最小可观测面（P0-2，决策 A：近似估算）。
//
// 说明：v0.3.5 不搭沙盒回放，指标口径由现网聚合量近似：
//   - error_rate：全局错误率（比例，0~1）
//   - p95_latency_ms：以全局平均延迟 ×1.5 近似 p95
//
// 这是"最小可观测面"，阈值守卫（MetricThresholds）依然生效；精确回放列入后续版本。
type hostMetricSource struct{}

// Snapshot 取当前窗口指标基线。
func (hostMetricSource) Snapshot(_ context.Context) (map[string]float64, error) {
	return currentHostMetrics(), nil
}

// Measure 以确定性启发式估算提案变更后的指标（非真实回放）。
func (hostMetricSource) Measure(_ context.Context, p *evolution.Proposal) (map[string]float64, error) {
	m := currentHostMetrics()
	applyEffectHeuristic(m, p)
	return m, nil
}

// currentHostMetrics 汇总全局指标；无流量时返回零值（dryrun 仍可裁决）。
func currentHostMetrics() map[string]float64 {
	m := map[string]float64{"error_rate": 0, "p95_latency_ms": 0}

	if metrics.GlobalMetrics != nil {
		s := metrics.GlobalMetrics.GetStats()
		if s != nil && s.TotalRequests > 0 {
			m["error_rate"] = s.ErrorRate / 100.0
			if s.AvgLatency > 0 {
				m["p95_latency_ms"] = float64(s.AvgLatency) * 1.5
			}
		}
	}

	if m["p95_latency_ms"] == 0 {
		if mon := monitor.GetMonitor(); mon != nil {
			if avg := mon.GetAvgLatency(); avg > 0 {
				m["p95_latency_ms"] = avg * 1.5
			}
		}
	}
	return m
}

// applyEffectHeuristic 依提案目标做方向性估算。
func applyEffectHeuristic(m map[string]float64, p *evolution.Proposal) {
	if p == nil {
		return
	}
	switch p.Target {
	case evolution.TargetRetryPolicy:
		n, _ := toFloat(p.Params["max_retries"])
		extra := n - 3 // 以 3 次为基线
		if extra > 0 {
			m["p95_latency_ms"] *= 1 + 0.05*extra
			m["error_rate"] *= math.Pow(0.7, extra)
		}
	case evolution.TargetCacheTTL:
		// TTL 延长 → 命中率上升 → 延迟略降（方向性近似）。
		m["p95_latency_ms"] *= 0.98
	case evolution.TargetPipelineWeight, evolution.TargetBackendSwitch:
		// 权重/后端切换的延迟影响依赖具体拓扑，最小面不做方向性假设。
	}
}
