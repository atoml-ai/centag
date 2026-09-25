package freellm

import (
	"testing"

	"centag/core/pkg/backend"
)

func TestListCatalog(t *testing.T) {
	all := ListCatalog("")
	if len(all) == 0 {
		t.Fatal("catalog should not be empty")
	}

	keyless := ListCatalog(AuthKeyless)
	if len(keyless) == 0 {
		t.Fatal("expected at least one keyless provider")
	}
	for _, e := range keyless {
		if e.AuthType != AuthKeyless {
			t.Errorf("entry %q filtered as keyless but auth_type=%s", e.ID, e.AuthType)
		}
	}

	// keyless 条目必须可零配置注册（无 DummyKey 依赖或已声明）
	for _, e := range keyless {
		if e.BaseURL == "" {
			t.Errorf("keyless entry %q missing BaseURL", e.ID)
		}
	}
}

func TestToBackendConfig(t *testing.T) {
	// keyless 条目（pollinations）
	var pol ProviderCatalogEntry
	for _, e := range Catalog {
		if e.ID == "pollinations" {
			pol = e
		}
	}
	cfg := ToBackendConfig(pol, "", "")
	if cfg.ID != "freellm-pollinations" {
		t.Errorf("unexpected id %q", cfg.ID)
	}
	if cfg.Type != "openai" {
		t.Errorf("free backends must be openai-compatible, got %q", cfg.Type)
	}
	if !cfg.AutoFetchModels {
		t.Error("AutoFetchModels should be true so real model list is fetched live")
	}
	if cfg.ProbeModel != "openai-fast" {
		t.Errorf("unexpected probe model %q", cfg.ProbeModel)
	}
	if cfg.Metadata["source"] != "freellm" || cfg.Metadata["free_tier"] != "true" {
		t.Errorf("missing freellm metadata: %+v", cfg.Metadata)
	}
	if len(cfg.SupportedModels) != 1 || cfg.SupportedModels[0].RequestedModel != "openai-fast" {
		t.Errorf("supported models not seeded: %+v", cfg.SupportedModels)
	}

	// keyless 但要求非空 key 的条目（llm7）应使用 DummyKey
	var llm7 ProviderCatalogEntry
	for _, e := range Catalog {
		if e.ID == "llm7" {
			llm7 = e
		}
	}
	cfg7 := ToBackendConfig(llm7, "", "")
	if cfg7.APIKey != "unused" {
		t.Errorf("expected dummy key 'unused' for llm7, got %q", cfg7.APIKey)
	}

	// keyed 条目（groq）传入 key 应落到 APIKey
	var groq ProviderCatalogEntry
	for _, e := range Catalog {
		if e.ID == "groq" {
			groq = e
		}
	}
	cfgG := ToBackendConfig(groq, "sk-test", "")
	if cfgG.APIKey != "sk-test" {
		t.Errorf("expected api key propagated, got %q", cfgG.APIKey)
	}
}

func TestRegisterKeyedRequiresKey(t *testing.T) {
	var groq ProviderCatalogEntry
	for _, e := range Catalog {
		if e.ID == "groq" {
			groq = e
		}
	}
	m := backend.NewManager()
	if _, err := Register(m, groq, "", ""); err == nil {
		t.Error("keyed provider without api key should error")
	}
}

func TestRegisterAndScanKeylessInMemory(t *testing.T) {
	m := backend.NewManager()

	// 注册单个 keyless 条目（忽略 Save 在无 config 环境下的错误，验证内存态）
	var pol ProviderCatalogEntry
	for _, e := range Catalog {
		if e.ID == "pollinations" {
			pol = e
		}
	}
	if _, err := Register(m, pol, "", ""); err != nil {
		t.Logf("Register Save step err (expected without store): %v", err)
	}
	got, gerr := m.Get("freellm-pollinations")
	if gerr != nil {
		t.Fatalf("keyless backend should be added in-memory: %v", gerr)
	}
	if got.Metadata["free_tier"] != "true" {
		t.Error("registered backend missing free_tier metadata")
	}

	// 批量扫描所有 keyless
	ids, err := ScanKeyless(m, "")
	if err != nil {
		t.Logf("ScanKeyless Save step err (expected without store): %v", err)
	}
	keylessCount := len(ListCatalog(AuthKeyless))
	if len(ids) != keylessCount {
		t.Errorf("expected %d keyless ids, got %d", keylessCount, len(ids))
	}
	for _, id := range ids {
		if _, e := m.Get(id); e != nil {
			t.Errorf("keyless backend %q not added: %v", id, e)
		}
	}
}
