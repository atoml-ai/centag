package server

import (
	"strings"
	"testing"
)

func TestGenerateBackendID_PrefersDistinctiveName(t *testing.T) {
	got := generateBackendID("openai", "Acme LLM Gateway", "https://api.openai.com/v1")
	if got != "acme-llm-gateway" {
		t.Fatalf("got %q, want acme-llm-gateway", got)
	}
}

func TestGenerateBackendID_GenericNameFallsBackToHost(t *testing.T) {
	got := generateBackendID("openai", "OpenAI", "https://api.deepseek.com/v1")
	if got != "api-deepseek-com" {
		t.Fatalf("got %q, want api-deepseek-com", got)
	}
}

func TestGenerateBackendID_AvoidsTypeNameDuplicate(t *testing.T) {
	got := generateBackendID("openai", "OpenAI", "https://api.openai.com/v1")
	if got != "api-openai-com" {
		t.Fatalf("got %q, want api-openai-com (not openai-openai)", got)
	}
}

func TestGenerateBackendID_ChineseNameUsesHost(t *testing.T) {
	got := generateBackendID("openai", "通义千问", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	if got != "dashscope-aliyuncs-com" {
		t.Fatalf("got %q, want dashscope-aliyuncs-com", got)
	}
}

func TestGenerateBackendID_LocalOllamaIncludesPort(t *testing.T) {
	got := generateBackendID("ollama", "Ollama", "http://localhost:11434")
	if got != "localhost-11434" {
		t.Fatalf("got %q, want localhost-11434", got)
	}
}

func TestGenerateBackendID_FallbackShortToken(t *testing.T) {
	got := generateBackendID("openai", "OpenAI", "")
	if !strings.HasPrefix(got, "openai-") || len(got) != len("openai-xxxx") {
		t.Fatalf("got %q, want openai-<4char>", got)
	}
}
