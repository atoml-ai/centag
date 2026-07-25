package agent

import (
	"os"
	"path/filepath"
	"runtime"
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

	meta := tmpl.Meta()
	if meta.WriteMode != WriteModeOverwrite {
		t.Fatalf("write mode = %s", meta.WriteMode)
	}
	if meta.InstallURL == "" {
		t.Fatal("missing install url")
	}

	cmd := tmpl.SetupCommand(info)
	if !strings.Contains(cmd, "ANTHROPIC_BASE_URL") {
		t.Error("missing ANTHROPIC_BASE_URL")
	}
	if !strings.Contains(cmd, "ANTHROPIC_AUTH_TOKEN") {
		t.Error("missing ANTHROPIC_AUTH_TOKEN")
	}

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "~/.claude/settings.json" {
		t.Fatalf("path = %s", files[0].Path)
	}
	if !strings.Contains(files[0].Content, "ANTHROPIC_AUTH_TOKEN") {
		t.Fatal("settings.json should contain ANTHROPIC_AUTH_TOKEN")
	}
}

func TestClaudeCodeWriteConfig_MergesEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"env":{"KEEP":"1","ANTHROPIC_MODEL":"old"},"permissions":{"allow":true}}`), 0644); err != nil {
		t.Fatal(err)
	}

	tmpl := &ClaudeCodeTemplate{}
	if err := tmpl.WriteConfig(testInfo()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, `"KEEP": "1"`) && !strings.Contains(got, `"KEEP":"1"`) {
		t.Fatalf("should preserve unrelated env: %s", got)
	}
	if !strings.Contains(got, "ANTHROPIC_BASE_URL") {
		t.Fatalf("missing base url: %s", got)
	}
	if !strings.Contains(got, "permissions") {
		t.Fatalf("should preserve top-level keys: %s", got)
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
	meta := tmpl.Meta()
	if meta.InstallURL == "" || len(meta.ConfigPaths) != 2 {
		t.Fatalf("bad meta: %+v", meta)
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
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	foundSettings := false
	for _, f := range files {
		if f.Path == "~/.gemini/settings.json" {
			foundSettings = true
			if !strings.Contains(f.Content, "gemini-api-key") {
				t.Fatalf("settings should set gemini-api-key: %s", f.Content)
			}
		}
	}
	if !foundSettings {
		t.Fatal("missing settings.json")
	}
}

func TestGrokBuildTemplate(t *testing.T) {
	tmpl := &GrokBuildTemplate{}
	files, err := tmpl.ConfigFiles(testInfo())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "~/.grok/config.toml" {
		t.Fatalf("unexpected files: %+v", files)
	}
	if !strings.Contains(files[0].Content, `[model."centag"]`) {
		t.Fatalf("missing model.centag: %s", files[0].Content)
	}
	if !strings.Contains(files[0].Content, `api_backend = "responses"`) {
		t.Fatalf("missing api_backend: %s", files[0].Content)
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
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
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
	if !strings.Contains(content, `"model": "centag/centag/direct-backend"`) {
		t.Fatalf("opencode config should set default model with provider prefix, got: %s", content)
	}
	if tmpl.Meta().WriteMode != WriteModeMerge {
		t.Fatalf("opencode should be merge mode")
	}
}

func TestOpenCodeWriteConfig_PreservesOtherProviders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	existing := `{
  "provider": {
    "openai": {"npm": "@ai-sdk/openai", "name": "OpenAI"}
  },
  "model": "openai/gpt-4o"
}`
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	info := testInfo()
	info.Model = "centag/direct-backend"
	if err := (&OpenCodeTemplate{}).WriteConfig(info); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, `"openai"`) {
		t.Fatalf("should keep openai provider: %s", got)
	}
	if !strings.Contains(got, `"centag"`) {
		t.Fatalf("should add centag provider: %s", got)
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
	if tmpl.Meta().WriteMode != WriteModeMerge {
		t.Fatal("openclaw should be merge")
	}
}

func TestOpenClawWriteConfig_PreservesOtherProviders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	path := filepath.Join(home, ".openclaw", "openclaw.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	existing := `{"models":{"providers":{"other":{"baseUrl":"https://x"}}}}`
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}
	info := testInfo()
	info.Model = "centag/p1"
	if err := (&OpenClawTemplate{}).WriteConfig(info); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	s := string(got)
	if !strings.Contains(s, `"other"`) || !strings.Contains(s, `"centag"`) {
		t.Fatalf("merge failed: %s", s)
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
	if tmpl.Meta().WriteMode != WriteModeMerge {
		t.Fatal("hermes should be merge")
	}
}

func TestHermesWriteConfig_PreservesOtherProviders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	path := filepath.Join(home, ".hermes", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	existing := `custom_providers:
  - name: other
    base_url: https://example.com
    api_key: x
