package configsync

import (
	"context"
	"errors"
	"testing"
	"time"
)

func timeNow() time.Time { return time.Now() }

// ---------- Q. 端到端（TC-E2E-001~003） ----------

func TestE2E(t *testing.T) {
	ctx := context.Background()

	t.Run("TC-E2E-001_价格全链路", func(t *testing.T) {
		// 通道 → 调度器 → 校验 → PriceApplier → 本地规则库
		p := &mockProvider{
			rows: []Row{validRow()},
			prices: []ProviderPrice{{
				BaseURL: "https://api.ppinfra.com/v3/openai", ProviderName: "PPIO",
				Currency: "USD", Enabled: true,
				Models: []ModelPrice{{Model: "deepseek-v3.2", InputPricePerM: 0.1389, OutputPricePerM: 0.1389, CostMultiplier: 1}},
			}},
		}
		store := &mockPriceStore{}
		s := NewScheduler(SchedulerConfig{Provider: p, StateDir: t.TempDir()})
		if err := s.SyncNow(ctx); err != nil {
			t.Fatal(err)
		}
		snap := s.Snapshot()
		if snap == nil || len(snap.Prices) != 1 {
			t.Fatal("snapshot must carry fetched prices")
		}
		res, err := ApplyPrices(ctx, snap.Prices, func(baseURL string) []string { return []string{"ppinfra"} }, store, true)
		if err != nil || res.Applied != 1 {
			t.Fatalf("applied=%d err=%v", res.Applied, err)
		}
		r := store.rules[0]
		if r.BackendID != "ppinfra" || r.Model != "deepseek-v3.2" || r.Source != "config" {
			t.Fatalf("rule content wrong: %+v", r)
		}
		if s.Status().LastSyncOK != true {
			t.Fatal("status must report success")
		}
	})

	t.Run("TC-E2E-002_版本链路", func(t *testing.T) {
		p := &mockProvider{rows: []Row{
			{Key: "release.channel.stable", Channel: "stable", Edition: "all", Enabled: true,
				Value: mustJSON(VersionInfo{Version: "0.3.4", PackageURL: "https://cdn/pkg.tar.gz", SHA256: "cafe"})},
		}}
		s := NewScheduler(SchedulerConfig{Provider: p})
		if err := s.SyncNow(ctx); err != nil {
			t.Fatal(err)
		}
		adapter := NewVersionProviderAdapter(func(ctx context.Context) ([]Row, error) { return s.Snapshot().Config, nil }, "stable")
		res, err := adapter.CheckLatest(ctx, "0.3.3")
		if err != nil || !res.UpdateAvailable || res.Version != "0.3.4" || res.DownloadURL != "https://cdn/pkg.tar.gz" {
			t.Fatalf("version chain broken: %+v err=%v", res, err)
		}
	})

	t.Run("TC-E2E-003_篡改防御_拒收保持last-good", func(t *testing.T) {
		dir := t.TempDir()
		good := &mockProvider{rows: []Row{validRow()}}
		s := NewScheduler(SchedulerConfig{Provider: good, StateDir: dir})
		if err := s.SyncNow(ctx); err != nil {
			t.Fatal(err)
		}
		before := s.Snapshot()

		// Tampered batch: one poisoned row → whole batch rejected.
		bad := &mockProvider{rows: []Row{
			validRow(),
			{Key: "release.channel.stable", Channel: "stable", Edition: "all", Enabled: true, Value: []byte(`{broken`)},
		}}
		s2 := NewScheduler(SchedulerConfig{Provider: bad, StateDir: dir})
		_ = s2.SyncNow(ctx) // must fail and keep last-good
		if s2.Status().LastSyncOK {
			t.Fatal("tampered batch must not be marked as success")
		}
		if s2.Status().LastError == "" || !errors.Is(nil, nil) && s2.Status().LastError == "" {
			t.Fatal("rejection must be recorded in status.last_error")
		}
		// last-good still served (loaded from stateDir on construction).
		kept := s2.Snapshot()
		if kept == nil || len(kept.Config) != len(before.Config) {
			t.Fatal("last-good snapshot must be preserved after tamper rejection")
		}
	})
}
