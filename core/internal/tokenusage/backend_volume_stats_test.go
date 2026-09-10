package tokenusage

import (
	"context"
	"testing"
	"time"
)

// TestGetRequestVolumeByBackend_Aggregation 覆盖分桶聚合与失败计数（需求 A-T3）。
func TestGetRequestVolumeByBackend_Aggregation(t *testing.T) {
	db := setupSQLiteTokenUsageDB(t)
	svc := NewService(db, "sqlite")
	ctx := context.Background()

	past2h := time.Now().UTC().Add(-2 * time.Hour).Format(tsLayout)
	past48h := time.Now().UTC().Add(-48 * time.Hour).Format(tsLayout)
	if _, err := db.Exec(`INSERT INTO token_usage (user_id, backend_id, model, total_tokens, success, created_at) VALUES
		(1, 'b1', 'm', 10, 1, ?1),
		(1, 'b1', 'm', 10, 1, ?1),
		(1, 'b1', 'm', 0, 0, ?1),
		(1, 'b2', 'm', 5, 1, ?1),
		(1, 'b2', 'm', 10, 1, ?2)`, past2h, past48h); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stats, err := svc.GetRequestVolumeByBackend(ctx, 24)
	if err != nil {
		t.Fatal(err)
	}
	byBackend := map[string]BackendVolumeStats{}
	for _, s := range stats {
		byBackend[s.BackendID] = s
	}
	b1, ok := byBackend["b1"]
	if !ok || b1.Requests != 3 || b1.Failed != 1 {
		t.Fatalf("b1 aggregation unexpected: %+v", b1)
	}
	b2, ok := byBackend["b2"]
	if !ok || b2.Requests != 1 || b2.Failed != 0 {
		t.Fatalf("b2 aggregation unexpected: %+v", b2)
	}
}

// TestGetRequestVolumeByBackend_Guard hours<1 拒绝（护栏前置，不执行 SQL）。
func TestGetRequestVolumeByBackend_Guard(t *testing.T) {
	db := setupSQLiteTokenUsageDB(t)
	svc := NewService(db, "sqlite")
	if _, err := svc.GetRequestVolumeByBackend(context.Background(), 0); err == nil {
		t.Fatal("expected error for hours=0")
	}

	svcPG := NewService(nil, "postgresql")
	if _, err := svcPG.GetRequestVolumeByBackend(context.Background(), 24); err == nil {
		t.Fatal("expected error for nil db")
	}
}

const tsLayout = "2006-01-02 15:04:05"
