package tokenusage

import (
	"context"
	"testing"
	"time"
)

func setupSQLiteCacheSavingsDB(t *testing.T) *Service {
	t.Helper()
	db := setupSQLiteTokenUsageDB(t)
	schema := `
	CREATE TABLE cache_savings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		backend_id TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		prompt_tokens INTEGER NOT NULL DEFAULT 0,
		completion_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		saved_usd REAL NOT NULL DEFAULT 0,
		cache_layer TEXT NOT NULL DEFAULT 'L1',
		tenant_id TEXT,
		dept_tag TEXT,
		request_id TEXT,
		pipeline_id TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("cache_savings schema: %v", err)
	}
	return NewService(db, "sqlite")
}

func TestRecordCacheSaving_SQLite(t *testing.T) {
	svc := setupSQLiteCacheSavingsDB(t)
	defer svc.db.Close()

	err := svc.RecordCacheSaving(context.Background(), &CacheSavingRecord{
		UserID:           1,
		BackendID:        "bigmodel",
		Model:            "glm-4-flash",
		PromptTokens:     40,
		CompletionTokens: 20,
		TotalTokens:      60,
		SavedUSD:         0.1,
		CacheLayer:       "L1",
		RequestID:        "req-cache-1",
		PipelineID:       "cache-hit",
	})
	if err != nil {
		t.Fatalf("RecordCacheSaving: %v", err)
	}

	var count int
	if err := svc.db.QueryRow(`SELECT COUNT(*) FROM cache_savings WHERE user_id = 1`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestGetCostSummary_IncludesCacheSavings(t *testing.T) {
	svc := setupSQLiteCacheSavingsDB(t)
	defer svc.db.Close()

	if err := svc.RecordCacheSaving(context.Background(), &CacheSavingRecord{
		UserID:       1,
		BackendID:    "bigmodel",
		Model:        "glm-4-flash",
		PromptTokens: 10,
		CompletionTokens: 10,
		TotalTokens:  20,
		SavedUSD:     0.42,
		CacheLayer:   "L1",
	}); err != nil {
		t.Fatalf("RecordCacheSaving: %v", err)
	}

	summary, err := svc.GetCostSummary(context.Background(), CostSummaryQuery{
		From: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("GetCostSummary: %v", err)
	}
	if summary.CacheSavedUSD < 0.41 || summary.CacheSavedUSD > 0.43 {
		t.Fatalf("CacheSavedUSD = %v, want ~0.42", summary.CacheSavedUSD)
	}
}

func TestEstimateSavedTokensFromResponse(t *testing.T) {
	raw := `{"choices":[{"message":{"content":"Paris is the capital."}}],"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`
	p, c, tot := EstimateSavedTokensFromResponse(raw, 0)
	if p != 12 || c != 8 || tot != 20 {
		t.Fatalf("usage parse: p=%d c=%d tot=%d", p, c, tot)
	}

	p2, c2, tot2 := EstimateSavedTokensFromResponse(`{"choices":[{"message":{"content":"hello world"}}]}`, 0)
	if tot2 < 1 {
		t.Fatalf("fallback estimate: p=%d c=%d tot=%d", p2, c2, tot2)
	}
}