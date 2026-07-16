package agent

import "fmt"

// GeminiTemplate Gemini CLI 配置模板
type GeminiTemplate struct{}

func (t *GeminiTemplate) AgentType() AgentType { return AgentGeminiCLI }
func (t *GeminiTemplate) DisplayName() string  { return "Gemini CLI" }
func (t *GeminiTemplate) Description() string  { return "Google 官方的 AI 编程助手 CLI 工具" }

func (t *GeminiTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	content := fmt.Sprintf(`GEMINI_API_KEY=%s
GOOGLE_GEMINI_BASE_URL=%s
GEMINI_MODEL=%s
`, info.APIKey, url, model)
	return []ConfigFile{
		{Path: "~/.gemini/.env", Content: content},
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
		{Title: "配置 .env 文件", Description: fmt.Sprintf("写入 ~/.gemini/.env: %s", url)},
		{Title: "启动 Gemini CLI", Code: "gemini"},
	}
}

func (t *GeminiTemplate) WriteConfig(info *BackendInfo) error {
	files, err := t.ConfigFiles(info)
	if err != nil {
		return err
	}
	return writeFiles(files)
}
