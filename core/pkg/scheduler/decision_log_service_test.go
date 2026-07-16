package scheduler

import (
	"testing"
	"time"
)

func TestDecisionLogEntry_Fields(t *testing.T) {
	now := time.Now()
	entry := &DecisionLogEntry{
		ID:        1,
		RequestID: "req-abc123",
		UserID:    100,
		TenantID:  "tenant-001",
		Model:     "gpt-4",
		Backend:   "openai",
		Strategy:  "weighted-random",
		Score:     0.85,
		Reason:    "high score due to low latency and good health",
		CreatedAt: now,
	}

	if entry.ID != 1 {
		t.Errorf("DecisionLogEntry.ID = %d, want 1", entry.ID)
	}
	if entry.RequestID != "req-abc123" {
		t.Errorf("DecisionLogEntry.RequestID = %s, want req-abc123", entry.RequestID)
	}
	if entry.UserID != 100 {
		t.Errorf("DecisionLogEntry.UserID = %d, want 100", entry.UserID)
	}
	if entry.TenantID != "tenant-001" {
		t.Errorf("DecisionLogEntry.TenantID = %s, want tenant-001", entry.TenantID)
	}
	if entry.Model != "gpt-4" {
		t.Errorf("DecisionLogEntry.Model = %s, want gpt-4", entry.Model)
	}
	if entry.Backend != "openai" {
		t.Errorf("DecisionLogEntry.Backend = %s, want openai", entry.Backend)
	}
	if entry.Strategy != "weighted-random" {
		t.Errorf("DecisionLogEntry.Strategy = %s, want weighted-random", entry.Strategy)
	}
	if entry.Score != 0.85 {
		t.Errorf("DecisionLogEntry.Score = %f, want 0.85", entry.Score)
	}
	if entry.Reason != "high score due to low latency and good health" {
		t.Errorf("DecisionLogEntry.Reason = %s, want expected reason", entry.Reason)
	}
}

func TestDecisionLogEntry_DefaultFields(t *testing.T) {
	entry := &DecisionLogEntry{
		ID:        1,
		RequestID: "req-xyz789",
		Model:     "gpt-3.5-turbo",
		Backend:   "ollama",
		Strategy:  "round-robin",
	}

	if entry.UserID != 0 {
		t.Errorf("default UserID should be 0, got %d", entry.UserID)
	}
	if entry.TenantID != "" {
		t.Errorf("default TenantID should be empty, got %s", entry.TenantID)
	}
	if entry.Score != 0 {
		t.Errorf("default Score should be 0, got %f", entry.Score)
	}
	if entry.Reason != "" {
		t.Errorf("default Reason should be empty, got %s", entry.Reason)
	}
}

func TestDecisionStats_Fields(t *testing.T) {
	stats := &DecisionStats{
		TotalDecisions: 1000,
		UniqueModels:   5,
		UniqueBackends: 3,
		AvgScore:       0.75,
	}

	if stats.TotalDecisions != 1000 {
		t.Errorf("DecisionStats.TotalDecisions = %d, want 1000", stats.TotalDecisions)
	}
	if stats.UniqueModels != 5 {
		t.Errorf("DecisionStats.UniqueModels = %d, want 5", stats.UniqueModels)
	}
	if stats.UniqueBackends != 3 {
		t.Errorf("DecisionStats.UniqueBackends = %d, want 3", stats.UniqueBackends)
	}
	if stats.AvgScore != 0.75 {
		t.Errorf("DecisionStats.AvgScore = %f, want 0.75", stats.AvgScore)
	}
}

func TestDecisionStats_DefaultFields(t *testing.T) {
	stats := &DecisionStats{}

	if stats.TotalDecisions != 0 {
		t.Errorf("default TotalDecisions should be 0, got %d", stats.TotalDecisions)
	}
	if stats.UniqueModels != 0 {
		t.Errorf("default UniqueModels should be 0, got %d", stats.UniqueModels)
	}
	if stats.UniqueBackends != 0 {
		t.Errorf("default UniqueBackends should be 0, got %d", stats.UniqueBackends)
	}
	if stats.AvgScore != 0 {
		t.Errorf("default AvgScore should be 0, got %f", stats.AvgScore)
	}
}
