package feishu

import (
	"context"
	"encoding/json"
	"testing"

	"centag/core/pkg/configsync"
)

func record(fields map[string]any) Record {
	return Record{Fields: fields}
}

// textField-compatible field builders
func text(v string) []any {
	if v == "" {
		return nil
	}
	return []any{map[string]any{"text": v, "type": "text"}}
}

func TestParsePipelineTemplate_ProductionSchema(t *testing.T) {
	content := map[string]any{
		"id":       "cache-pipeline",
		"name":     "Cache Pipeline",
		"version":  "0.9",
		"nodes":    []any{},
		"metadata": map[string]any{"origin": "yaml"},
		"global_config": map[string]any{
			"timeout":     120,
			"max_retries": 3,
		},
	}
	contentJSON, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}

	rec := record(map[string]any{
		"pipeline_id":    text("cache-pipeline"),
		"name":           text("Cache Pipeline (updated)"),
		"description":    text("updated desc"),
		"edition":        text("common"),
		"version":        text("1.1"),
		"schema_version": text("centag.pipeline/v1alpha1"),
		"shortcut_code":  text("#cache"),
		"enabled":        true,
		"content_json":   string(contentJSON),
	})

	tmpl := parsePipelineTemplate(rec)
	if tmpl == nil {
		t.Fatal("expected template, got nil")
	}
	if tmpl.ID != "cache-pipeline" {
		t.Errorf("ID = %q", tmpl.ID)
	}
	// Record scalars win over content_json copies.
	if tmpl.Name != "Cache Pipeline (updated)" {
		t.Errorf("Name = %q, want record value", tmpl.Name)
	}
	if tmpl.Version != "1.1" {
		t.Errorf("Version = %q, want record value", tmpl.Version)
	}
	if tmpl.Description != "updated desc" {
		t.Errorf("Description = %q", tmpl.Description)
	}
	if tmpl.SchemaVersion != "centag.pipeline/v1alpha1" {
		t.Errorf("SchemaVersion = %q", tmpl.SchemaVersion)
	}
	if tmpl.ShortcutCode != "#cache" {
		t.Errorf("ShortcutCode = %q", tmpl.ShortcutCode)
	}
	// Directory-style edition normalizes to product semantics.
	if tmpl.Edition != "all" {
		t.Errorf("Edition = %q, want all", tmpl.Edition)
	}
	if tmpl.Metadata["origin"] != "yaml" {
		t.Errorf("Metadata not parsed from content_json: %v", tmpl.Metadata)
	}
	if tmpl.GlobalConfig == nil || tmpl.GlobalConfig.Timeout != 120 {
		t.Errorf("GlobalConfig not parsed from content_json: %+v", tmpl.GlobalConfig)
	}
}

func TestParsePipelineTemplate_TeamEditionKept(t *testing.T) {
	rec := record(map[string]any{
		"pipeline_id": text("aggregator-mode"),
		"name":        text("Aggregator Mode"),
		"edition":     text("team"),
		"enabled":     true,
	})
	tmpl := parsePipelineTemplate(rec)
	if tmpl == nil {
		t.Fatal("expected template, got nil")
	}
	if tmpl.Edition != "team" {
		t.Errorf("Edition = %q, want team", tmpl.Edition)
	}
	if tmpl.Nodes == nil || len(tmpl.Nodes) != 0 {
		t.Errorf("Nodes should default to empty slice, got %v", tmpl.Nodes)
	}
}

func TestParsePipelineTemplate_LegacySchema(t *testing.T) {
	rec := record(map[string]any{
		"id":     text("legacy"),
		"name":   text("Legacy Template"),
		"type":   text("team"),
		"config": `{"steps": [{"name": "step1", "tool": "router"}]}`,
	})
	tmpl := parsePipelineTemplate(rec)
	if tmpl == nil {
		t.Fatal("expected template, got nil")
	}
	if tmpl.ID != "legacy" || tmpl.Name != "Legacy Template" || tmpl.Edition != "team" {
		t.Errorf("legacy parse = %+v", tmpl)
	}
}

func TestParsePipelineTemplate_MissingID(t *testing.T) {
	rec := record(map[string]any{"name": text("no id")})
	if tmpl := parsePipelineTemplate(rec); tmpl != nil {
		t.Errorf("expected nil for missing id, got %+v", tmpl)
	}
}

func TestParsePipelineTemplate_InvalidContentJSON(t *testing.T) {
	rec := record(map[string]any{
		"pipeline_id":  text("bad-json"),
		"name":         text("Bad JSON"),
		"content_json": "{not valid json",
	})
	tmpl := parsePipelineTemplate(rec)
	if tmpl == nil {
		t.Fatal("expected template, got nil")
	}
	if tmpl.ID != "bad-json" || tmpl.Name != "Bad JSON" {
		t.Errorf("fallback parse = %+v", tmpl)
	}
}

