// Package metrics provides Prometheus metrics collection for Centag.
//
// Design goals:
//   - Standard Prometheus metrics for monitoring and alerting
//   - Labels for user, team, backend, model dimensions
//   - Histogram for latency distribution
//   - Counter for requests, tokens, errors
//   - Gauge for active connections, queue depth
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ── Request Metrics ────────────────────────────────────────────────────────

	// TotalRequestsCounter counts all requests by user, team, backend, model, status
	TotalRequestsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "centag_requests_total",
			Help: "Total number of requests processed",
		},
		[]string{"user_id", "team_id", "backend", "model", "status"},
	)

	// RequestDurationHistogram measures request duration in seconds
	RequestDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "centag_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"user_id", "team_id", "backend", "model"},
	)

	// ── Token Metrics ──────────────────────────────────────────────────────────

	// TokensUsedCounter counts tokens used by user, team, backend, model
	TokensUsedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "centag_tokens_used_total",
			Help: "Total tokens used",
		},
		[]string{"user_id", "team_id", "backend", "model", "type"},
	)

	// ── Quota Metrics ──────────────────────────────────────────────────────────

	// QuotaExceededCounter counts quota exceeded events
	QuotaExceededCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "centag_quota_exceeded_total",
			Help: "Total number of quota exceeded events",
		},
		[]string{"user_id", "team_id", "quota_type"},
	)

	// QuotaUsageGauge tracks current quota usage
	QuotaUsageGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "centag_quota_usage",
			Help: "Current quota usage",
		},
		[]string{"user_id", "team_id", "quota_type"},
	)

	// ── Backend Metrics ────────────────────────────────────────────────────────

	// BackendHealthGauge tracks backend health status (1=healthy, 0=unhealthy)
	BackendHealthGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "centag_backend_health",
			Help: "Backend health status",
		},
		[]string{"backend"},
	)

	// BackendLatencyHistogram measures backend response latency
	BackendLatencyHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "centag_backend_latency_seconds",
			Help:    "Backend response latency in seconds",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"backend"},
	)

	// ── Scheduler Metrics ──────────────────────────────────────────────────────

	// SchedulerDecisionCounter counts scheduler decisions
	SchedulerDecisionCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "centag_scheduler_decisions_total",
			Help: "Total scheduler decisions",
		},
		[]string{"strategy", "reason"},
	)

	// ── System Metrics ─────────────────────────────────────────────────────────

	// ActiveConnectionsGauge tracks active connections
	ActiveConnectionsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "centag_active_connections",
			Help: "Number of active connections",
		},
	)

	// UptimeGauge tracks server uptime in seconds
	UptimeGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "centag_uptime_seconds",
			Help: "Server uptime in seconds",
		},
	)
)

// ── Helper Functions ──────────────────────────────────────────────────────────

// RecordRequest records a request with all relevant labels
func RecordRequest(userID, teamID, backend, model, status string) {
	TotalRequestsCounter.WithLabelValues(userID, teamID, backend, model, status).Inc()
}

// RecordRequestDuration records request duration
func RecordRequestDuration(userID, teamID, backend, model string, duration float64) {
	RequestDurationHistogram.WithLabelValues(userID, teamID, backend, model).Observe(duration)
}

// RecordTokens records token usage
func RecordTokens(userID, teamID, backend, model, tokenType string, count float64) {
	TokensUsedCounter.WithLabelValues(userID, teamID, backend, model, tokenType).Add(count)
}

// RecordQuotaExceeded records a quota exceeded event
func RecordQuotaExceeded(userID, teamID, quotaType string) {
	QuotaExceededCounter.WithLabelValues(userID, teamID, quotaType).Inc()
}

// SetQuotaUsage sets current quota usage
func SetQuotaUsage(userID, teamID, quotaType string, usage float64) {
	QuotaUsageGauge.WithLabelValues(userID, teamID, quotaType).Set(usage)
}

// SetBackendHealth sets backend health status
func SetBackendHealth(backend string, healthy bool) {
	if healthy {
		BackendHealthGauge.WithLabelValues(backend).Set(1)
	} else {
		BackendHealthGauge.WithLabelValues(backend).Set(0)
	}
}

// RecordBackendLatency records backend latency
func RecordBackendLatency(backend string, latency float64) {
	BackendLatencyHistogram.WithLabelValues(backend).Observe(latency)
}

// RecordSchedulerDecision records a scheduler decision
func RecordSchedulerDecision(strategy, reason string) {
	SchedulerDecisionCounter.WithLabelValues(strategy, reason).Inc()
}
