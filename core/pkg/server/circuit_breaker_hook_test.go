package server

import (
	"testing"

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
		Timeout:          scheduler.DefaultCircuitBreakerConfig().Timeout,
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
}