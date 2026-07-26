package agent

import "fmt"

// GeminiTemplate Gemini CLI 配置模板（对齐 cc-switch：.env + settings.json）
type GeminiTemplate struct{}

func (t *GeminiTemplate) AgentType() AgentType { return AgentGeminiCLI }
func (t *GeminiTemplate) DisplayName() string  { return "Gemini CLI" }
func (t *GeminiTemplate) Description() string  { return "Google 官方的 AI 编程助手 CLI 工具" }

func (t *GeminiTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryCLI,
		WriteMode: WriteModeOverwrite,
		ConfigPaths: []string{
			"~/.gemini/.env",
			"~/.gemini/settings.json",
		},
		KeyFields: []string{
			"GEMINI_API_KEY",
			"GOOGLE_GEMINI_BASE_URL",
			"GEMINI_MODEL",
			"security.auth.selectedType",
		},
		ConfigMethod:  "写入 ~/.gemini/.env（GEMINI_API_KEY / GOOGLE_GEMINI_BASE_URL / GEMINI_MODEL），并合并 settings.json 的 security.auth.selectedType=gemini-api-key。",
		InstallURL:    "https://github.com/google-gemini/gemini-cli",
		InstallHint:   "npm i -g @google/gemini-cli；或 brew install gemini-cli",
		AccessMethods: []AccessMethod{AccessWriteConfig, AccessWrapCLI},
		CompanionCLI:  NewCLICompanion("gemini", "https://github.com/google-gemini/gemini-cli", "npm i -g @google/gemini-cli；或 brew install gemini-cli"),
	}
}

func (t *GeminiTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	envContent := fmt.Sprintf(`GEMINI_API_KEY=%s
GOOGLE_GEMINI_BASE_URL=%s
GEMINI_MODEL=%s
`, info.APIKey, url, model)
	settings := `{
  "security": {
    "auth": {
      "selectedType": "gemini-api-key"
    }
  }
}`
	return []ConfigFile{
		{Path: "~/.gemini/.env", Content: envContent},
		{Path: "~/.gemini/settings.json", Content: settings},
	}, nil
}

func (t *GeminiTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return fmt.Sprintf(`# Gemini CLI 一键配置
mkdir -p ~/.gemini
cat > ~/.gemini/.env << 'EOF'
GEMINI_API_KEY=%s
GOOGLE_GEMINI_BASE_URL=%s
GEMINI_MODEL=%s
EOF
# 同时设置 settings.json → security.auth.selectedType=gemini-api-key
`, info.APIKey, url, model)
}

func (t *GeminiTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *GeminiTemplate) VerifyCommand(info *BackendInfo) string {
	return `gemini "Hello, can you hear me?"`
}

func (t *GeminiTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "配置 .env", Description: fmt.Sprintf("写入 ~/.gemini/.env: %s", url)},
		{Title: "配置 auth 类型", Description: "settings.json → security.auth.selectedType=gemini-api-key"},
		{Title: "启动 Gemini CLI", Code: "gemini"},
	}
}

func (t *GeminiTemplate) WriteConfig(info *BackendInfo) error {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	envContent := fmt.Sprintf(`GEMINI_API_KEY=%s
GOOGLE_GEMINI_BASE_URL=%s
GEMINI_MODEL=%s
`, info.APIKey, url, model)
	if err := writeFiles([]ConfigFile{{Path: "~/.gemini/.env", Content: envContent}}); err != nil {
		return err
	}
	return mergeGeminiAuthSettings("~/.gemini/settings.json")
}
