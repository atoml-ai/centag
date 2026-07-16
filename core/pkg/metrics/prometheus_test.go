package metrics

import (
	"testing"
)

func TestRecordRequest(t *testing.T) {
	// Test that RecordRequest doesn't panic
	RecordRequest("user1", "team1", "openai", "gpt-4", "200")
	RecordRequest("user1", "team1", "openai", "gpt-4", "500")
	RecordRequest("user1", "team1", "openai", "gpt-4", "200")
}

func TestRecordRequestDuration(t *testing.T) {
	// Test that RecordRequestDuration doesn't panic
	RecordRequestDuration("user1", "team1", "openai", "gpt-4", 0.1)
	RecordRequestDuration("user1", "team1", "openai", "gpt-4", 1.5)
}

func TestRecordTokens(t *testing.T) {
	// Test that RecordTokens doesn't panic
	RecordTokens("user1", "team1", "openai", "gpt-4", "input", 100)
	RecordTokens("user1", "team1", "openai", "gpt-4", "output", 50)
}

func TestRecordQuotaExceeded(t *testing.T) {
	// Test that RecordQuotaExceeded doesn't panic
	RecordQuotaExceeded("user1", "team1", "daily_token")
	RecordQuotaExceeded("user1", "team1", "monthly_token")
}

func TestSetQuotaUsage(t *testing.T) {
	// Test that SetQuotaUsage doesn't panic
	SetQuotaUsage("user1", "team1", "daily_token", 1000)
	SetQuotaUsage("user1", "team1", "monthly_token", 50000)
}

func TestSetBackendHealth(t *testing.T) {
	// Test that SetBackendHealth doesn't panic
	SetBackendHealth("openai", true)
	SetBackendHealth("anthropic", false)
}

func TestRecordBackendLatency(t *testing.T) {
	// Test that RecordBackendLatency doesn't panic
	RecordBackendLatency("openai", 0.1)
	RecordBackendLatency("anthropic", 0.5)
}

func TestRecordSchedulerDecision(t *testing.T) {
	// Test that RecordSchedulerDecision doesn't panic
	RecordSchedulerDecision("round-robin", "success")
	RecordSchedulerDecision("least-latency", "fallback")
}
