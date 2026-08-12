package tokenusage

import (
	"context"
	"testing"
	"time"
)

func TestGetUsageBreakdown_SQLite(t *testing.T) {
	db := setupSQLiteTokenUsageDB(t)
	defer db.Close()

	svc := NewService(db, "sqlite")
	ctx := context.Background()

	records := []*UsageRecord{
		{
			UserID: 1, BackendID: "deepseek", Model: "deepseek-v4-flash",
			PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120,
			RequestID: "req-1", Success: true,
			CostInputPrice: 0.27, CostOutputPrice: 1.1,
			InputCost: 0.027, OutputCost: 0.022, CostUSD: 0.049,
		},
		{
			UserID: 1, BackendID: "deepseek", Model: "deepseek-v4-flash",
			PromptTokens: 200, CompletionTokens: 40, TotalTokens: 240,
			RequestID: "req-2", Success: true,
			CostInputPrice: 0.27, CostOutputPrice: 1.1,
			InputCost: 0.054, OutputCost: 0.044, CostUSD: 0.098,
		},
		{
			UserID: 1, BackendID: "openrouter", Model: "claude-3.5-haiku",
			PromptTokens: 50, CompletionTokens: 50, TotalTokens: 100,
			RequestID: "req-3", Success: true,
			CostInputPrice: 0.8, CostOutputPrice: 4.0,
			InputCost: 0.04, OutputCost: 0.2, CostUSD: 0.24,
		},
		{
			UserID: 1, BackendID: "deepseek", Model: "deepseek-v4-flash",
			PromptTokens: 0, CompletionTokens: 0, TotalTokens: 0,
			RequestID: "req-failed", Success: false,
		},
		{
			UserID: 2, BackendID: "deepseek", Model: "deepseek-v4-flash",
			PromptTokens: 999, CompletionTokens: 999, TotalTokens: 1998,
			RequestID: "req-other", Success: true,
			CostInputPrice: 0.27, CostOutputPrice: 1.1,
			InputCost: 0.27, OutputCost: 1.1, CostUSD: 1.37,
		},
	}
	for _, r := range records {
		if err := svc.RecordUsage(ctx, r); err != nil {
			t.Fatalf("RecordUsage %s: %v", r.RequestID, err)
		}
	}

	from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	bd, err := svc.GetUsageBreakdown(ctx, 1, from, to)
	if err != nil {
		t.Fatalf("GetUsageBreakdown: %v", err)
	}

	if len(bd.Records) != 2 {
		t.Fatalf("records = %d, want 2 (user scoped + success only)", len(bd.Records))
	}

	// Highest cost first: openrouter 0.24 > deepseek 0.147
	if bd.Records[0].BackendID != "openrouter" || bd.Records[0].Model != "claude-3.5-haiku" {
		t.Fatalf("top record = %s/%s, want openrouter/claude-3.5-haiku", bd.Records[0].BackendID, bd.Records[0].Model)
	}
	if bd.Records[0].RequestCount != 1 {
		t.Fatalf("top request_count = %d, want 1", bd.Records[0].RequestCount)
	}
	if bd.Records[0].CostInputPrice != 0.8 || bd.Records[0].CostOutputPrice != 4.0 {
		t.Fatalf("top prices = %v/%v, want 0.8/4.0", bd.Records[0].CostInputPrice, bd.Records[0].CostOutputPrice)
	}

	ds := bd.Records[1]
	if ds.BackendID != "deepseek" || ds.Model != "deepseek-v4-flash" {
		t.Fatalf("second record = %s/%s, want deepseek/deepseek-v4-flash", ds.BackendID, ds.Model)
	}
	if ds.RequestCount != 2 {
		t.Fatalf("deepseek request_count = %d, want 2 (failed row excluded)", ds.RequestCount)
	}
	if ds.InputTokens != 300 || ds.OutputTokens != 60 || ds.TotalTokens != 360 {
		t.Fatalf("deepseek tokens = %d/%d/%d, want 300/60/360", ds.InputTokens, ds.OutputTokens, ds.TotalTokens)
	}
	if abs(ds.TotalCost-0.147) > 1e-9 {
		t.Fatalf("deepseek total_cost = %v, want 0.147", ds.TotalCost)
	}

	if bd.Summary.TotalTokens != 460 {
		t.Fatalf("summary total_tokens = %d, want 460", bd.Summary.TotalTokens)
	}
	if abs(bd.Summary.TotalCost-0.387) > 1e-9 {
		t.Fatalf("summary total_cost = %v, want 0.387", bd.Summary.TotalCost)
	}
}
