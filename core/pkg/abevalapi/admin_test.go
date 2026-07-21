package abevalapi

import (
	"context"
	"testing"
	"time"
)

func TestWrap_NilServiceUnavailable(t *testing.T) {
	svc := Wrap(nil)
	ctx := context.Background()
	from := time.Now().Add(-time.Hour)
	to := time.Now()

	if _, err := svc.ListResults(ctx, from, to, 10); err == nil {
		t.Fatal("expected unavailable error from ListResults")
	}
	if _, err := svc.GetSummary(ctx, from, to); err == nil {
		t.Fatal("expected unavailable error from GetSummary")
	}
}
