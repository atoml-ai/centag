package server

import (
	"testing"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/scheduler"
)

func TestWireCircuitBreaker(t *testing.T) {
	// Defensive: wireCircuitBreaker logs; do not rely solely on TestMain ordering.
	if err := logger.Init(logger.Config{Level: "error", Format: "console", Output: "stdout"}); err != nil {
		t.Fatalf("logger.Init: %v", err)
	}
	cbManager := scheduler.NewCircuitBreakerManager(scheduler.CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          20 * time.Millisecond,
		WindowDuration:   scheduler.DefaultCircuitBreakerConfig().WindowDuration,
	})
	wireCircuitBreaker(cbManager)
	defer func() {
		pipeline.IsCircuitOpen = nil
		pipeline.RecordCircuitOutcome = nil
	}()

	if pipeline.IsCircuitOpen == nil || pipeline.RecordCircuitOutcome == nil {
		t.Fatal("circuit breaker hooks not wired")
	}

	pipeline.RecordCircuitOutcome("test-backend", false)
	if !pipeline.IsCircuitOpen("test-backend") {
		t.Fatal("expected circuit to open after recorded failure")
	}
	// Allow() 驱动半开：超时后 IsCircuitOpen 必须放行探测，不能永久挡住。
	time.Sleep(40 * time.Millisecond)
	if pipeline.IsCircuitOpen("test-backend") {
		t.Fatal("expected half-open probe after timeout (IsCircuitOpen should use Allow)")
	}
}
