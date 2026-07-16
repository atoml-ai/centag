package server

import (
	"testing"
	"time"
)

func TestSchedulerDecisionResponse(t *testing.T) {
	// Test schedulerDecisionResponse structure
	resp := &schedulerDecisionResponse{
		ID:        1,
		RequestID: "req-123",
		UserID:    1,
		TeamID:    "team-1",
		Model:     "gpt-4",
		Backend:   "openai",
		Strategy:  "round-robin",
		Score:     0.85,
		Reason:    "selected",
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if resp.ID != 1 {
		t.Errorf("expected ID 1, got %d", resp.ID)
	}
	if resp.RequestID != "req-123" {
		t.Errorf("expected RequestID req-123, got %s", resp.RequestID)
	}
	if resp.Model != "gpt-4" {
		t.Errorf("expected Model gpt-4, got %s", resp.Model)
	}
}

func TestSchedulerDecisionStatsResponse(t *testing.T) {
	// Test schedulerDecisionStatsResponse structure
	resp := &schedulerDecisionStatsResponse{
		TotalDecisions: 100,
		UniqueModels:   5,
		UniqueBackends: 3,
		AvgScore:       0.75,
	}

	if resp.TotalDecisions != 100 {
		t.Errorf("expected TotalDecisions 100, got %d", resp.TotalDecisions)
	}
	if resp.UniqueModels != 5 {
		t.Errorf("expected UniqueModels 5, got %d", resp.UniqueModels)
	}
	if resp.AvgScore != 0.75 {
		t.Errorf("expected AvgScore 0.75, got %f", resp.AvgScore)
	}
}

func TestNewSchedulerHandler(t *testing.T) {
	// Test that NewSchedulerHandler returns a valid handler
	// Note: This requires a real DecisionLogService, so we test with nil
	// In a real test, you would mock the service
	handler := NewSchedulerHandler(nil)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