`
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}
	if err := (&HermesTemplate{}).WriteConfig(testInfo()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	s := string(got)
	if !strings.Contains(s, "other") || !strings.Contains(s, "centag") {
		t.Fatalf("merge failed: %s", s)
	}
}

func TestTemplateRegistry_MetaComplete(t *testing.T) {
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
		meta := tmpl.Meta()
		if meta.WriteMode == "" {
			t.Errorf("%s missing write_mode", at)
		}
		if meta.ConfigMethod == "" {
			t.Errorf("%s missing config_method", at)
		}
		switch meta.WriteMode {
		case WriteModeNone:
			// ok
		case WriteModeOverwrite, WriteModeMerge:
			if len(meta.ConfigPaths) == 0 && at != AgentClaudeDesktop {
				// Claude Desktop may have empty paths on unsupported OS
				if runtime.GOOS == "linux" && at == AgentClaudeDesktop {
					break
				}
				if at != AgentClaudeDesktop || runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
					if len(meta.ConfigPaths) == 0 {
						t.Errorf("%s missing config_paths", at)
					}
				}
			}
			if meta.InstallURL == "" && meta.WriteMode != WriteModeNone {
				t.Errorf("%s missing install_url", at)
			}
		default:
			t.Errorf("%s unknown write_mode %s", at, meta.WriteMode)
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

func TestShellQuote(t *testing.T) {
	got := shellQuote(`/Users/a/Library/Application Support/Claude`)
	if got != `'/Users/a/Library/Application Support/Claude'` {
		t.Fatalf("got %s", got)
	}
}

func TestChatCompletionsURL(t *testing.T) {
	got := chatCompletionsURL("localhost", 20060)
	want := "http://localhost:20060/v1/chat/completions"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCodeBuddyTemplate_ConfigUsesFullChatURL(t *testing.T) {
	tmpl := &CodeBuddyTemplate{}
	info := testInfo()
	info.Model = "centag/direct-backend"
	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "~/.codebuddy/models.json" {
		t.Fatalf("unexpected files: %+v", files)
	}
	if !strings.Contains(files[0].Content, `/v1/chat/completions`) {
		t.Fatalf("url must be full chat completions path: %s", files[0].Content)
	}
	if !strings.Contains(files[0].Content, `"id": "centag/direct-backend"`) {
		t.Fatalf("missing model id: %s", files[0].Content)
	}
	if tmpl.Meta().WriteMode != WriteModeMerge {
		t.Fatal("codebuddy should merge")
	}
}

func TestCodeBuddyWriteConfig_PreservesOtherModels(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	path := filepath.Join(home, ".codebuddy", "models.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	existing := `{"models":[{"id":"other","name":"Other","url":"https://x/v1/chat/completions","apiKey":"k"}]}`
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}
	info := testInfo()
	info.Model = "centag/p1"
	if err := (&CodeBuddyTemplate{}).WriteConfig(info); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	s := string(got)
	if !strings.Contains(s, `"other"`) || !strings.Contains(s, `"centag/p1"`) {
		t.Fatalf("merge failed: %s", s)
	}
}

func TestWorkBuddySharesCodeBuddyPath(t *testing.T) {
	wb := &WorkBuddyTemplate{}
	if wb.Meta().ConfigPaths[0] != codeBuddyModelsPath {
		t.Fatal("workbuddy should share codebuddy models.json")
	}
	files, err := wb.ConfigFiles(testInfo())
	if err != nil || len(files) != 1 {
		t.Fatalf("files: %v %v", files, err)
	}
}

func TestTraeTemplate_MetaAndConfig(t *testing.T) {
	tmpl := &TraeTemplate{}
	meta := tmpl.Meta()
	if meta.WriteMode != WriteModeMerge || meta.InstallURL == "" {
		t.Fatalf("bad meta: %+v", meta)
	}
	info := testInfo()
	info.Model = "centag/direct-backend"
	files, err := tmpl.ConfigFiles(info)
	if err != nil || len(files) == 0 {
		t.Fatalf("config files: %v %v", files, err)
	}
	if !strings.Contains(files[0].Content, `"trae.customModels"`) {
		t.Fatalf("missing trae.customModels: %s", files[0].Content)
	}
	if !strings.Contains(files[0].Content, `http://localhost:20060/v1"`) {
		t.Fatalf("base url should be /v1 without chat/completions: %s", files[0].Content)
	}
	if strings.Contains(files[0].Content, `/v1/chat/completions`) {
		t.Fatalf("trae baseUrl should not include chat/completions by default: %s", files[0].Content)
	}
}

func TestTraeWriteConfig_MergesCustomModels(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
		t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	}
	var settings string
	switch runtime.GOOS {
	case "darwin":
		settings = filepath.Join(home, "Library", "Application Support", "Trae", "User", "settings.json")
	case "windows":
		settings = filepath.Join(home, "AppData", "Roaming", "Trae", "User", "settings.json")
	default:
		settings = filepath.Join(home, ".config", "Trae", "User", "settings.json")
	}
	if err := os.MkdirAll(filepath.Dir(settings), 0755); err != nil {
		t.Fatal(err)
	}
	// 创建父级 Trae 目录以触发「已安装」检测
	if err := os.WriteFile(settings, []byte(`{"editor.fontSize":14,"trae.customModels":[{"id":"keep","modelId":"keep"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	info := testInfo()
	info.Model = "centag/p1"
	if err := (&TraeTemplate{}).WriteConfig(info); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, `"keep"`) || !strings.Contains(s, `"centag/p1"`) {
		t.Fatalf("merge failed: %s", s)
	}
}
