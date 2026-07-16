package auth

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryTokenBucket_Unlimited(t *testing.T) {
	b := newMemoryTokenBucket()
	ok, rpm, tpm, err := b.Allow(context.Background(), "test", 0, 0)
	if !ok {
		t.Error("unlimited (rpm=0, tpm=0) should always allow")
	}
	if rpm != 0 || tpm != 0 {
		t.Errorf("remaining should be 0 for unlimited, got rpm=%d tpm=%d", rpm, tpm)
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMemoryTokenBucket_BasicRPM(t *testing.T) {
	b := newMemoryTokenBucket()

	// Allow 3 requests with RPM = 2
	ok1, _, _, _ := b.Allow(context.Background(), "test-rpm", 2, 0)
	if !ok1 {
		t.Error("first request should be allowed")
	}

	ok2, _, _, _ := b.Allow(context.Background(), "test-rpm", 2, 0)
	if !ok2 {
		t.Error("second request should be allowed")
	}

	ok3, _, _, _ := b.Allow(context.Background(), "test-rpm", 2, 0)
	if ok3 {
		t.Error("third request should be rate limited (rpm=2)")
	}
}

func TestMemoryTokenBucket_BasicTPM(t *testing.T) {
	b := newMemoryTokenBucket()

	ok1, _, _, _ := b.Allow(context.Background(), "test-tpm", 0, 3)
	if !ok1 {
		t.Error("first request should be allowed")
	}

	ok2, _, _, _ := b.Allow(context.Background(), "test-tpm", 0, 3)
	if !ok2 {
		t.Error("second request should be allowed")
	}

	ok3, _, _, _ := b.Allow(context.Background(), "test-tpm", 0, 3)
	if !ok3 {
		t.Error("third request should be allowed")
	}

	ok4, _, _, _ := b.Allow(context.Background(), "test-tpm", 0, 3)
	if ok4 {
		t.Error("fourth request should be rate limited (tpm=3)")
	}
}

func TestMemoryTokenBucket_KeyIsolation(t *testing.T) {
	b := newMemoryTokenBucket()

	ctx := context.Background()
	okA1, _, _, _ := b.Allow(ctx, "key-a", 1, 0)
	if !okA1 {
		t.Error("key-a first should be allowed")
	}

	okB1, _, _, _ := b.Allow(ctx, "key-b", 1, 0)
	if !okB1 {
		t.Error("key-b first should be allowed")
	}

	okB2, _, _, _ := b.Allow(ctx, "key-b", 1, 0)
	if okB2 {
		t.Error("key-b second should be rate limited")
	}
}

func TestMemoryTokenBucket_RemainingCount(t *testing.T) {
	b := newMemoryTokenBucket()

	ctx := context.Background()
	_, rpm, _, _ := b.Allow(ctx, "test-rem", 5, 0)
	if rpm != 4 {
		t.Errorf("remaining RPM should be 4 after first request, got %d", rpm)
	}
}

func TestMemoryTokenBucket_Refill(t *testing.T) {
	b := newMemoryTokenBucket()

	ctx := context.Background()
	// RPM = 60 → refill rate = 1 token/second. Drain all 60 tokens.
	for i := 0; i < 60; i++ {
		ok, _, _, _ := b.Allow(ctx, "test-refill", 60, 0)
		if !ok {
			t.Fatalf("request %d should be allowed within initial capacity", i+1)
		}
	}

	// 61st request — should be rate limited.
	ok61, _, _, _ := b.Allow(ctx, "test-refill", 60, 0)
	if ok61 {
		t.Fatal("61st request should be rate limited (RPM=60)")
	}

	// Wait for refill (~1 second for 1 token at RPM=60).
	time.Sleep(1100 * time.Millisecond)

	okAfter, _, _, _ := b.Allow(ctx, "test-refill", 60, 0)
	if !okAfter {
		t.Error("after refill, a new request should be allowed")
	}
}

func TestMemoryTokenBucket_ConcurrentAccess(t *testing.T) {
	b := newMemoryTokenBucket()
	ctx := context.Background()
	var wg sync.WaitGroup
	allowed := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, _, _ := b.Allow(ctx, "conc", 5, 0)
			allowed <- ok
		}()
	}
	wg.Wait()
	close(allowed)

	var allowedCount int
	for v := range allowed {
		if v {
			allowedCount++
		}
	}
	if allowedCount > 5 {
		t.Errorf("at most 5 concurrent requests should be allowed (rpm=5), got %d", allowedCount)
	}
}
