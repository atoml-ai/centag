package configsync

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"centag/core/pkg/billing"
)

type mockPriceStore struct {
	rules  []*billing.PricingRule
	nextID int64
}

func (s *mockPriceStore) GetRuleByModelAndType(_ context.Context, backendID, model string, _ billing.PriceType) (*billing.PricingRule, error) {
	for _, r := range s.rules {
		if r.BackendID == backendID && r.Model == model {
			return r, nil
		}
	}
	return nil, billing.ErrRuleNotFound
}

func (s *mockPriceStore) CreateRule(_ context.Context, r *billing.PricingRule) error {
	s.nextID++
	r.ID = s.nextID
	s.rules = append(s.rules, r)
	return nil
}

func (s *mockPriceStore) UpdateRule(_ context.Context, id int64, r *billing.PricingRule) error {
	for i, existing := range s.rules {
		if existing.ID == id {
			r.ID = id
			s.rules[i] = r
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func ppioPrice(models ...ModelPrice) ProviderPrice {
	return ProviderPrice{
		BaseURL: "https://api.ppinfra.com/v3/openai", ProviderName: "PPIO",
		Currency: "USD", Enabled: true, Models: models,
	}
}

// ---------- F. PriceApplier（TC-PAP-001~011） ----------

func TestPriceApplier(t *testing.T) {
	mapper := func(baseURL string) []string {
		switch strings_TrimRight(baseURL) {
		case "https://api.ppinfra.com/v3/openai":
			return []string{"ppinfra"}
		case "https://API.PPINFRA.COM/v3/openai":
			return []string{"ppinfra"}
		case "https://twin.example.com/v3/openai":
			return []string{"backendA", "backendB"}
		}
		return nil
	}

	t.Run("TC-PAP-001_新键创建Source_config", func(t *testing.T) {
		store := &mockPriceStore{}
		res, err := ApplyPrices(context.Background(), []ProviderPrice{ppioPrice(ModelPrice{Model: "deepseek-v3.2", InputPricePerM: 0.1389, OutputPricePerM: 0.1389})}, mapper, store, true)
		if err != nil || res.Applied != 1 {
			t.Fatalf("applied=%d err=%v", res.Applied, err)
		}
		if store.rules[0].Source != "config" {
			t.Fatalf("source=%q, want config", store.rules[0].Source)
		}
	})

	t.Run("TC-PAP-002_已有config规则更新", func(t *testing.T) {
		store := &mockPriceStore{}
		_, _ = ApplyPrices(context.Background(), []ProviderPrice{ppioPrice(ModelPrice{Model: "m", InputPricePerM: 1})}, mapper, store, true)
		_, err := ApplyPrices(context.Background(), []ProviderPrice{ppioPrice(ModelPrice{Model: "m", InputPricePerM: 2})}, mapper, store, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(store.rules) != 1 || store.rules[0].InputPricePerM != 2 {
			t.Fatalf("rule must be updated in place: rules=%d price=%v", len(store.rules), store.rules[0].InputPricePerM)
		}
	})

	t.Run("TC-PAP-003_manual跳过", func(t *testing.T) {
		store := &mockPriceStore{rules: []*billing.PricingRule{{ID: 1, BackendID: "ppinfra", Model: "m", InputPricePerM: 30, Source: "manual"}}}
		res, err := ApplyPrices(context.Background(), []ProviderPrice{ppioPrice(ModelPrice{Model: "m", InputPricePerM: 1})}, mapper, store, true)
		if err != nil || res.Skipped != 1 || res.Applied != 0 {
			t.Fatalf("applied=%d skipped=%d err=%v", res.Applied, res.Skipped, err)
		}
		if store.rules[0].InputPricePerM != 30 {
			t.Fatal("manual rule must stay untouched")
		}
	})

	t.Run("TC-PAP-004_尾斜杠规范化", func(t *testing.T) {
		store := &mockPriceStore{}
		p := ppioPrice(ModelPrice{Model: "m", InputPricePerM: 1})
		p.BaseURL = "https://api.ppinfra.com/v3/openai/"
		res, err := ApplyPrices(context.Background(), []ProviderPrice{p}, mapper, store, true)
		if err != nil || res.Applied != 1 {
			t.Fatalf("trailing slash must normalize to a hit: applied=%d err=%v", res.Applied, err)
		}
	})

	t.Run("TC-PAP-005_host大小写不敏感", func(t *testing.T) {
		store := &mockPriceStore{}
		p := ppioPrice(ModelPrice{Model: "m", InputPricePerM: 1})
		p.BaseURL = "https://API.ppinfra.com/v3/openai"
		res, err := ApplyPrices(context.Background(), []ProviderPrice{p}, mapper, store, true)
		if err != nil || res.Applied != 1 {
			t.Fatalf("case-insensitive host must match: applied=%d err=%v", res.Applied, err)
		}
	})

	t.Run("TC-PAP-006_多后端命中复制", func(t *testing.T) {
		store := &mockPriceStore{}
		p := ProviderPrice{BaseURL: "https://twin.example.com/v3/openai", ProviderName: "Twin", Enabled: true, Models: []ModelPrice{{Model: "m", InputPricePerM: 1}}}
		res, err := ApplyPrices(context.Background(), []ProviderPrice{p}, mapper, store, true)
		if err != nil || res.Applied != 2 {
			t.Fatalf("two backends must both receive rules: applied=%d err=%v", res.Applied, err)
		}
		if len(store.rules) != 2 {
			t.Fatalf("rules=%d, want 2", len(store.rules))
		}
	})

	t.Run("TC-PAP-007_零命中降级到DeriveBackendID", func(t *testing.T) {
		store := &mockPriceStore{}
		res, err := ApplyPrices(context.Background(), []ProviderPrice{ppioPrice(ModelPrice{Model: "m"})}, func(string) []string { return nil }, store, true)
		if err != nil || res.Applied != 1 {
			t.Fatalf("zero-hit should fallback to DeriveBackendID: %+v err=%v", res, err)
		}
		if len(store.rules) != 1 {
			t.Fatal("one rule should be created via DeriveBackendID fallback")
		}
	})

	t.Run("TC-PAP-008_禁用供应商行不生成规则", func(t *testing.T) {
		store := &mockPriceStore{}
		p := ppioPrice(ModelPrice{Model: "m"})
		p.Enabled = false
		res, err := ApplyPrices(context.Background(), []ProviderPrice{p}, mapper, store, true)
		if err != nil || res.Applied != 0 || len(store.rules) != 0 {
			t.Fatalf("disabled row must produce no rules: applied=%d err=%v", res.Applied, err)
		}
	})

	t.Run("TC-PAP-009_价格字段透传", func(t *testing.T) {
		store := &mockPriceStore{}
		// pricing-sync 写入侧已折算成本价（cost_multiplier 已应用），
		// 设备侧按值透传，不重复折扣。
		p := ppioPrice(ModelPrice{Model: "deepseek-v3.2", Name: "DeepSeek V3.2", InputPricePerM: 0.0695, OutputPricePerM: 0.0695, CostMultiplier: 0.5})
		_, err := ApplyPrices(context.Background(), []ProviderPrice{p}, mapper, store, true)
		if err != nil {
			t.Fatal(err)
		}
		r := store.rules[0]
		if r.InputPricePerM != 0.0695 || r.OutputPricePerM != 0.0695 {
			t.Fatalf("prices must pass through as resolved: %v/%v", r.InputPricePerM, r.OutputPricePerM)
		}
	})

	t.Run("TC-PAP-010_Personal全量覆盖manual", func(t *testing.T) {
		store := &mockPriceStore{rules: []*billing.PricingRule{{ID: 1, BackendID: "ppinfra", Model: "m", InputPricePerM: 30, Source: "manual"}}}
		res, err := ApplyPrices(context.Background(), []ProviderPrice{ppioPrice(ModelPrice{Model: "m", InputPricePerM: 1})}, mapper, store, false)
		if err != nil || res.Applied != 1 || res.Skipped != 0 {
			t.Fatalf("personal mode must overwrite manual: applied=%d skipped=%d err=%v", res.Applied, res.Skipped, err)
		}
		if store.rules[0].InputPricePerM != 1 {
			t.Fatal("personal full sync must replace the value")
		}
	})

	t.Run("TC-PAP-011_结果可观测", func(t *testing.T) {
		store := &mockPriceStore{rules: []*billing.PricingRule{{ID: 1, BackendID: "ppinfra", Model: "manualM", InputPricePerM: 30, Source: "manual"}}}
		res, _ := ApplyPrices(context.Background(), []ProviderPrice{ppioPrice(ModelPrice{Model: "m"}, ModelPrice{Model: "manualM"})}, mapper, store, true)
		if res.Applied != 1 || res.Skipped != 1 {
			t.Fatalf("counts must be observable: applied=%d skipped=%d", res.Applied, res.Skipped)
		}
		b, _ := json.Marshal(res)
		if !json.Valid(b) {
			t.Fatal("result must serialize for status endpoint")
		}
	})
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func strings_TrimRight(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// ---------- G. VersionApplier（core 侧适配器，TC-VAP-001/004/006/007） ----------

func TestVersionApplier(t *testing.T) {
	releaseRows := func() []Row {
		return []Row{
			{Key: "release.channel.stable", Channel: "stable", Edition: "all", Enabled: true,
				Value: mustJSON(VersionInfo{Version: "0.3.4", PackageURL: "https://example.com/pkg", SHA256: "abc", MinCompatible: "0.2.0", ForceUpdate: true})},
			{Key: "release.channel.beta", Channel: "beta", Edition: "all", Enabled: true,
				Value: mustJSON(VersionInfo{Version: "0.4.0-beta1"})},
		}
	}

	t.Run("TC-VAP-001_有效版本行返回元数据", func(t *testing.T) {
		a := NewVersionProviderAdapter(func(ctx context.Context) ([]Row, error) { return releaseRows(), nil }, "stable")
		res, err := a.CheckLatest(context.Background(), "0.3.3")
		if err != nil || !res.UpdateAvailable || res.Version != "0.3.4" || res.DownloadURL != "https://example.com/pkg" || res.SHA256 != "abc" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})

	t.Run("TC-VAP-004_channel匹配", func(t *testing.T) {
		a := NewVersionProviderAdapter(func(ctx context.Context) ([]Row, error) { return releaseRows(), nil }, "beta")
		res, err := a.CheckLatest(context.Background(), "0.3.3")
		if err != nil || res.Version != "0.4.0-beta1" {
			t.Fatalf("beta channel must match beta row: %+v err=%v", res, err)
		}
	})

	t.Run("TC-VAP-006_force_update透传", func(t *testing.T) {
		rows := releaseRows()
		var info VersionInfo
		_ = json.Unmarshal(rows[0].Value, &info)
		if !info.ForceUpdate {
			t.Fatal("force_update must survive row parsing")
		}
	})

	t.Run("TC-VAP-007_低于min_compatible不提示更新", func(t *testing.T) {
		a := NewVersionProviderAdapter(func(ctx context.Context) ([]Row, error) { return releaseRows(), nil }, "stable")
		res, err := a.CheckLatest(context.Background(), "0.1.0")
		if err != nil {
			t.Fatal(err)
		}
		if res.UpdateAvailable {
			t.Fatal("version below min_compatible must not be prompted to update")
		}
		if res.Message == "" {
			t.Fatal("suppression must carry an explanatory message")
		}
	})
}

// ---------- H. BootstrapApplier（TC-BAP-001~003） ----------

func TestBootstrapApplier(t *testing.T) {
	t.Run("TC-BAP-001_表指针解析构造Provider", func(t *testing.T) {
		created := ""
		ba := NewBootstrapApplier(map[string]ProviderFactory{
			"table.model_price": func(v json.RawMessage) (Provider, error) {
				created = string(v)
				return NewSnapshotProvider([]string{"http://example.com/snap.json"}), nil
			},
		})
		rows := []Row{{Key: "table.model_price", Enabled: true, Value: mustJSON(map[string]string{"provider": "public_snapshot", "source": "pricing"})}}
		if err := ba.Apply(rows); err != nil {
			t.Fatal(err)
		}
		if ba.GetProvider("table.model_price") == nil || created == "" {
			t.Fatal("factory must run and provider must be registered")
		}
	})

	t.Run("TC-BAP-002_指针热更新", func(t *testing.T) {
		var lastURL string
		ba := NewBootstrapApplier(map[string]ProviderFactory{
			"table.model_price": func(v json.RawMessage) (Provider, error) {
				var m map[string]string
				_ = json.Unmarshal(v, &m)
				lastURL = m["url"]
				return NewSnapshotProvider([]string{lastURL}), nil
			},
		})
		_ = ba.Apply([]Row{{Key: "table.model_price", Enabled: true, Value: mustJSON(map[string]string{"url": "http://a"})}})
		_ = ba.Apply([]Row{{Key: "table.model_price", Enabled: true, Value: mustJSON(map[string]string{"url": "http://b"})}})
		if lastURL != "http://b" {
			t.Fatalf("pointer must hot-reload to new table address, got %s", lastURL)
		}
	})

	t.Run("TC-BAP-003_畸形指针不崩溃", func(t *testing.T) {
		ba := NewBootstrapApplier(map[string]ProviderFactory{
			"table.model_price": func(v json.RawMessage) (Provider, error) {
				return nil, fmt.Errorf("bad pointer")
			},
		})
		if err := ba.Apply([]Row{{Key: "table.model_price", Enabled: true, Value: mustJSON("junk")}}); err == nil {
			t.Fatal("malformed pointer must surface an error (counted in status), not panic")
		}
	})
}

// ---------- I. GenericApplier（TC-GAP-001~003） ----------

func TestGenericApplier(t *testing.T) {
	t.Run("TC-GAP-001_feature行落库", func(t *testing.T) {
		ga := NewGenericApplier()
		ga.Apply([]Row{{Key: "feature.dark_mode", Enabled: true, Value: mustJSON(true)}})
		if ga.Get("feature.dark_mode") == nil {
			t.Fatal("enabled feature row must be stored")
		}
	})

	t.Run("TC-GAP-002_禁用行不落库", func(t *testing.T) {
		ga := NewGenericApplier()
		ga.Apply([]Row{{Key: "feature.off", Enabled: false, Value: mustJSON(true)}})
		if ga.Get("feature.off") != nil {
			t.Fatal("disabled feature row must not be stored")
		}
	})

	t.Run("TC-GAP-003_生效值可读", func(t *testing.T) {
		ga := NewGenericApplier()
		ga.Apply([]Row{
			{Key: "feature.a", Enabled: true, Value: mustJSON("x")},
			{Key: "table.model_price", Enabled: true, Value: mustJSON("y")},
		})
		all := ga.All()
		if len(all) != 1 || string(all["feature.a"]) != `"x"` {
			t.Fatalf("status view must expose only effective feature values: %v", all)
		}
	})
}
