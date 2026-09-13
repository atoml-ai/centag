package tokenusage

import (
	"context"
	"testing"
	"time"
)

// TestAccountStats_AccountPoolKeyMetering verifies per-key (account_id) metering:
// rows with account_id aggregate separately; NULL rows map to "" and filtering works.
func TestAccountStats_AccountPoolKeyMetering(t *testing.T) {
	svc, err := NewEphemeralService()
	if err != nil {
		t.Fatalf("ephemeral service: %v", err)
	}
	defer func() { _ = svc.db.Close() }()
	ctx := context.Background()

	records := []*UsageRecord{
		{UserID: 1, BackendID: "b1", AccountID: "key-a", Model: "m", PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, Source: "real"},
		{UserID: 1, BackendID: "b1", AccountID: "key-a", Model: "m", PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, Source: "real"},
		{UserID: 1, BackendID: "b1", AccountID: "key-b", Model: "m", PromptTokens: 100, CompletionTokens: 0, TotalTokens: 100, Source: "real"},
		{UserID: 1, BackendID: "b1", Model: "m", PromptTokens: 1, CompletionTokens: 0, TotalTokens: 1, Source: "real"}, // 无 Key（单 Key 后端）
	}
	for i, r := range records {
		c := r
		c.RequestID = string(rune('a' + i))
		if err := svc.RecordUsage(ctx, c); err != nil {
			t.Fatalf("record #%d: %v", i, err)
		}
	}

	// 未过滤：3 个 key 桶（key-a / key-b / ""）
	stats, err := svc.GetAccountStats(ctx, 1, "", 30)
	if err != nil {
		t.Fatalf("GetAccountStats: %v", err)
	}
	sum := map[string]int{}
	for _, s := range stats {
		if s.BackendID != "b1" {
			t.Fatalf("unexpected backend %q", s.BackendID)
		}
		sum[s.AccountID] += s.TotalTokens
	}
	if sum["key-a"] != 45 || sum["key-b"] != 100 || sum[""] != 1 {
		t.Fatalf("unexpected buckets: %v", sum)
	}

	// 按 key 过滤
	statsA, err := svc.GetAccountStats(ctx, 1, "key-a", 30)
	if err != nil || len(statsA) != 1 || statsA[0].TotalTokens != 45 {
		t.Fatalf("filter key-a: stats=%v err=%v", statsA, err)
	}

	// -default 过滤 NULL 行
	statsDef, err := svc.GetAccountStats(ctx, 1, "-default", 30)
	if err != nil || len(statsDef) != 1 || statsDef[0].AccountID != "" || statsDef[0].TotalTokens != 1 {
		t.Fatalf("filter -default: stats=%v err=%v", statsDef, err)
	}

	// usage breakdown 按 account 过滤
	bd, err := svc.GetUsageBreakdown(ctx, 1, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), "key-b")
	if err != nil || len(bd.Records) != 1 || bd.Summary.TotalTokens != 100 {
		t.Fatalf("breakdown filter key-b: +%v summary=%v err=%v", bd != nil, func() interface{} { if bd != nil { return bd.Summary }; return nil }(), err)
	}

	// 其他用户的行不计入
	if err := svc.RecordUsage(ctx, &UsageRecord{UserID: 2, BackendID: "b1", AccountID: "key-a", Model: "m", PromptTokens: 999, TotalTokens: 999, Source: "real"}); err != nil {
		t.Fatalf("record user2: %v", err)
	}
	statsA2, err := svc.GetAccountStats(ctx, 1, "key-a", 30)
	if err != nil || len(statsA2) != 1 || statsA2[0].TotalTokens != 45 {
		t.Fatalf("user isolation: stats=%v err=%v", statsA2, err)
	}
}
