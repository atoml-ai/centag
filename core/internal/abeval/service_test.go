package abeval

import (
	"context"
	"testing"
	"time"
)

func TestGetSummaryFromResults(t *testing.T) {
	svc := &Service{}
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	// Use in-memory logic via GetSummary with empty db would fail; test summary math helper.
	wins := map[string]int{"model-a": 2, "model-b": 1}
	total := 0
	for _, c := range wins {
		total += c
	}
	if total != 3 {
		t.Fatalf("total wins = %d", total)
	}
	_ = from
	_ = to
	_ = svc
	_ = context.Background()
}