package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileBackendStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "initial-backends.yaml")
	store := NewFileBackendStore(path)

	in := []*BackendConfig{
		{
			ID:         "openai-main",
			Name:       "OpenAI",
			Type:       "openai",
			BaseURL:    "https://api.openai.com/v1",
			APIKey:     "sk-test",
			Enabled:    true,
			Timeout:    60,
			Weight:     2,
			Priority:   1,
			ProbeModel: "gpt-4o-mini",
			SupportedModels: []ModelMapping{
				{RequestedModel: "gpt-4o", ActualModel: "gpt-4o", IsExact: true, CompatibilityScore: 1},
				{RequestedModel: "gpt-4o-mini", ActualModel: "gpt-4o-mini", IsExact: true, CompatibilityScore: 1},
			},
		},
	}

	if err := store.Save(in); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	out, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(out))
	}
	got := out[0]
	if got.ID != "openai-main" || got.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("unexpected backend: %+v", got)
	}
	if got.Weight != 2 || got.Priority != 1 {
		t.Fatalf("weight/priority not preserved: weight=%d priority=%d", got.Weight, got.Priority)
	}
	if len(got.SupportedModels) != 2 || got.SupportedModels[0].RequestedModel != "gpt-4o" {
		t.Fatalf("supported models not preserved: %+v", got.SupportedModels)
	}
	if got.ProbeModel != "gpt-4o-mini" {
		t.Fatalf("probe_model not preserved: %q", got.ProbeModel)
	}
}

func TestFileBackendStoreMissingFile(t *testing.T) {
	store := NewFileBackendStore(filepath.Join(t.TempDir(), "missing.yaml"))
	out, err := store.Load()
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty list, got %d", len(out))
	}
}

func TestConfigBackendConversionPreservesWeight(t *testing.T) {
	src := &BackendConfig{
		ID:       "b1",
		Name:     "B1",
		Type:     "openai",
		BaseURL:  "http://localhost",
		Weight:   7,
		Priority: 3,
		TenantID: "t1",
	}
	cfg := backendToConfigBackend(src)
	if cfg.Weight != 7 || cfg.Priority != 3 || cfg.TenantID != "t1" {
		t.Fatalf("backendToConfigBackend dropped fields: %+v", cfg)
	}
	round := configBackendToBackend(&cfg)
	if round.Weight != 7 || round.Priority != 3 || round.TenantID != "t1" {
		t.Fatalf("configBackendToBackend dropped fields: %+v", round)
	}
}
