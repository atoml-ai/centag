package tokenusage

import (
	"testing"
	"time"
)

func TestUserQuotaStatus_Fields(t *testing.T) {
	now := time.Now()
	status := &UserQuotaStatus{
		HasQuota:         true,
		DefaultPipelineID: "pipeline-default",
		DailyTokenLimit:   100000,
		MonthlyTokenLimit: 3000000,
		DailyTokenUsed:   50000,
		MonthlyTokenUsed: 1500000,
		DailyRemaining:   50000,
		MonthlyRemaining: 1500000,
	}

	if !status.HasQuota {
		t.Error("UserQuotaStatus.HasQuota should be true")
	}
	if status.DefaultPipelineID != "pipeline-default" {
		t.Errorf("UserQuotaStatus.DefaultPipelineID = %s, want pipeline-default", status.DefaultPipelineID)
	}
	if status.DailyTokenLimit != 100000 {
		t.Errorf("UserQuotaStatus.DailyTokenLimit = %d, want 100000", status.DailyTokenLimit)
	}
	if status.MonthlyTokenLimit != 3000000 {
		t.Errorf("UserQuotaStatus.MonthlyTokenLimit = %d, want 3000000", status.MonthlyTokenLimit)
	}
	if status.DailyTokenUsed != 50000 {
		t.Errorf("UserQuotaStatus.DailyTokenUsed = %d, want 50000", status.DailyTokenUsed)
	}
	if status.MonthlyTokenUsed != 1500000 {
		t.Errorf("UserQuotaStatus.MonthlyTokenUsed = %d, want 1500000", status.MonthlyTokenUsed)
	}
	if status.DailyRemaining != 50000 {
		t.Errorf("UserQuotaStatus.DailyRemaining = %d, want 50000", status.DailyRemaining)
	}
	if status.MonthlyRemaining != 1500000 {
		t.Errorf("UserQuotaStatus.MonthlyRemaining = %d, want 1500000", status.MonthlyRemaining)
	}
	_ = now // suppress unused variable warning
}

func TestUserQuotaStatus_DefaultFields(t *testing.T) {
	status := &UserQuotaStatus{}

	if status.HasQuota {
		t.Error("default HasQuota should be false")
	}
	if status.DefaultPipelineID != "" {
		t.Errorf("default DefaultPipelineID should be empty, got %s", status.DefaultPipelineID)
	}
	if status.DailyTokenLimit != 0 {
		t.Errorf("default DailyTokenLimit should be 0, got %d", status.DailyTokenLimit)
	}
	if status.MonthlyTokenLimit != 0 {
		t.Errorf("default MonthlyTokenLimit should be 0, got %d", status.MonthlyTokenLimit)
	}
	if status.DailyTokenUsed != 0 {
		t.Errorf("default DailyTokenUsed should be 0, got %d", status.DailyTokenUsed)
	}
	if status.MonthlyTokenUsed != 0 {
		t.Errorf("default MonthlyTokenUsed should be 0, got %d", status.MonthlyTokenUsed)
	}
	if status.DailyRemaining != 0 {
		t.Errorf("default DailyRemaining should be 0, got %d", status.DailyRemaining)
	}
	if status.MonthlyRemaining != 0 {
		t.Errorf("default MonthlyRemaining should be 0, got %d", status.MonthlyRemaining)
	}
}

func TestUserQuotaEntry_Fields(t *testing.T) {
	now := time.Now()
	resetDate := now.Add(30 * 24 * time.Hour)
	entry := &UserQuotaEntry{
		UserID:           100,
		DefaultPipelineID: "pipeline-coding",
		DailyTokenLimit:   200000,
		MonthlyTokenLimit: 6000000,
		DailyTokenUsed:   100000,
		MonthlyTokenUsed: 3000000,
		QuotaResetDate:   &resetDate,
	}

	if entry.UserID != 100 {
		t.Errorf("UserQuotaEntry.UserID = %d, want 100", entry.UserID)
	}
	if entry.DefaultPipelineID != "pipeline-coding" {
		t.Errorf("UserQuotaEntry.DefaultPipelineID = %s, want pipeline-coding", entry.DefaultPipelineID)
	}
	if entry.DailyTokenLimit != 200000 {
		t.Errorf("UserQuotaEntry.DailyTokenLimit = %d, want 200000", entry.DailyTokenLimit)
	}
	if entry.MonthlyTokenLimit != 6000000 {
		t.Errorf("UserQuotaEntry.MonthlyTokenLimit = %d, want 6000000", entry.MonthlyTokenLimit)
	}
	if entry.DailyTokenUsed != 100000 {
		t.Errorf("UserQuotaEntry.DailyTokenUsed = %d, want 100000", entry.DailyTokenUsed)
	}
	if entry.MonthlyTokenUsed != 3000000 {
		t.Errorf("UserQuotaEntry.MonthlyTokenUsed = %d, want 3000000", entry.MonthlyTokenUsed)
	}
	if entry.QuotaResetDate == nil {
		t.Error("UserQuotaEntry.QuotaResetDate should not be nil")
	}
}

func TestUserQuotaEntry_DefaultFields(t *testing.T) {
	entry := &UserQuotaEntry{
		UserID: 200,
	}

	if entry.DefaultPipelineID != "" {
		t.Errorf("default DefaultPipelineID should be empty, got %s", entry.DefaultPipelineID)
	}
	if entry.DailyTokenLimit != 0 {
		t.Errorf("default DailyTokenLimit should be 0, got %d", entry.DailyTokenLimit)
	}
	if entry.MonthlyTokenLimit != 0 {
		t.Errorf("default MonthlyTokenLimit should be 0, got %d", entry.MonthlyTokenLimit)
	}
	if entry.DailyTokenUsed != 0 {
		t.Errorf("default DailyTokenUsed should be 0, got %d", entry.DailyTokenUsed)
	}
	if entry.MonthlyTokenUsed != 0 {
		t.Errorf("default MonthlyTokenUsed should be 0, got %d", entry.MonthlyTokenUsed)
	}
	if entry.QuotaResetDate != nil {
		t.Error("default QuotaResetDate should be nil")
	}
}