func TestNormalizePipelineEdition(t *testing.T) {
	cases := map[string]string{
		"":         "all",
		"common":   "all",
		"extras":   "all",
		"all":      "all",
		"team":     "team",
		"personal": "personal",
		"Team":     "team",
		" custom ": "custom", // unknown values pass through
	}
	for in, want := range cases {
		if got := normalizePipelineEdition(in); got != want {
			t.Errorf("normalizePipelineEdition(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseBackendRow_ContentJSON(t *testing.T) {
	// configctl backend add shape: backend_id at table level, full JSON in content_json.
	rec := record(map[string]any{
		"backend_id":   text("deepseek-main"),
		"name":         text("DeepSeek"),
		"enabled":      true,
		"content_json": `{"backend_id":"deepseek-main","name":"DeepSeek","type":"openai","base_url":"https://api.deepseek.com","enabled":true,"timeout":60,"max_retries":3,"description":"seed"}`,
	})
	row := parseBackendRow(rec)
	if row == nil {
		t.Fatal("expected row, got nil")
	}
	if row.Key != "backend.deepseek-main" {
		t.Errorf("Key = %q", row.Key)
	}
	if row.Edition != "all" || !row.Enabled {
		t.Errorf("Edition/Enabled = %q/%v", row.Edition, row.Enabled)
	}
	var cfg configsync.BackendConfig
	if err := json.Unmarshal(row.Value, &cfg); err != nil {
		t.Fatalf("value not valid BackendConfig JSON: %v", err)
	}
	if cfg.ID != "deepseek-main" || cfg.BaseURL != "https://api.deepseek.com" || cfg.Type != "openai" {
		t.Errorf("parsed backend = %+v", cfg)
	}
}

func TestParseBackendRow_JSONIDWins(t *testing.T) {
	rec := record(map[string]any{
		"backend_id":   text("table-id"),
		"content_json": `{"id":"json-id","name":"X","type":"openai","base_url":"https://x.example.com"}`,
	})
	row := parseBackendRow(rec)
	if row == nil {
		t.Fatal("expected row, got nil")
	}
	if row.Key != "backend.json-id" {
		t.Errorf("Key = %q, want backend.json-id", row.Key)
	}
}

func TestParseBackendRow_ScalarSchema(t *testing.T) {
	// Production backend table layout: id / name / type / base_url / ...
	rec := record(map[string]any{
		"id":           text("openai-main"),
		"name":         text("OpenAI"),
		"type":         "openai",
		"base_url":     map[string]any{"link": "https://api.openai.com", "text": "https://api.openai.com"},
		"timeout":      float64(60),
		"max_retries":  float64(3),
		"weight":       float64(10),
		"probe_model":  text("gpt-4o-mini"),
		"description":  text("scalar"),
		"enabled":      true,
		"capabilities": `{"max_context_tokens":128000,"supports_images":true,"supports_tools":true}`,
	})
	row := parseBackendRow(rec)
	if row == nil {
		t.Fatal("expected row, got nil")
	}
	if row.Key != "backend.openai-main" {
		t.Errorf("Key = %q", row.Key)
	}
	var cfg configsync.BackendConfig
	if err := json.Unmarshal(row.Value, &cfg); err != nil {
		t.Fatalf("value not valid BackendConfig JSON: %v", err)
	}
	if cfg.BaseURL != "https://api.openai.com" || cfg.Name != "OpenAI" || cfg.Type != "openai" {
		t.Errorf("parsed backend = %+v", cfg)
	}
	if cfg.Timeout != 60 || cfg.MaxRetries != 3 || cfg.Weight != 10 {
		t.Errorf("numeric fields = %d/%d/%d", cfg.Timeout, cfg.MaxRetries, cfg.Weight)
	}
	if cfg.Capabilities.MaxContextTokens != 128000 || !cfg.Capabilities.SupportsTools {
		t.Errorf("capabilities = %+v", cfg.Capabilities)
	}
	if cfg.ProbeModel != "gpt-4o-mini" {
		t.Errorf("ProbeModel = %q", cfg.ProbeModel)
	}
}

func TestParseBackendRow_MissingID(t *testing.T) {
	rec := record(map[string]any{"name": text("no id"), "content_json": `{"name":"x"}`})
	if row := parseBackendRow(rec); row != nil {
		t.Errorf("expected nil, got %+v", row)
	}
}

func TestFetchBackendRows_NotConfigured(t *testing.T) {
	p := NewProvider(ProviderConfig{AppID: "a", AppSecret: "s", AppToken: "t"})
	if _, err := p.FetchBackendRows(context.Background()); err != configsync.ErrNotSupported {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}
