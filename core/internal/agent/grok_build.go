package agent

import "fmt"

// GrokBuildTemplate Grok Build (xAI) 配置模板（对齐 cc-switch：~/.grok/config.toml）
type GrokBuildTemplate struct{}

func (t *GrokBuildTemplate) AgentType() AgentType { return AgentGrokBuild }
func (t *GrokBuildTemplate) DisplayName() string  { return "Grok Build" }
func (t *GrokBuildTemplate) Description() string  { return "xAI Grok 编程助手 CLI" }

func (t *GrokBuildTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryCLI,
		WriteMode: WriteModeOverwrite,
		ConfigPaths: []string{
			"~/.grok/config.toml",
		},
		KeyFields: []string{
			"models.default",
			`model."centag".model`,
			`model."centag".base_url`,
			`model."centag".api_key`,
			`model."centag".api_backend`,
		},
		ConfigMethod:  "覆盖写入 ~/.grok/config.toml：[models] default=\"centag\"，[model.\"centag\"] 内设置 model / base_url / api_key / api_backend=responses / context_window（对齐 cc-switch Grok Build）。",
		InstallURL:    "https://x.ai/cli",
		InstallHint:   "curl -fsSL https://x.ai/cli/install.sh | bash；或 npm i -g @xai-official/grok",
		AccessMethods: []AccessMethod{AccessWriteConfig, AccessWrapCLI},
		CompanionCLI:  NewCLICompanion("grok", "https://x.ai/cli", "curl -fsSL https://x.ai/cli/install.sh | bash；或 npm i -g @xai-official/grok"),
	}
}

func (t *GrokBuildTemplate) grokModel(info *BackendInfo) string {
	model := defaultModel(info)
	if model == "gpt-4o" {
		return "grok-4.5"
	}
	return model
}

func (t *GrokBuildTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := t.grokModel(info)
	configTOML := fmt.Sprintf(`[models]
default = "centag"

[model."centag"]
model = "%s"
base_url = "%s"
name = "Centag"
api_key = "%s"
api_backend = "responses"
context_window = 500000
`, model, url, info.APIKey)

	return []ConfigFile{
		{Path: "~/.grok/config.toml", Content: configTOML},
	}, nil
}

func (t *GrokBuildTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	model := t.grokModel(info)
	return fmt.Sprintf(`# Grok Build 一键配置（~/.grok/config.toml）
mkdir -p ~/.grok
cat > ~/.grok/config.toml << 'EOF'
[models]
default = "centag"

[model."centag"]
model = "%s"
base_url = "%s"
name = "Centag"
api_key = "%s"
api_backend = "responses"
context_window = 500000
EOF
`, model, url, info.APIKey)
}

func (t *GrokBuildTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *GrokBuildTemplate) VerifyCommand(info *BackendInfo) string {
	return `grok -m "Hello, can you hear me?"`
}

func (t *GrokBuildTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "配置 ~/.grok/config.toml", Description: fmt.Sprintf("设置 [model.\"centag\"].base_url=%s", url)},
		{Title: "启动 Grok", Code: "grok"},
	}
}

func (t *GrokBuildTemplate) WriteConfig(info *BackendInfo) error {
	files, err := t.ConfigFiles(info)
	if err != nil {
		return err
	}
	return writeFiles(files)
}
