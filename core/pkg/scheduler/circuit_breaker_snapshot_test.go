package scheduler

import (
	"testing"
	"time"
)

// TestCircuitBreakerSnapshot 验证快照字段：closed → open 迁移、失败计数、open_since。
func TestCircuitBreakerSnapshot(t *testing.T) {
	cb := NewCircuitBreaker("snap-backend", CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		WindowDuration:   1 * time.Minute,
	})

	snap := cb.Snapshot()
	if snap.State != string(StateClosed) || snap.IsOpen {
		t.Fatalf("initial state = %s, want closed", snap.State)
	}
	if snap.FailureThreshold != 3 || snap.TimeoutSec != 30 {
		t.Fatalf("config snapshot mismatch: threshold=%d timeout=%ds", snap.FailureThreshold, snap.TimeoutSec)
	}

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	snap = cb.Snapshot()
	if snap.State != string(StateOpen) || !snap.IsOpen {
		t.Fatalf("state after 3 failures = %s, want open", snap.State)
	}
	if snap.ConsecutiveFailures != 3 || snap.FailureCount != 3 {
		t.Fatalf("failure counts = %d/%d, want 3/3", snap.ConsecutiveFailures, snap.FailureCount)
	}
	if snap.OpenSince == nil {
		t.Fatal("open_since should be set when state is open")
	}
	if snap.LastFailureAt == nil {
		t.Fatal("last_failure_at should be set after failure")
	}
}

// TestManagerGetAllDetailedStates 验证管理器返回全部后端快照。
func TestManagerGetAllDetailedStates(t *testing.T) {
	m := NewCircuitBreakerManager(DefaultCircuitBreakerConfig())
	m.RecordFailure("b1")
	m.RecordSuccess("b2")

	snaps := m.GetAllDetailedStates()
	if len(snaps) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(snaps))
	}
	byID := make(map[string]CircuitBreakerSnapshot, len(snaps))
	for _, s := range snaps {
		byID[s.BackendID] = s
	}
	if s, ok := byID["b1"]; !ok || s.ConsecutiveFailures != 1 {
		t.Fatalf("b1 snapshot missing or wrong failures: %+v", s)
	}
	if s, ok := byID["b2"]; !ok || s.SuccessCount != 1 {
		t.Fatalf("b2 snapshot missing or wrong successes: %+v", s)
	}
}
