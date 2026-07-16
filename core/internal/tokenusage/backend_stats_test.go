package tokenusage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackendStats_Fields(t *testing.T) {
	successRate := 0.95
	stats := &BackendStats{
		BackendID:    "openai-primary",
		TotalTokens:  1000000,
		RequestCount: 500,
		SuccessRate:  &successRate,
	}

	assert.Equal(t, "openai-primary", stats.BackendID)
	assert.Equal(t, 1000000, stats.TotalTokens)
	assert.Equal(t, 500, stats.RequestCount)
	assert.NotNil(t, stats.SuccessRate)
	assert.Equal(t, 0.95, *stats.SuccessRate)
}

func TestBackendStats_NilSuccessRate(t *testing.T) {
	stats := &BackendStats{
		BackendID:    "ollama-local",
		TotalTokens:  50000,
		RequestCount: 100,
		SuccessRate:  nil,
	}

	assert.Nil(t, stats.SuccessRate)
}

func TestBackendStats_ZeroValues(t *testing.T) {
	stats := &BackendStats{}

	assert.Equal(t, "", stats.BackendID)
	assert.Equal(t, 0, stats.TotalTokens)
	assert.Equal(t, 0, stats.RequestCount)
	assert.Nil(t, stats.SuccessRate)
}

func TestUsageStats_Fields(t *testing.T) {
	stats := &UsageStats{
		TotalPromptTokens:     100,
		TotalCompletionTokens: 200,
		TotalTokens:           300,
		RequestCount:          10,
	}

	assert.Equal(t, 100, stats.TotalPromptTokens)
	assert.Equal(t, 200, stats.TotalCompletionTokens)
	assert.Equal(t, 300, stats.TotalTokens)
	assert.Equal(t, 10, stats.RequestCount)
}

func TestDailyStats_Fields(t *testing.T) {
	stats := &DailyStats{
		Date:         "2026-07-13",
		TotalTokens:  50000,
		PromptTokens: 30000,
		CompTokens:   20000,
		RequestCount: 25,
		UniqueUsers:  5,
		UniqueModels: 3,
	}

	assert.Equal(t, "2026-07-13", stats.Date)
	assert.Equal(t, 50000, stats.TotalTokens)
	assert.Equal(t, 30000, stats.PromptTokens)
	assert.Equal(t, 20000, stats.CompTokens)
	assert.Equal(t, 25, stats.RequestCount)
	assert.Equal(t, 5, stats.UniqueUsers)
	assert.Equal(t, 3, stats.UniqueModels)
}

func TestModelStats_Fields(t *testing.T) {
	stats := &ModelStats{
		Model:        "gpt-4",
		TotalTokens:  200000,
		RequestCount: 100,
		AvgTokens:    2000.0,
	}

	assert.Equal(t, "gpt-4", stats.Model)
	assert.Equal(t, 200000, stats.TotalTokens)
	assert.Equal(t, 100, stats.RequestCount)
	assert.Equal(t, 2000.0, stats.AvgTokens)
}

func TestUsageRecord_Fields(t *testing.T) {
	record := &UsageRecord{
		UserID:           1,
		APIKeyID:         10,
		BackendID:        "openai-primary",
		Model:            "gpt-4",
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
		CostUSD:          0.015,
		Success:          true,
		TenantID:         "t_1",
		DeptTag:          "engineering",
		RequestID:        "req-abc",
		ClientIP:         "127.0.0.1",
		AgentType:        "coding",
	}

	assert.Equal(t, int64(1), record.UserID)
	assert.Equal(t, int64(10), record.APIKeyID)
	assert.Equal(t, "openai-primary", record.BackendID)
	assert.Equal(t, "gpt-4", record.Model)
	assert.Equal(t, 100, record.PromptTokens)
	assert.Equal(t, 200, record.CompletionTokens)
	assert.Equal(t, 300, record.TotalTokens)
	assert.InDelta(t, 0.015, record.CostUSD, 0.001)
	assert.True(t, record.Success)
	assert.Equal(t, "t_1", record.TenantID)
	assert.Equal(t, "engineering", record.DeptTag)
	assert.Equal(t, "req-abc", record.RequestID)
	assert.Equal(t, "127.0.0.1", record.ClientIP)
	assert.Equal(t, "coding", record.AgentType)
}
