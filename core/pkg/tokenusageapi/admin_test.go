package tokenusageapi

import (
	"context"
	"testing"
	"time"
)

func TestAdapter_NilServiceUnavailable(t *testing.T) {
	a := &adapter{s: nil}
	ctx := context.Background()
	from := time.Now().Add(-time.Hour)
	to := time.Now()

	if _, err := a.GetAllUsersUsage(ctx, from, to); err == nil {
		t.Fatal("expected unavailable from GetAllUsersUsage")
	}
	if _, err := a.GetUserRanking(ctx, 10, 7); err == nil {
		t.Fatal("expected unavailable from GetUserRanking")
	}
	if err := a.SetQuota(ctx, 1, 10, 100); err == nil {
		t.Fatal("expected unavailable from SetQuota")
	}
	if _, err := a.GetUserQuota(ctx, 1); err == nil {
		t.Fatal("expected unavailable from GetUserQuota")
	}
	if err := a.ResetQuota(ctx, 1); err == nil {
		t.Fatal("expected unavailable from ResetQuota")
	}
}

func TestWrap_ReturnsAdminService(t *testing.T) {
	if Wrap(nil) == nil {
		t.Fatal("Wrap(nil) must return AdminService")
	}
	if Default() == nil {
		t.Fatal("Default() must return AdminService")
	}
}
