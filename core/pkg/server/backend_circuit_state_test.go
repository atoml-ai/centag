package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/circuitbreaker"
	"centag/core/pkg/scheduler"

	"github.com/gin-gonic/gin"
)

// setupBreakerForTest 初始化全局熔断器（低阈值便于测试触发 open）。
func setupBreakerForTest(t *testing.T) {
	t.Helper()
	circuitbreaker.Init(scheduler.CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
		WindowDuration:   scheduler.DefaultCircuitBreakerConfig().WindowDuration,
	})
	t.Cleanup(func() {
		circuitbreaker.Reset("bk-degraded")
		circuitbreaker.Reset("bk-healthy")
	})
}

// TestListBackendsExposesCircuitState 是 P0-T2 的回归：后端列表必须暴露
// 熔断状态，管理员可在 health=healthy 而 chat 链路熔断打开时感知故障。
func TestListBackendsExposesCircuitState(t *testing.T) {
	setupBreakerForTest(t)
	gin.SetMode(gin.TestMode)

	mgr := backend.NewManager()
	_ = mgr.Add(&backend.BackendConfig{ID: "bk-degraded", Name: "degraded", Type: "openai", Enabled: true})
	_ = mgr.Add(&backend.BackendConfig{ID: "bk-healthy", Name: "healthy", Type: "openai", Enabled: true})

	h := NewBackendHandler(mgr)
	if h == nil {
		t.Fatal("NewBackendHandler() = nil")
	}

	// 触发 bk-degraded 熔断
	for i := 0; i < 3; i++ {
		circuitbreaker.RecordFailure("bk-degraded")
	}
	if !circuitbreaker.IsOpen("bk-degraded") {
		t.Fatal("precondition: bk-degraded breaker should be open")
	}

	router := gin.New()
	router.GET("/api/v1/backends", func(c *gin.Context) { h.ListBackends(c) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/backends", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list backends status = %d", w.Code)
	}

	body := w.Body.String()
	if !containsJSONField(body, `"circuit_state":"open"`) {
		t.Fatalf("open breaker must surface in list: %s", body)
	}
	// 从未触发的后端没有熔断器实例 → 无 circuit_state（unknown 语义）
	if stringContains(body, `"circuit_state":"open","weight":1,"tenant_id":"bk-healthy`) ||
		countOccurrences(body, `"circuit_state"`) != 1 {
		t.Fatalf("only the tripped backend should carry circuit_state: %s", body)
	}
}

// TestOpenCircuitBreakersAggregation 验证 open 熔断聚合列表。
func TestOpenCircuitBreakersAggregation(t *testing.T) {
	setupBreakerForTest(t)

	if got := openCircuitBreakers(); len(got) != 0 {
		t.Fatalf("no breakers should be open initially, got %v", got)
	}
	for i := 0; i < 3; i++ {
		circuitbreaker.RecordFailure("bk-degraded")
	}
	got := openCircuitBreakers()
	if len(got) != 1 || got[0] != "bk-degraded" {
		t.Fatalf("openCircuitBreakers = %v, want [bk-degraded]", got)
	}
}

func containsJSONField(body, substr string) bool {
	return len(body) > 0 && len(substr) > 0 && stringContains(body, substr)
}

func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
