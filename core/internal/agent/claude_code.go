package agent

import "fmt"

// ClaudeCodeTemplate Claude Code 配置模板
type ClaudeCodeTemplate struct{}

func (t *ClaudeCodeTemplate) AgentType() AgentType    { return AgentClaudeCode }
func (t *ClaudeCodeTemplate) DisplayName() string     { return "Claude Code" }
func (t *ClaudeCodeTemplate) Description() string     { return "Anthropic 官方的 AI 编程助手 CLI 工具" }

func (t *ClaudeCodeTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)

	claudeJSON := fmt.Sprintf(`{
  "primaryApiKey": "%s",
  "env": {
    "ANTHROPIC_BASE_URL": "%s",
    "ANTHROPIC_MODEL": "%s"
  }
}`, info.APIKey, url, model)

	envContent := fmt.Sprintf(`export ANTHROPIC_BASE_URL="%s"
export ANTHROPIC_AUTH_TOKEN="%s"
export ANTHROPIC_MODEL="%s"
`, url, info.APIKey, model)

	return []ConfigFile{
		{Path: "~/.claude.json", Content: claudeJSON},
		{Path: "~/.claude/.env", Content: envContent, Append: true},
	}, nil
}

func (t *ClaudeCodeTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return fmt.Sprintf(`# Claude Code 一键配置
export ANTHROPIC_BASE_URL="%s"
export ANTHROPIC_AUTH_TOKEN="%s"
export ANTHROPIC_MODEL="%s"
`, url, info.APIKey, model)
}

func (t *ClaudeCodeTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *ClaudeCodeTemplate) VerifyCommand(info *BackendInfo) string {
	return `claude -e "Hello, can you hear me?" --print`
}

func (t *ClaudeCodeTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "配置环境变量", Description: fmt.Sprintf("设置 ANTHROPIC_BASE_URL 指向 Centag: %s", url)},
		{Title: "启动 Claude Code", Description: "在终端中运行 claude 命令", Code: "claude"},
	}
}

func (t *ClaudeCodeTemplate) WriteConfig(info *BackendInfo) error {
	files, err := t.ConfigFiles(info)
	if err != nil {
		return err
	}
	return writeFiles(files)
}
