package configsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockProvider struct {
	rows      []Row
	prices    []ProviderPrice
	cfgErr    error
	priceErr  error
	blockTime time.Duration
	fetches   atomic.Int32
}

func (m *mockProvider) FetchConfig(ctx context.Context, q Query) ([]Row, error) {
	m.fetches.Add(1)
	if m.blockTime > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.blockTime):
		}
	}
	if m.cfgErr != nil {
		return nil, m.cfgErr
	}
	return m.rows, nil
}

func (m *mockProvider) FetchModelPrices(ctx context.Context) ([]ProviderPrice, error) {
	if m.priceErr != nil {
		return nil, m.priceErr
	}
	return m.prices, nil
}

func validRow() Row {
	return Row{Key: "table.model_price", Edition: "all", Value: []byte(`{"provider":"feishu"}`), Enabled: true}
}

// ---------- D. 调度器（TC-SCH-001~008） ----------

func TestScheduler(t *testing.T) {
	t.Run("TC-SCH-001_启动不阻塞", func(t *testing.T) {
		p := &mockProvider{blockTime: 500 * time.Millisecond, rows: []Row{validRow()}}
		s := NewScheduler(SchedulerConfig{Provider: p})
		start := time.Now()
		done := make(chan struct{})
		go func() { s.Start(context.Background()); close(done) }()
		select {
		case <-done:
			if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
				t.Fatalf("Start blocked %v", elapsed)
			}
		case <-time.After(time.Second):
			t.Fatal("Start blocked >1s")
		}
		s.Stop()
	})

	t.Run("TC-SCH-002_首拉抖动窗口", func(t *testing.T) {
		for i := 0; i < 200; i++ {
			d := initialJitter()
			if d < 30*time.Second || d > 90*time.Second {
				t.Fatalf("jitter %v outside [30s,90s]", d)
			}
		}
	})

	t.Run("TC-SCH-003_轮询抖动区间", func(t *testing.T) {
		s := NewScheduler(SchedulerConfig{Provider: &mockProvider{}, Interval: time.Hour})
		base := float64(time.Hour)
		for i := 0; i < 100; i++ {
			s.mu.Lock()
			s.failCount = 0
			s.mu.Unlock()
			d := s.nextDelay()
			if float64(d) < base*0.9 || float64(d) > base*1.1 {
				t.Fatalf("delay %v outside ±10%% of 1h", d)
			}
		}
	})

	t.Run("TC-SCH-004_指数退避", func(t *testing.T) {
		s := NewScheduler(SchedulerConfig{Provider: &mockProvider{}, Interval: time.Minute})
		var prev time.Duration
		for fail := 1; fail <= 4; fail++ {
			s.mu.Lock()
			s.failCount = fail
			s.mu.Unlock()
			d := s.nextDelay()
			if fail <= 3 && d <= prev {
				t.Fatalf("backoff must grow: fail=%d d=%v prev=%v", fail, d, prev)
			}
			prev = d
		}
		// Capped at 8× interval (with jitter ≤ 1.1×).
		s.mu.Lock()
		s.failCount = 10
		s.mu.Unlock()
		if s.nextDelay() > time.Minute*8*11/10 {
			t.Fatalf("backoff not capped at 8x")
		}
	})

	t.Run("TC-SCH-005_429尊重RetryAfter", func(t *testing.T) {
		s := NewScheduler(SchedulerConfig{Provider: &mockProvider{cfgErr: &RateLimitError{RetryAfter: 5 * time.Minute}}, Interval: time.Minute})
		if err := s.SyncNow(context.Background()); err == nil {
			t.Fatal("rate-limited sync must fail")
		}
		// Next delay must be the server-requested window, not backoff.
		d := s.nextDelay()
		if d != 5*time.Minute {
			t.Fatalf("next delay %v, want Retry-After 5m", d)
		}
		// Consumed once.
		if d2 := s.nextDelay(); d2 >= 5*time.Minute {
			t.Fatalf("Retry-After consumed once; got %v", d2)
		}
	})

	t.Run("TC-SCH-006_last-good落盘0600", func(t *testing.T) {
		dir := t.TempDir()
		p := &mockProvider{rows: []Row{validRow()}, prices: []ProviderPrice{{BaseURL: "https://x/", ProviderName: "T", Enabled: true, Models: []ModelPrice{{Model: "m", InputPricePerM: 1, OutputPricePerM: 2}}}}}
		s := NewScheduler(SchedulerConfig{Provider: p, StateDir: dir})
		if err := s.SyncNow(context.Background()); err != nil {
			t.Fatal(err)
		}
		snap, err := ReadSnapshot(dir)
		if err != nil || snap == nil {
			t.Fatalf("snapshot missing: %v", err)
		}
		if len(snap.Config) != 1 || len(snap.Prices) != 1 {
			t.Fatalf("snapshot content wrong: %d rows %d prices", len(snap.Config), len(snap.Prices))
		}
		info, _ := os.Stat(filepath.Join(dir, snapshotFileName))
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("snapshot perms %v, want 0600", info.Mode().Perm())
		}
	})

	t.Run("TC-SCH-007_快照回载断网可用", func(t *testing.T) {
		dir := t.TempDir()
		if err := WriteSnapshot(dir, &Snapshot{Config: []Row{validRow()}}); err != nil {
			t.Fatal(err)
		}
		// Provider always fails (offline).
		s := NewScheduler(SchedulerConfig{Provider: &mockProvider{cfgErr: errors.New("network down")}, StateDir: dir})
		_ = s.SyncNow(context.Background())
		snap := s.Snapshot()
		if snap == nil || len(snap.Config) != 1 {
			t.Fatal("last-good snapshot must survive failed syncs")
		}
	})

	t.Run("TC-SCH-008_手动触发单飞", func(t *testing.T) {
		p := &mockProvider{blockTime: 80 * time.Millisecond, rows: []Row{validRow()}}
		s := NewScheduler(SchedulerConfig{Provider: p})
		var wg sync.WaitGroup
		var inFlightErr atomic.Int32
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := s.SyncNow(context.Background()); err != nil && err.Error() == "sync already in flight" {
					inFlightErr.Add(1)
				}
			}()
		}
		wg.Wait()
		if inFlightErr.Load() == 0 {
			t.Fatal("concurrent SyncNow must be single-flight")
		}
		if p.fetches.Load() > 5 {
			t.Fatalf("fetches=%d, no duplication expected", p.fetches.Load())
		}
	})
}
