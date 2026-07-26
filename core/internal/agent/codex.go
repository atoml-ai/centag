package agent

import "fmt"

// CodexTemplate Codex CLI 配置模板（对齐 cc-switch：auth.json + config.toml）
type CodexTemplate struct{}

func (t *CodexTemplate) AgentType() AgentType { return AgentCodex }
func (t *CodexTemplate) DisplayName() string  { return "Codex CLI" }
func (t *CodexTemplate) Description() string {
	return "OpenAI 官方的 AI 编程助手 CLI (ChatGPT Codex)"
}

func (t *CodexTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryCLI,
		WriteMode: WriteModeOverwrite,
		ConfigPaths: []string{
			"~/.codex/auth.json",
			"~/.codex/config.toml",
		},
		KeyFields: []string{
			"OPENAI_API_KEY",
			"model_provider",
			"model",
			"model_providers.custom.base_url",
			"wire_api",
		},
		ConfigMethod:  "覆盖写入 ~/.codex/auth.json（OPENAI_API_KEY）与 ~/.codex/config.toml（model_provider=custom，[model_providers.custom].base_url 指向 Centag，wire_api=responses）。",
		InstallURL:    "https://github.com/openai/codex",
		InstallHint:   "curl -fsSL https://chatgpt.com/codex/install.sh | sh；或 npm i -g @openai/codex",
		AccessMethods: []AccessMethod{AccessWriteConfig, AccessWrapCLI},
		CompanionCLI:  NewCLICompanion("codex", "https://github.com/openai/codex", "curl -fsSL https://chatgpt.com/codex/install.sh | sh；或 npm i -g @openai/codex"),
		VerifiedWrite: true, // wrap/系统代理方式尚未维护者验证
	}
}

func (t *CodexTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)

	authJSON := fmt.Sprintf(`{"OPENAI_API_KEY": "%s"}`, info.APIKey)
	configTOML := fmt.Sprintf(`model_provider = "custom"
model = "%s"
model_reasoning_effort = "high"
disable_response_storage = true

[model_providers.custom]
name = "centag"
base_url = "%s"
wire_api = "responses"
requires_openai_auth = true
`, model, url)

	return []ConfigFile{
		{Path: "~/.codex/auth.json", Content: authJSON},
		{Path: "~/.codex/config.toml", Content: configTOML},
	}, nil
}

func (t *CodexTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return fmt.Sprintf(`# Codex CLI 一键配置
mkdir -p ~/.codex
echo '{"OPENAI_API_KEY": "%s"}' > ~/.codex/auth.json
cat > ~/.codex/config.toml << 'EOF'
model_provider = "custom"
model = "%s"
[model_providers.custom]
name = "centag"
base_url = "%s"
wire_api = "responses"
requires_openai_auth = true
EOF
`, info.APIKey, model, url)
}

func (t *CodexTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *CodexTemplate) VerifyCommand(info *BackendInfo) string {
	return `codex -m "Hello, can you hear me?" --no-interactive`
}

func (t *CodexTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "配置 auth.json", Code: fmt.Sprintf(`echo '{"OPENAI_API_KEY": "%s"}' > ~/.codex/auth.json`, info.APIKey)},
		{Title: "配置 config.toml", Description: fmt.Sprintf("设置 base_url 指向 Centag: %s", url)},
		{Title: "启动 Codex", Code: "codex"},
	}
}

func (t *CodexTemplate) WriteConfig(info *BackendInfo) error {
	files, err := t.ConfigFiles(info)
	if err != nil {
		return err
	}
	return writeFiles(files)
}
