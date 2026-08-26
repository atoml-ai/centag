package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testInfo() *BackendInfo {
	// 代理模式夹具：BaseURL 留空，模板应回退到 Centag 代理地址 host:port（localhost:20060）。
	return &BackendInfo{
		ID:      "test-be-1",
		Name:    "MyProxy",
		BaseURL: "",
		APIKey:  "sk-test-key",
		Type:    "openai",
		Model:   "gpt-4o",
		Host:    "localhost",
		Port:    20060,
	}
}

// testInfoDirect 直连模式夹具：BaseURL 为真实后端地址，模板应直接写入该地址。
func testInfoDirect() *BackendInfo {
	return &BackendInfo{
		ID:      "test-be-2",
		Name:    "MyBackend",
		BaseURL: "https://api.direct.example.com/v1",
		APIKey:  "sk-direct-key",
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

	meta := tmpl.Meta()
	if meta.WriteMode != WriteModeOverwrite {
		t.Fatalf("write mode = %s", meta.WriteMode)
	}
	if !meta.VerifiedWrite {
		t.Error("Gemini write config should be verified")
	}
	if !meta.VerifiedWrap {
		t.Error("Gemini wrap should be verified")
	}

	cmd := tmpl.SetupCommand(info)
	if !strings.Contains(cmd, "GEMINI_API_KEY") {
		t.Error("missing GEMINI_API_KEY")
	}
	if !strings.Contains(cmd, "GOOGLE_GEMINI_BASE_URL") {
		t.Error("missing GOOGLE_GEMINI_BASE_URL")
	}
	// GOOGLE_GEMINI_BASE_URL must NOT contain /v1, otherwise Gemini CLI appends /v1beta → /v1/v1beta 404.
	if strings.Contains(cmd, "GOOGLE_GEMINI_BASE_URL=http://localhost:20060/v1") {
		t.Error("base URL should not include /v1 suffix")
	}
	if !strings.Contains(cmd, "gemini-3.1-flash-lite") {
		t.Error("expected default Gemini model")
	}

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	foundSettings := false
	foundEnv := false
	for _, f := range files {
		if f.Path == "~/.gemini/settings.json" {
			foundSettings = true
			if !strings.Contains(f.Content, "gemini-api-key") {
				t.Fatalf("settings should set gemini-api-key: %s", f.Content)
			}
		}
		if f.Path == "~/.gemini/.env" {
			foundEnv = true
			if strings.Contains(f.Content, "GOOGLE_GEMINI_BASE_URL=http://localhost:20060/v1") {
				t.Fatalf("env base URL should not include /v1 suffix: %s", f.Content)
			}
			if !strings.Contains(f.Content, "GEMINI_MODEL=gemini-3.1-flash-lite") {
				t.Fatalf("env should use real Gemini model: %s", f.Content)
			}
		}
	}
	if !foundSettings {
		t.Fatal("missing settings.json")
	}
	if !foundEnv {
		t.Fatal("missing .env")
	}
}

func TestGeminiTemplate_UsesExplicitGeminiModel(t *testing.T) {
	tmpl := &GeminiTemplate{}
	info := testInfo()
	info.Model = "gemini-2.5-flash"

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "~/.gemini/.env" && !strings.Contains(f.Content, "GEMINI_MODEL=gemini-2.5-flash") {
			t.Fatalf("should use explicit gemini model: %s", f.Content)
		}
	}
}

func TestGeminiTemplate_DropsVirtualModel(t *testing.T) {
	tmpl := &GeminiTemplate{}
	info := testInfo()
	info.Model = "centag/transparent-proxy"

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "~/.gemini/.env" {
			if strings.Contains(f.Content, "centag/transparent-proxy") {
				t.Fatalf("should not use virtual model in Gemini config: %s", f.Content)
			}
			if !strings.Contains(f.Content, "GEMINI_MODEL=gemini-3.1-flash-lite") {
				t.Fatalf("should fall back to default Gemini model: %s", f.Content)
			}
		}
	}
}

