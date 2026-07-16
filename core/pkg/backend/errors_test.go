package backend

import (
	"errors"
	"strings"
	"testing"
)

func TestIsUsableLLMBackend(t *testing.T) {
	tests := []struct {
		name string
		cfg  *BackendConfig
		want bool
	}{
		{name: "nil", cfg: nil, want: false},
		{name: "disabled", cfg: &BackendConfig{Enabled: false, Type: "openai", BaseURL: "https://api.openai.com", APIKey: "sk"}, want: false},
		{name: "openai ok", cfg: &BackendConfig{Enabled: true, Type: "openai", BaseURL: "https://api.openai.com", APIKey: "sk-test"}, want: true},
		{name: "openai no key", cfg: &BackendConfig{Enabled: true, Type: "openai", BaseURL: "https://api.openai.com"}, want: false},
		{name: "ollama no key", cfg: &BackendConfig{Enabled: true, Type: "ollama", BaseURL: "http://localhost:11434"}, want: true},
		{name: "ollama no url", cfg: &BackendConfig{Enabled: true, Type: "ollama"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUsableLLMBackend(tt.cfg); got != tt.want {
				t.Fatalf("IsUsableLLMBackend() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasUsableLLMBackend(t *testing.T) {
	m := NewManager()
	if m.HasUsableLLMBackend() {
		t.Fatal("empty manager should have no usable backend")
	}
	if err := m.Add(&BackendConfig{ID: "o1", Type: "openai", BaseURL: "https://x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if m.HasUsableLLMBackend() {
		t.Fatal("openai without key should not count as usable")
	}
	if err := m.Update(&BackendConfig{ID: "o1", Type: "openai", BaseURL: "https://x", APIKey: "sk", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if !m.HasUsableLLMBackend() {
		t.Fatal("expected usable after api key set")
	}
}

func TestClassifyClientError(t *testing.T) {
	ce := ClassifyClientError(NewNoUsableBackendError(errors.New("no enabled backends found")))
	if ce == nil || ce.Code != ErrorCodeNoBackendConfigured {
		t.Fatalf("got %#v", ce)
	}
	if !strings.Contains(ce.Message, "后端管理") {
		t.Fatalf("message should guide user: %s", ce.Message)
	}

	// 全局无后端时：not found → 未配置
	ce = ClassifyClientError(fmtWrap("backend \"openai\" not found: backend with id openai not found"))
	if ce == nil || ce.Code != ErrorCodeNoBackendConfigured {
		t.Fatalf("not-found with empty manager should map to no_backend_configured, got %#v", ce)
	}

	// 已有启用后端时：不吞掉 not found（避免误报「未配置」）
	m := NewManager()
	_ = m.Add(&BackendConfig{ID: "openai", Type: "openai", BaseURL: "https://x", APIKey: "sk", Enabled: true})
	// GetManager may be a different singleton; simulate by checking HasEnabled on local mgr via direct path:
	// ClassifyClientError uses GetManager(); ensure singleton has a backend for this assertion.
	gm := GetManager()
	_ = gm.Add(&BackendConfig{ID: "cr-test-openai", Type: "openai", BaseURL: "https://x", APIKey: "sk", Enabled: true})
	t.Cleanup(func() { _ = gm.Delete("cr-test-openai") })
	if ClassifyClientError(fmtWrap(`backend "missing" not found`)) != nil {
		t.Fatal("not-found must not classify when enabled backends exist")
	}
	_ = m // keep for readability of scenario above

	ce = ClassifyClientError(NewNoBackendAPIKeyError("openai"))
	if ce == nil || ce.Code != ErrorCodeNoBackendAPIKey {
		t.Fatalf("got %#v", ce)
	}

	if ClassifyClientError(errors.New("timeout talking to upstream")) != nil {
		t.Fatal("unrelated errors must not be classified")
	}
}

func fmtWrap(s string) error { return errors.New(s) }

func TestOpenAIErrorBody(t *testing.T) {
	body := OpenAIErrorBody(ErrorCodeNoBackendConfigured, ClientHintNoBackendConfigured)
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("body = %#v", body)
	}
	if errObj["type"] != "centag_configuration_error" {
		t.Fatalf("type = %v", errObj["type"])
	}
	if errObj["code"] != ErrorCodeNoBackendConfigured {
		t.Fatalf("code = %v", errObj["code"])
	}
}

func TestSelectDefaultBackend_NoEnabledUsesClientError(t *testing.T) {
	m := NewManager()
	_, err := m.SelectDefaultBackend()
	ce := ClassifyClientError(err)
	if ce == nil || ce.Code != ErrorCodeNoBackendConfigured {
		t.Fatalf("got %#v / %v", ce, err)
	}
	if !errors.Is(err, ErrNoUsableBackend) {
		t.Fatalf("errors.Is(ErrNoUsableBackend) = false for %v", err)
	}
}
