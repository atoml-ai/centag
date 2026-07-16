package agent

import "fmt"

// ClaudeDesktopTemplate Claude Desktop 配置模板
type ClaudeDesktopTemplate struct{}

func (t *ClaudeDesktopTemplate) AgentType() AgentType { return AgentClaudeDesktop }
func (t *ClaudeDesktopTemplate) DisplayName() string  { return "Claude Desktop" }
func (t *ClaudeDesktopTemplate) Description() string  { return "Anthropic 官方的桌面客户端" }

func (t *ClaudeDesktopTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	content := fmt.Sprintf(`{
  "env": {
    "ANTHROPIC_BASE_URL": "%s",
    "ANTHROPIC_AUTH_TOKEN": "%s"
  }
}`, url, info.APIKey)
	return []ConfigFile{
		{Path: "~/.config/Claude/claude_desktop_config.json", Content: content},
	}, nil
}

func (t *ClaudeDesktopTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	return fmt.Sprintf(`# Claude Desktop 配置
# 编辑 ~/.config/Claude/claude_desktop_config.json
# 设置 env.ANTHROPIC_BASE_URL="%s"
# 设置 env.ANTHROPIC_AUTH_TOKEN="%s"
`, url, info.APIKey)
}

func (t *ClaudeDesktopTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *ClaudeDesktopTemplate) VerifyCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	return fmt.Sprintf(`curl -s %s/models -H "x-api-key: %s"`, url, info.APIKey)
}

func (t *ClaudeDesktopTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "打开 Claude Desktop 设置", Description: fmt.Sprintf("填入 Gateway: %s, API Key: %s", url, info.APIKey)},
	}
}

func (t *ClaudeDesktopTemplate) WriteConfig(info *BackendInfo) error {
	files, err := t.ConfigFiles(info)
	if err != nil {
		return err
	}
	return writeFiles(files)
}
