package agent

import "fmt"

// GrokBuildTemplate Grok Build (xAI) 配置模板
// 使用 Codex 风格的 TOML 配置格式
type GrokBuildTemplate struct{}

func (t *GrokBuildTemplate) AgentType() AgentType { return AgentGrokBuild }
func (t *GrokBuildTemplate) DisplayName() string  { return "Grok Build" }
func (t *GrokBuildTemplate) Description() string  { return "xAI Grok 编程助手（使用 Codex 风格配置）" }

func (t *GrokBuildTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	if model == "gpt-4o" {
		model = "grok-4.5"
	}

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
		{Path: "~/.grok-build/auth.json", Content: authJSON},
		{Path: "~/.grok-build/config.toml", Content: configTOML},
	}, nil
}

func (t *GrokBuildTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	if model == "gpt-4o" {
		model = "grok-4.5"
	}
	return fmt.Sprintf(`# Grok Build 一键配置
mkdir -p ~/.grok-build
echo '{"OPENAI_API_KEY": "%s"}' > ~/.grok-build/auth.json
cat > ~/.grok-build/config.toml << 'EOF'
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

func (t *GrokBuildTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *GrokBuildTemplate) VerifyCommand(info *BackendInfo) string {
	return `grok-build -m "Hello, can you hear me?" --no-interactive`
}

func (t *GrokBuildTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "配置 auth.json", Code: fmt.Sprintf(`echo '{"OPENAI_API_KEY": "%s"}' > ~/.grok-build/auth.json`, info.APIKey)},
		{Title: "配置 config.toml", Description: fmt.Sprintf("设置 base_url 指向 Centag: %s", url)},
		{Title: "启动 Grok Build", Code: "grok-build"},
	}
}

func (t *GrokBuildTemplate) WriteConfig(info *BackendInfo) error {
	files, err := t.ConfigFiles(info)
	if err != nil {
		return err
	}
	return writeFiles(files)
}
