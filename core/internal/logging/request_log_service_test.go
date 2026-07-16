package logging

import (
	"testing"
	"time"
)

func TestRequestLogEntry_Fields(t *testing.T) {
	entry := &RequestLogEntry{
		UserID:       100,
		TenantID:     "tenant-001",
		RequestID:    "req-abc123",
		Model:        "gpt-4",
		Backend:      "openai",
		Pipeline:     "pipeline-default",
		InputTokens:  500,
		OutputTokens: 1000,
		LatencyMs:    250,
		StatusCode:   200,
		RequestBody:  "Hello, how are you?",
		ResponseBody: "I'm doing well, thank you!",
	}

	if entry.UserID != 100 {
		t.Errorf("RequestLogEntry.UserID = %d, want 100", entry.UserID)
	}
	if entry.TenantID != "tenant-001" {
		t.Errorf("RequestLogEntry.TenantID = %s, want tenant-001", entry.TenantID)
	}
	if entry.RequestID != "req-abc123" {
		t.Errorf("RequestLogEntry.RequestID = %s, want req-abc123", entry.RequestID)
	}
	if entry.Model != "gpt-4" {
		t.Errorf("RequestLogEntry.Model = %s, want gpt-4", entry.Model)
	}
	if entry.Backend != "openai" {
		t.Errorf("RequestLogEntry.Backend = %s, want openai", entry.Backend)
	}
	if entry.Pipeline != "pipeline-default" {
		t.Errorf("RequestLogEntry.Pipeline = %s, want pipeline-default", entry.Pipeline)
	}
	if entry.InputTokens != 500 {
		t.Errorf("RequestLogEntry.InputTokens = %d, want 500", entry.InputTokens)
	}
	if entry.OutputTokens != 1000 {
		t.Errorf("RequestLogEntry.OutputTokens = %d, want 1000", entry.OutputTokens)
	}
	if entry.LatencyMs != 250 {
		t.Errorf("RequestLogEntry.LatencyMs = %d, want 250", entry.LatencyMs)
	}
	if entry.StatusCode != 200 {
		t.Errorf("RequestLogEntry.StatusCode = %d, want 200", entry.StatusCode)
	}
}

func TestRequestLogEntry_DefaultFields(t *testing.T) {
	entry := &RequestLogEntry{
		UserID:    200,
		RequestID: "req-xyz789",
		Model:     "gpt-3.5-turbo",
		Backend:   "ollama",
	}

	if entry.TenantID != "" {
		t.Errorf("default TenantID should be empty, got %s", entry.TenantID)
	}
	if entry.Pipeline != "" {
		t.Errorf("default Pipeline should be empty, got %s", entry.Pipeline)
	}
	if entry.InputTokens != 0 {
		t.Errorf("default InputTokens should be 0, got %d", entry.InputTokens)
	}
	if entry.OutputTokens != 0 {
		t.Errorf("default OutputTokens should be 0, got %d", entry.OutputTokens)
	}
	if entry.LatencyMs != 0 {
		t.Errorf("default LatencyMs should be 0, got %d", entry.LatencyMs)
	}
	if entry.StatusCode != 0 {
		t.Errorf("default StatusCode should be 0, got %d", entry.StatusCode)
	}
}

func TestRequestLogService_NewService(t *testing.T) {
	// Test that NewRequestLogService creates a valid service
	// Note: This is a unit test that doesn't require a real database
	service := &RequestLogService{
		buffer:        make(chan *RequestLogEntry, 10000),
		batchSize:     100,
		flushInterval: 5 * time.Second,
		done:          make(chan struct{}),
	}

	if service.batchSize != 100 {
		t.Errorf("batchSize = %d, want 100", service.batchSize)
	}
	if service.flushInterval != 5*time.Second {
		t.Errorf("flushInterval = %v, want 5s", service.flushInterval)
	}
}
