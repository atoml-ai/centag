package tokenusage

import (
	"testing"
)

func TestTeamQuotaStatus_Fields(t *testing.T) {
	status := &TeamQuotaStatus{
		HasQuota:          true,
		TenantID:         "tenant-001",
		DailyTokenLimit:   1000000,
		MonthlyTokenLimit: 30000000,
		DailyTokenUsed:   500000,
		MonthlyTokenUsed: 15000000,
		DailyRemaining:   500000,
		MonthlyRemaining: 15000000,
	}

	if !status.HasQuota {
		t.Error("TeamQuotaStatus.HasQuota should be true")
	}
	if status.TenantID != "tenant-001" {
		t.Errorf("TeamQuotaStatus.TenantID = %s, want tenant-001", status.TenantID)
	}
	if status.DailyTokenLimit != 1000000 {
		t.Errorf("TeamQuotaStatus.DailyTokenLimit = %d, want 1000000", status.DailyTokenLimit)
	}
	if status.MonthlyTokenLimit != 30000000 {
		t.Errorf("TeamQuotaStatus.MonthlyTokenLimit = %d, want 30000000", status.MonthlyTokenLimit)
	}
	if status.DailyTokenUsed != 500000 {
		t.Errorf("TeamQuotaStatus.DailyTokenUsed = %d, want 500000", status.DailyTokenUsed)
	}
	if status.MonthlyTokenUsed != 15000000 {
		t.Errorf("TeamQuotaStatus.MonthlyTokenUsed = %d, want 15000000", status.MonthlyTokenUsed)
	}
	if status.DailyRemaining != 500000 {
		t.Errorf("TeamQuotaStatus.DailyRemaining = %d, want 500000", status.DailyRemaining)
	}
	if status.MonthlyRemaining != 15000000 {
		t.Errorf("TeamQuotaStatus.MonthlyRemaining = %d, want 15000000", status.MonthlyRemaining)
	}
}

func TestTeamQuotaStatus_DefaultFields(t *testing.T) {
	status := &TeamQuotaStatus{}

	if status.HasQuota {
		t.Error("default HasQuota should be false")
	}
	if status.TenantID != "" {
		t.Errorf("default TenantID should be empty, got %s", status.TenantID)
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

func TestTeamQuotaCacheEntry_Fields(t *testing.T) {
	entry := &TeamQuotaCacheEntry{
		TenantID:         "tenant-002",
		DailyTokenLimit:   2000000,
		MonthlyTokenLimit: 60000000,
		DailyTokenUsed:   1000000,
		MonthlyTokenUsed: 30000000,
	}

	if entry.TenantID != "tenant-002" {
		t.Errorf("TeamQuotaCacheEntry.TenantID = %s, want tenant-002", entry.TenantID)
	}
	if entry.DailyTokenLimit != 2000000 {
		t.Errorf("TeamQuotaCacheEntry.DailyTokenLimit = %d, want 2000000", entry.DailyTokenLimit)
	}
	if entry.MonthlyTokenLimit != 60000000 {
		t.Errorf("TeamQuotaCacheEntry.MonthlyTokenLimit = %d, want 60000000", entry.MonthlyTokenLimit)
	}
	if entry.DailyTokenUsed != 1000000 {
		t.Errorf("TeamQuotaCacheEntry.DailyTokenUsed = %d, want 1000000", entry.DailyTokenUsed)
	}
	if entry.MonthlyTokenUsed != 30000000 {
		t.Errorf("TeamQuotaCacheEntry.MonthlyTokenUsed = %d, want 30000000", entry.MonthlyTokenUsed)
	}
}

func TestTeamQuotaCacheEntry_DefaultFields(t *testing.T) {
	entry := &TeamQuotaCacheEntry{
		TenantID: "tenant-003",
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
}
