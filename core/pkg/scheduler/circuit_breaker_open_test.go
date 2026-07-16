package scheduler

import "testing"

func TestCircuitBreakerManager_IsOpen(t *testing.T) {
	manager := NewCircuitBreakerManager(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          DefaultCircuitBreakerConfig().Timeout,
		WindowDuration:   DefaultCircuitBreakerConfig().WindowDuration,
	})
	if manager.IsOpen("backend-a") {
		t.Fatal("expected closed circuit initially")
	}
	manager.RecordFailure("backend-a")
	if !manager.IsOpen("backend-a") {
		t.Fatal("expected open circuit after failure")
	}
}