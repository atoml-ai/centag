package tokenusage

import (
	"context"
	"testing"
	"time"
)

func TestNewEphemeralService_RecordAndQuery(t *testing.T) {
	svc, err := NewEphemeralService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := svc.RecordUsage(ctx, &UsageRecord{
		UserID: 1, BackendID: "b1", Model: "m1",
		PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, Success: true,
	}); err != nil {
		t.Fatal(err)
	}
	stats, err := svc.GetUserUsage(ctx, 1, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if stats == nil || stats.TotalTokens != 15 || stats.RequestCount != 1 {
		t.Fatalf("stats=%+v", stats)
	}
	models, err := svc.GetModelStats(ctx, 1, 7)
	if err != nil || len(models) != 1 || models[0].Model != "m1" {
		t.Fatalf("models=%v err=%v", models, err)
	}
}
