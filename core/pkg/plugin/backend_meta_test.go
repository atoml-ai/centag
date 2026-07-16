package plugin

import (
	"sync"
	"testing"
)

func TestRegisterBackendMeta(t *testing.T) {
	// Clear the registry before test
	backendMetaRegistry = &sync.Map{}

	meta := BackendMeta{
		Type:           "test",
		Name:           "Test Backend",
		DefaultBaseURL: "https://test.example.com",
		KeyHelp:        "Test API Key",
		Capabilities:   []string{"chat", "streaming"},
		AuthSchemes:    []string{"bearer"},
	}

	RegisterBackendMeta(meta)

	// Verify registration
	result, ok := GetBackendMeta("test")
	if !ok {
		t.Fatal("GetBackendMeta failed for registered meta")
	}
	if result.Type != meta.Type {
		t.Errorf("Type = %q, want %q", result.Type, meta.Type)
	}
	if result.Name != meta.Name {
		t.Errorf("Name = %q, want %q", result.Name, meta.Name)
	}
	if result.DefaultBaseURL != meta.DefaultBaseURL {
		t.Errorf("DefaultBaseURL = %q, want %q", result.DefaultBaseURL, meta.DefaultBaseURL)
	}
}

func TestListBackendMetas(t *testing.T) {
	// Clear the registry before test
	backendMetaRegistry = &sync.Map{}

	// Register two metas
	RegisterBackendMeta(BackendMeta{
		Type: "openai",
		Name: "OpenAI",
	})
	RegisterBackendMeta(BackendMeta{
		Type: "anthropic",
		Name: "Anthropic",
	})

	metas := ListBackendMetas()
	if len(metas) != 2 {
		t.Errorf("ListBackendMetas() returned %d metas, want 2", len(metas))
	}

	// Check that both are present
	types := make(map[string]bool)
	for _, m := range metas {
		types[m.Type] = true
	}
	if !types["openai"] {
		t.Error("ListBackendMetas() missing openai")
	}
	if !types["anthropic"] {
		t.Error("ListBackendMetas() missing anthropic")
	}
}

func TestGetBackendMeta_NotFound(t *testing.T) {
	// Clear the registry before test
	backendMetaRegistry = &sync.Map{}

	_, ok := GetBackendMeta("nonexistent")
	if ok {
		t.Error("GetBackendMeta should return false for nonexistent type")
	}
}
