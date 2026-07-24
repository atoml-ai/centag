package agent

import (
	"os"
	"strings"
	"testing"
)

func testInfo() *BackendInfo {
	return &BackendInfo{
		ID:      "test-be-1",
		Name:    "MyProxy",
		BaseURL: "https://api.example.com/v1",
		APIKey:  "sk-test-key",
		Type:    "openai",
		Model:   "gpt-4o",
		Host:    "localhost",
		Port:    20060,
	}
}

func TestClaudeCodeTemplate(t *testing.T) {
	tmpl := &ClaudeCodeTemplate{}
	info := testInfo()

	cmd := tmpl.SetupCommand(info)
	if !strings.Contains(cmd, "ANTHROPIC_BASE_URL") {
		t.Error("missing ANTHROPIC_BASE_URL")
	}
	if !strings.Contains(cmd, "ANTHROPIC_AUTH_TOKEN") {
		t.Error("missing ANTHROPIC_AUTH_TOKEN")
	}
	if !strings.Contains(cmd, info.APIKey) {
		t.Error("missing API key")
	}

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("expected config files")
	}
	for _, f := range files {
		if f.Path == "" {
			t.Error("empty file path")
		}
		if f.Content == "" {
			t.Error("empty file content")
		}
	}
}

func TestCodexTemplate(t *testing.T) {
	tmpl := &CodexTemplate{}
	info := testInfo()

	cmd := tmpl.SetupCommand(info)
	if !strings.Contains(cmd, "config.toml") {
		t.Error("missing config.toml")
	}
	if !strings.Contains(cmd, "auth.json") {
		t.Error("missing auth.json")
	}

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestGeminiTemplate(t *testing.T) {
	tmpl := &GeminiTemplate{}
	info := testInfo()

	cmd := tmpl.SetupCommand(info)
	if !strings.Contains(cmd, "GEMINI_API_KEY") {
		t.Error("missing GEMINI_API_KEY")
	}
	if !strings.Contains(cmd, "GOOGLE_GEMINI_BASE_URL") {
		t.Error("missing GOOGLE_GEMINI_BASE_URL")
	}

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
}

func TestOpenCodeTemplate_UsesOfficialProviderSchema(t *testing.T) {
	tmpl := &OpenCodeTemplate{}
	info := testInfo()
	info.Model = "centag/direct-backend"

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	content := files[0].Content
	if !strings.Contains(content, `"provider": {`) {
		t.Fatalf("opencode config should use top-level provider field, got: %s", content)
	}
	if strings.Contains(content, `"providers": {`) {
		t.Fatalf("opencode config should not use deprecated providers field, got: %s", content)
	}
	if !strings.Contains(content, `"npm": "@ai-sdk/openai-compatible"`) {
		t.Fatalf("opencode config should set npm adapter, got: %s", content)
	}
	// OpenCode 的 model 为 provider/apiModel；apiModel 已是 centag/<id>，故为 centag/centag/<id>
	if !strings.Contains(content, `"model": "centag/centag/direct-backend"`) {
		t.Fatalf("opencode config should set default model with provider prefix, got: %s", content)
	}
	if !strings.Contains(content, `"centag/direct-backend": {`) {
		t.Fatalf("opencode models map key should be API model id, got: %s", content)
	}
}

func TestOpenCodeTemplate_VerifyCommand(t *testing.T) {
	tmpl := &OpenCodeTemplate{}
	info := testInfo()
	info.Model = "centag/direct-backend"
	got := tmpl.VerifyCommand(info)
	want := `opencode run -m centag/centag/direct-backend "Hello, can you hear me?"`
	if got != want {
		t.Fatalf("VerifyCommand = %q, want %q", got, want)
	}
}

func TestOpenClawTemplate_UsesOpenAICompletionsAPI(t *testing.T) {
	tmpl := &OpenClawTemplate{}
	info := testInfo()
	info.Type = "anthropic"
	info.Model = "centag/direct-backend"

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	content := files[0].Content
	if !strings.Contains(content, `"api": "openai-completions"`) {
		t.Fatalf("openclaw config should use openai-completions api, got: %s", content)
	}
	if !strings.Contains(content, `"primary": "centag/centag/direct-backend"`) {
		t.Fatalf("openclaw config should set primary default model, got: %s", content)
	}
	if !strings.Contains(content, `"id": "centag/direct-backend"`) {
		t.Fatalf("openclaw models id should be API model id, got: %s", content)
	}
}

func TestHermesTemplate_ConfigUsesSingleModelName(t *testing.T) {
	tmpl := &HermesTemplate{}
	info := testInfo()
	info.Model = "centag/direct-backend"

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	content := files[0].Content
	if strings.Contains(content, "centag/direct-backend ") {
		t.Fatalf("config should not contain model suffix after pipeline id: %s", content)
	}
	if !strings.Contains(content, `model: "centag/direct-backend"`) {
		t.Fatalf("config should set custom provider model to pipeline id, got: %s", content)
	}
	if !strings.Contains(content, `api_mode: "chat_completions"`) {
		t.Fatalf("config should contain chat_completions api mode, got: %s", content)
	}
}

func TestTemplateRegistry(t *testing.T) {
	r := NewTemplateRegistry()
	types := r.List()
	if len(types) == 0 {
		t.Fatal("expected template types")
	}
	for _, at := range types {
		tmpl, ok := r.Get(at)
		if !ok {
			t.Fatalf("template not found for %s", at)
		}
		if tmpl.DisplayName() == "" {
			t.Errorf("empty display name for %s", at)
		}
	}
}

func TestProxyURL(t *testing.T) {
	url := proxyURL("", 0)
	if url != "http://localhost:20060/v1" {
		t.Errorf("unexpected default URL: %s", url)
	}

	url = proxyURL("192.168.1.100", 30060)
	if url != "http://192.168.1.100:30060/v1" {
		t.Errorf("unexpected custom URL: %s", url)
	}
}

func TestWriteAndRestoreConfigFiles(t *testing.T) {
	dir := t.TempDir()
	target := dir + "/opencode.json"
	original := `{"model":"original"}`
	if err := os.WriteFile(target, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	files := []ConfigFile{{Path: target, Content: `{"model":"centag"}`}}
	if err := writeFiles(files); err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"model":"centag"}` {
		t.Fatalf("after write got %s", got)
	}
	bak, err := os.ReadFile(backupPath(target))
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bak) != original {
		t.Fatalf("backup = %s, want %s", bak, original)
	}

	results, err := RestoreConfigFiles(files)
	if err != nil {
		t.Fatalf("RestoreConfigFiles: %v", err)
	}
	if len(results) != 1 || results[0].Action != "restored" {
		t.Fatalf("unexpected results: %+v", results)
	}
	restored, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Fatalf("restored = %s, want %s", restored, original)
	}
	if fileExists(backupPath(target)) {
		t.Fatal("backup should be removed after restore")
	}
}

func TestRestoreConfigFiles_RemovesCentagCreated(t *testing.T) {
	dir := t.TempDir()
	target := dir + "/new.json"
	files := []ConfigFile{{Path: target, Content: `{"centag":true}`}}
	if err := writeFiles(files); err != nil {
		t.Fatal(err)
	}
	results, err := RestoreConfigFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "removed" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if fileExists(target) {
		t.Fatal("centag-created file should be removed")
	}
}
