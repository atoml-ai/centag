package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilePipelineStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilePipelineStore(dir)
	if err != nil {
		t.Fatalf("NewFilePipelineStore: %v", err)
	}

	p := &AgentPatternPipeline{
		ID:          "router-mode",
		Name:        "路由模式",
		Description: "test",
		Version:     "1.0",
		Nodes: []PipelineNodeConfig{
			{
				ID:      "code-generator",
				Type:    NodeTypeGenerator,
				Name:    "代码生成",
				Backend: "openai",
				Model:   "gpt-4o",
				Config: NodeConfig{
					Backend: "openai",
					Model:   "gpt-4o",
				},
			},
		},
		GlobalConfig: DefaultGlobalConfig(),
	}

	if err := store.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	path := filepath.Join(dir, "router-mode.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected yaml file: %v", err)
	}

	got, err := store.Get("router-mode")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Nodes[0].Backend != "openai" || got.Nodes[0].Config.Model != "gpt-4o" {
		t.Fatalf("backend/model not persisted: %+v", got.Nodes[0])
	}

	got.Nodes[0].Backend = "ollama"
	got.Nodes[0].Model = "llama3"
	got.Nodes[0].Config.Backend = "ollama"
	got.Nodes[0].Config.Model = "llama3"
	if err := store.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	again, err := store.Get("router-mode")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if again.Nodes[0].Backend != "ollama" {
		t.Fatalf("update not visible: %q", again.Nodes[0].Backend)
	}

	reg := NewPipelineRegistryWithStore(store)
	if err := reg.LoadFromStore(); err != nil {
		t.Fatalf("LoadFromStore: %v", err)
	}
	if !reg.Exists("router-mode") {
		t.Fatal("registry missing after LoadFromStore")
	}
}

// TestFilePipelineStore_ReloadFromDisk simulates minimal restart: Update → new store → Get.
func TestFilePipelineStore_ReloadFromDisk(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilePipelineStore(dir)
	if err != nil {
		t.Fatalf("NewFilePipelineStore: %v", err)
	}

	p := &AgentPatternPipeline{
		ID:      "education-agent",
		Name:    "教育",
		Version: "1.0",
		Nodes: []PipelineNodeConfig{
			{
				ID:      "tutor",
				Type:    NodeTypeGenerator,
				Name:    "导师",
				Backend: "{{system.default_backend}}",
				Model:   "{{system.default_model}}",
				Config: NodeConfig{
					Backend: "{{system.default_backend}}",
					Model:   "{{system.default_model}}",
				},
			},
		},
		GlobalConfig: DefaultGlobalConfig(),
		Metadata: map[string]interface{}{
			"capability_slots": []map[string]interface{}{
				{"slot_id": "tutor", "node_id": "tutor", "label": "导师"},
			},
		},
	}
	if err := store.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	p.Nodes[0].Backend = "openai"
	p.Nodes[0].Model = "gpt-4o-mini"
	p.Nodes[0].Config.Backend = "openai"
	p.Nodes[0].Config.Model = "gpt-4o-mini"
	if err := store.Update(p); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// New process: open same data dir
	store2, err := NewFilePipelineStore(dir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	got, err := store2.Get("education-agent")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.Nodes[0].Backend != "openai" || got.Nodes[0].Model != "gpt-4o-mini" {
		t.Fatalf("bindings lost after reload: backend=%q model=%q", got.Nodes[0].Backend, got.Nodes[0].Model)
	}
	if got.Metadata == nil {
		t.Fatal("metadata missing after reload")
	}
}
