package llm

import (
	"testing"
)

func TestDefaultChatConfig(t *testing.T) {
	config := DefaultChatConfig()

	if config.Provider != "ollama" {
		t.Errorf("Expected provider to be 'ollama', got '%s'", config.Provider)
	}

	if config.Model != "llama3.2:3b" {
		t.Errorf("Expected model to be 'llama3.2:3b', got '%s'", config.Model)
	}

	if config.BaseURL != "http://localhost:21434" {
		t.Errorf("Expected base_url to be 'http://localhost:21434', got '%s'", config.BaseURL)
	}

	if config.Timeout != 30 {
		t.Errorf("Expected timeout to be 30, got %d", config.Timeout)
	}

	if !config.Enabled {
		t.Error("Expected enabled to be true")
	}
}

func TestNewOllamaChatService(t *testing.T) {
	tests := []struct {
		name    string
		config  *ChatConfig
		wantErr bool
	}{
		{
			name:   "valid config",
			config: &ChatConfig{
				Provider: "ollama",
				Model:    "llama3.2:3b",
				BaseURL:  "http://localhost:21434",
				Timeout:  30,
			},
			wantErr: false,
		},
		{
			name:   "nil config",
			config: nil,
			wantErr: false,
		},
		{
			name:   "empty model",
			config: &ChatConfig{
				Provider: "ollama",
				Model:    "",
			},
			wantErr: false,
		},
		{
			name:   "empty base url",
			config: &ChatConfig{
				Provider: "ollama",
				Model:    "llama3.2:3b",
				BaseURL:  "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewOllamaChatService(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOllamaChatService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if svc == nil {
				t.Error("Expected service to not be nil")
			}
		})
	}
}

func TestOllamaChatService_GetProviderInfo(t *testing.T) {
	config := &ChatConfig{
		Provider: "ollama",
		Model:    "llama3.2:3b",
		BaseURL:  "http://localhost:21434",
	}

	svc, err := NewOllamaChatService(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	info := svc.GetProviderInfo()

	if info.Provider != "ollama" {
		t.Errorf("Expected provider to be 'ollama', got '%s'", info.Provider)
	}

	if info.Model != "llama3.2:3b" {
		t.Errorf("Expected model to be 'llama3.2:3b', got '%s'", info.Model)
	}

	if info.BaseURL != "http://localhost:21434" {
		t.Errorf("Expected base_url to be 'http://localhost:21434', got '%s'", info.BaseURL)
	}
}