func TestGeminiWriteConfig_MergesAuthSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	envPath := filepath.Join(home, ".gemini", ".env")
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"general":{"vimMode":true}}`), 0644); err != nil {
		t.Fatal(err)
	}

	tmpl := &GeminiTemplate{}
	if err := tmpl.WriteConfig(testInfo()); err != nil {
		t.Fatal(err)
	}
	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(envData), "/v1") {
		t.Fatalf("env base URL should not include /v1: %s", envData)
	}
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(settingsData)
	if !strings.Contains(got, "gemini-api-key") {
		t.Fatalf("missing gemini-api-key auth: %s", got)
	}
	if !strings.Contains(got, "vimMode") {
		t.Fatalf("should preserve existing settings keys: %s", got)
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

func TestPiTemplate_UsesOpenAICompletionsAPI(t *testing.T) {
	tmpl := &PiTemplate{}
	info := testInfo()
	info.Model = "centag/direct-backend"

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	modelsContent := files[0].Content
	if files[0].Path != "~/.pi/agent/models.json" {
		t.Fatalf("models path = %s", files[0].Path)
	}
	if !strings.Contains(modelsContent, `"api": "openai-completions"`) {
		t.Fatalf("pi models.json should use openai-completions, got: %s", modelsContent)
	}
	if !strings.Contains(modelsContent, `"id": "centag/direct-backend"`) {
		t.Fatalf("pi models.json should set model id, got: %s", modelsContent)
	}
	settingsContent := files[1].Content
	if files[1].Path != "~/.pi/agent/settings.json" {
		t.Fatalf("settings path = %s", files[1].Path)
	}
	if !strings.Contains(settingsContent, `"defaultProvider": "centag"`) {
		t.Fatalf("settings should set defaultProvider, got: %s", settingsContent)
	}
	if !strings.Contains(settingsContent, `"defaultModel": "centag/direct-backend"`) {
		t.Fatalf("settings should set defaultModel, got: %s", settingsContent)
	}
	if tmpl.Meta().WriteMode != WriteModeMerge {
		t.Fatal("pi should be merge")
	}
	wantVerify := `pi -p --model centag/centag/direct-backend "Hello, can you hear me?"`
	if got := tmpl.VerifyCommand(info); got != wantVerify {
		t.Fatalf("VerifyCommand = %q, want %q", got, wantVerify)
	}
}

func TestPiWriteConfig_PreservesOtherProviders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	modelsPath := filepath.Join(home, ".pi", "agent", "models.json")
	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	if err := os.MkdirAll(filepath.Dir(modelsPath), 0755); err != nil {
		t.Fatal(err)
	}
	existingModels := `{"providers":{"other":{"baseUrl":"https://x","api":"openai-completions","models":[{"id":"m1"}]}}}`
	if err := os.WriteFile(modelsPath, []byte(existingModels), 0644); err != nil {
		t.Fatal(err)
	}
	existingSettings := `{"theme":"dark","defaultProvider":"other","defaultModel":"m1"}`
	if err := os.WriteFile(settingsPath, []byte(existingSettings), 0644); err != nil {
		t.Fatal(err)
	}

	info := testInfo()
	info.Model = "centag/p1"
	if err := (&PiTemplate{}).WriteConfig(info); err != nil {
		t.Fatal(err)
	}

	modelsGot, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatal(err)
	}
	ms := string(modelsGot)
	if !strings.Contains(ms, `"other"`) || !strings.Contains(ms, `"centag"`) {
		t.Fatalf("models merge failed: %s", ms)
	}

	settingsGot, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	ss := string(settingsGot)
	if !strings.Contains(ss, `"theme": "dark"`) && !strings.Contains(ss, `"theme":"dark"`) {
		t.Fatalf("settings should preserve theme: %s", ss)
	}
	if !strings.Contains(ss, `"defaultProvider": "centag"`) && !strings.Contains(ss, `"defaultProvider":"centag"`) {
		t.Fatalf("settings should set defaultProvider: %s", ss)
	}
	if !strings.Contains(ss, `"defaultModel": "centag/p1"`) && !strings.Contains(ss, `"defaultModel":"centag/p1"`) {
		t.Fatalf("settings should set defaultModel: %s", ss)
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

func TestAccessMatrixComplete(t *testing.T) {
	r := NewTemplateRegistry()
	for _, at := range r.List() {
		tmpl, ok := r.Get(at)
		if !ok {
			t.Fatalf("missing %s", at)
		}
		meta := tmpl.Meta().Normalize()
		if len(meta.AccessMethods) == 0 {
			t.Errorf("%s: access_methods empty after Normalize", at)
		}
		hasWrap := meta.HasAccess(AccessWrapCLI)
		hasCLI := meta.CompanionCLI != nil && strings.TrimSpace(meta.CompanionCLI.Binary) != ""
		if hasWrap != hasCLI {
			t.Errorf("%s: wrap_cli=%v companion=%v", at, hasWrap, hasCLI)
		}
		if meta.HasAccess(AccessWriteConfig) {
			if meta.WriteMode != WriteModeMerge && meta.WriteMode != WriteModeOverwrite {
				t.Errorf("%s: write_config requires merge/overwrite, got %s", at, meta.WriteMode)
			}
		}
		if meta.HasAccess(AccessUIGuide) && meta.UIGuide == nil {
			t.Errorf("%s: ui_guide access without UIGuide payload", at)
		}
		if meta.HasAccess(AccessBuiltin) && meta.HasAccess(AccessWriteConfig) {
			t.Errorf("%s: builtin should not also declare write_config", at)
		}
	}

	trae := (&TraeTemplate{}).Meta().Normalize()
	if trae.HasAccess(AccessWriteConfig) {
		t.Fatal("trae must not declare write_config")
	}
	if trae.UIGuide == nil || trae.UIGuide.RequestURLKind != RequestURLOpenAIBase {
		t.Fatal("trae request URL must be openai_base (…/v1)")
	}
	wb := (&WorkBuddyTemplate{}).Meta().Normalize()
	if wb.UIGuide == nil || wb.UIGuide.RequestURLKind != RequestURLOpenAIBase {
		t.Fatal("workbuddy request URL must default to openai_base (…/v1)")
	}
	if wb.UIGuide.URLHint == "" {
		t.Fatal("workbuddy should hint that …/chat/completions also works")
	}
	cb := (&CodeBuddyTemplate{}).Meta().Normalize()
	if !cb.HasAccess(AccessWriteConfig) || !cb.HasAccess(AccessWrapCLI) {
		t.Fatalf("codebuddy should be write+wrap: %#v", cb.AccessMethods)
	}

	for _, pair := range []struct {
		name string
		meta AgentSetupMeta
	}{
		{"hermes", (&HermesTemplate{}).Meta()},
		{"openclaw", (&OpenClawTemplate{}).Meta()},
		{"opencode", (&OpenCodeTemplate{}).Meta()},
		{"pi", (&PiTemplate{}).Meta()},
	} {
		if !pair.meta.VerifiedWrite || !pair.meta.VerifiedWrap {
			t.Errorf("%s: expected VerifiedWrite+VerifiedWrap, got write=%v wrap=%v",
				pair.name, pair.meta.VerifiedWrite, pair.meta.VerifiedWrap)
		}
	}

	oc := (&OpenClawTemplate{}).Meta().Normalize()
	wantArgv := []string{"openclaw", "tui", "--local"}
	gotArgv := oc.WrapArgv()
	if len(gotArgv) != len(wantArgv) {
		t.Fatalf("openclaw wrap argv: got %#v want %#v", gotArgv, wantArgv)
	}
	for i := range wantArgv {
		if gotArgv[i] != wantArgv[i] {
			t.Fatalf("openclaw wrap argv: got %#v want %#v", gotArgv, wantArgv)
		}
	}
	if strings.TrimSpace(oc.CompanionCLI.Note) == "" {
		t.Fatal("openclaw companion_cli.note should explain LaunchAgent / gateway stop")
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

// TestEndpointHelpers 验证直连/代理两种模式下 URL 与 model 引用助手的行为。
func TestEndpointHelpers(t *testing.T) {
	// 代理模式：BaseURL 为空 → 回退 Centag 代理地址（含 /v1）
	proxy := testInfo()
	if got := endpointURL(proxy); got != "http://localhost:20060/v1" {
		t.Fatalf("endpointURL(proxy) = %s", got)
	}
	if got := endpointHostRoot(proxy); got != "http://localhost:20060" {
		t.Fatalf("endpointHostRoot(proxy) = %s", got)
	}

	// 直连模式：BaseURL 已填 → 原样使用真实地址
	direct := testInfoDirect()
	if got := endpointURL(direct); got != "https://api.direct.example.com/v1" {
		t.Fatalf("endpointURL(direct) = %s", got)
	}
	// host 根需去掉 /v1、/v1beta 后缀，避免 Claude/Gemini 自行拼接路径时双重前缀
	if got := endpointHostRoot(direct); got != "https://api.direct.example.com" {
		t.Fatalf("endpointHostRoot(direct) = %s", got)
	}
	if got := endpointHostRoot(&BackendInfo{BaseURL: "https://generativelanguage.googleapis.com/v1beta"}); got != "https://generativelanguage.googleapis.com" {
		t.Fatalf("endpointHostRoot(v1beta) = %s", got)
	}

	// model 引用：虚拟模型保留 centag/ 前缀，真实模型原样使用（不再强加前缀）
	if got := agentModelRef("centag/p1"); got != "centag/centag/p1" {
		t.Fatalf("agentModelRef(virtual) = %s", got)
	}
	if got := agentModelRef("gpt-4o"); got != "gpt-4o" {
		t.Fatalf("agentModelRef(real) = %s", got)
	}
	if got := agentModelRef("gemini-2.0-flash"); got != "gemini-2.0-flash" {
		t.Fatalf("agentModelRef(gemini) = %s", got)
	}
}

// TestOpenCodeTemplate_DirectMode 验证直连模式把真实 BaseURL 与真实模型名写入配置。
func TestOpenCodeTemplate_DirectMode(t *testing.T) {
	info := testInfoDirect()
	files, err := (&OpenCodeTemplate{}).ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	c := files[0].Content
	if !strings.Contains(c, `"baseURL": "https://api.direct.example.com/v1"`) {
		t.Fatalf("direct mode should write real baseURL: %s", c)
	}
	if !strings.Contains(c, `"apiKey": "sk-direct-key"`) {
		t.Fatalf("direct mode should write real apiKey: %s", c)
	}
	// 真实模型名不应被强加 centag/ 前缀
	if strings.Contains(c, `"gpt-4o"`) == false {
		t.Fatalf("direct mode should keep real model name gpt-4o: %s", c)
	}
	if strings.Contains(c, `centag/gpt-4o`) {
		t.Fatalf("direct mode must not prefix centag/ on real model: %s", c)
	}
}

// TestClaudeCodeDirectMode 验证直连模式 Claude Code 写入真实后端 host 根（无 /v1）。
func TestClaudeCodeDirectMode(t *testing.T) {
	info := testInfoDirect()
	files, err := (&ClaudeCodeTemplate{}).ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	c := files[0].Content
	if !strings.Contains(c, `"ANTHROPIC_BASE_URL": "https://api.direct.example.com"`) {
		t.Fatalf("claude direct mode should use host root without /v1: %s", c)
	}
	if !strings.Contains(c, `"ANTHROPIC_AUTH_TOKEN": "sk-direct-key"`) {
		t.Fatalf("claude direct mode should write real apiKey: %s", c)
	}
}

// TestCodeBuddyDirectMode 验证直连模式 CodeBuddy 写入完整 chat/completions 真实地址。
func TestCodeBuddyDirectMode(t *testing.T) {
	info := testInfoDirect()
	files, err := (&CodeBuddyTemplate{}).ConfigFiles(info)
	if err != nil {
		t.Fatal(err)
	}
	c := files[0].Content
	if !strings.Contains(c, `"url": "https://api.direct.example.com/v1/chat/completions"`) {
		t.Fatalf("codebuddy direct mode should write full chat url: %s", c)
	}
	if !strings.Contains(c, `"apiKey": "sk-direct-key"`) {
		t.Fatalf("codebuddy direct mode should write real apiKey: %s", c)
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

func TestWorkBuddyUIGuide(t *testing.T) {
	wb := &WorkBuddyTemplate{}
	meta := wb.Meta().Normalize()
	if meta.WriteMode != WriteModeNone {
		t.Fatalf("write_mode=%s, want none", meta.WriteMode)
	}
	if !meta.HasAccess(AccessUIGuide) || meta.HasAccess(AccessWriteConfig) || meta.HasAccess(AccessWrapCLI) {
		t.Fatalf("access methods: %#v", meta.AccessMethods)
	}
	if meta.UIGuide == nil || meta.UIGuide.Summary == "" {
		t.Fatal("expected UIGuide summary")
	}
	info := testInfo()
	info.Model = "centag/p1"
	files, err := wb.ConfigFiles(info)
	if err != nil || len(files) != 1 {
		t.Fatalf("files: %v %v", files, err)
	}
	if !strings.Contains(files[0].Content, "http://localhost:20060/v1") || !strings.Contains(files[0].Content, "centag/p1") {
		t.Fatalf("guide incomplete: %s", files[0].Content)
	}
	if err := wb.WriteConfig(info); err != nil {
		t.Fatal(err)
	}
}

func TestTraeTemplate_MetaAndConfig(t *testing.T) {
	tmpl := &TraeTemplate{}
	meta := tmpl.Meta().Normalize()
	if meta.WriteMode != WriteModeNone || meta.InstallURL == "" {
		t.Fatalf("bad meta: %+v", meta)
	}
	if !meta.HasAccess(AccessUIGuide) || meta.HasAccess(AccessWriteConfig) || meta.HasAccess(AccessWrapCLI) {
		t.Fatalf("trae access methods: %#v", meta.AccessMethods)
	}
	if meta.UIGuide == nil || meta.UIGuide.Summary == "" {
		t.Fatal("trae should provide UIGuide summary")
	}
	info := testInfo()
	info.Model = "centag/direct-backend"
	files, err := tmpl.ConfigFiles(info)
	if err != nil || len(files) == 0 {
		t.Fatalf("config files: %v %v", files, err)
	}
	content := files[0].Content
	if !strings.Contains(content, `http://localhost:20060/v1`) {
		t.Fatalf("guide should use OpenAI base …/v1: %s", content)
	}
	if strings.Contains(content, "http://localhost:20060/v1/chat/completions") {
		t.Fatalf("guide must not recommend chat/completions as primary URL: %s", content)
	}
	if meta.UIGuide == nil || meta.UIGuide.RequestURLKind != RequestURLOpenAIBase || meta.UIGuide.FullURLMode != "off" {
		t.Fatalf("trae UIGuide url kind: %+v", meta.UIGuide)
	}
	if !strings.Contains(content, `centag/direct-backend`) {
		t.Fatalf("guide should include model id: %s", content)
	}
	if !strings.Contains(content, `设置 → 模型`) && !strings.Contains(strings.ToLower(content), `settings`) {
		t.Fatalf("guide should mention UI settings: %s", content)
	}
	if strings.Contains(content, `"trae.customModels"`) {
		t.Fatalf("guide must not claim settings.json customModels works: %s", content)
	}
}

func TestTraeWriteConfig_WritesSetupGuide(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
		t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	}
	var guide string
	switch runtime.GOOS {
	case "darwin":
		guide = filepath.Join(home, "Library", "Application Support", "Trae", "CENTAG_SETUP.md")
	case "windows":
		guide = filepath.Join(home, "AppData", "Roaming", "Trae", "CENTAG_SETUP.md")
	default:
		guide = filepath.Join(home, ".config", "Trae", "CENTAG_SETUP.md")
	}
	if err := os.MkdirAll(filepath.Dir(guide), 0755); err != nil {
		t.Fatal(err)
	}
	info := testInfo()
	info.Model = "centag/p1"
	if err := (&TraeTemplate{}).WriteConfig(info); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "centag/p1") || !strings.Contains(s, "http://localhost:20060/v1") {
		t.Fatalf("guide content incomplete: %s", s)
	}
	if strings.Contains(s, "http://localhost:20060/v1/chat/completions") {
		t.Fatalf("written guide must not use chat/completions as primary URL: %s", s)
	}
}
