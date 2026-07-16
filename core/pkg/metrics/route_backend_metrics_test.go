package metrics

import "testing"

func TestRouteBackendMetricsCollector_RecordAndSnapshot(t *testing.T) {
	c := newRouteBackendMetricsCollector()

	c.Record("code", "openai", true, 120)
	c.Record("code", "openai", false, 240)
	c.Record("chat", "deepseek", true, 80)

	stats := c.GetStats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 route-backend stats, got %d", len(stats))
	}

	var codeFound bool
	for _, item := range stats {
		if item.SelectedRoute == "code" && item.BackendID == "openai" {
			codeFound = true
			if item.TotalRequests != 2 {
				t.Fatalf("code/openai total_requests = %d, want 2", item.TotalRequests)
			}
			if item.SuccessCount != 1 || item.ErrorCount != 1 {
				t.Fatalf("code/openai success/error = %d/%d, want 1/1", item.SuccessCount, item.ErrorCount)
			}
			if item.AvgLatencyMs != 180 {
				t.Fatalf("code/openai avg_latency_ms = %d, want 180", item.AvgLatencyMs)
			}
			if item.SuccessRate < 49.9 || item.SuccessRate > 50.1 {
				t.Fatalf("code/openai success_rate_percent = %.2f, want around 50", item.SuccessRate)
			}
		}
	}
	if !codeFound {
		t.Fatalf("missing code/openai stats item")
	}
}

func TestRouteBackendMetricsCollector_DefaultUnknownKeys(t *testing.T) {
	c := newRouteBackendMetricsCollector()

	c.Record("", "", true, 0)
	stats := c.GetStats()
	if len(stats) != 1 {
		t.Fatalf("expected 1 stats item, got %d", len(stats))
	}
	if stats[0].SelectedRoute != "__unknown_route__" {
		t.Fatalf("selected_route = %q, want __unknown_route__", stats[0].SelectedRoute)
	}
	if stats[0].BackendID != "__unknown_backend__" {
		t.Fatalf("backend_id = %q, want __unknown_backend__", stats[0].BackendID)
	}
}

