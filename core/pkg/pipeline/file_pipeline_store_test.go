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
