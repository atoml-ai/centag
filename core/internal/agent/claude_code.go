package agent

import "fmt"

// ClaudeCodeTemplate Claude Code 配置模板（对齐 cc-switch：~/.claude/settings.json）
type ClaudeCodeTemplate struct{}

func (t *ClaudeCodeTemplate) AgentType() AgentType { return AgentClaudeCode }
func (t *ClaudeCodeTemplate) DisplayName() string  { return "Claude Code" }
func (t *ClaudeCodeTemplate) Description() string {
	return "Anthropic 官方的 AI 编程助手 CLI 工具"
}

func (t *ClaudeCodeTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryCLI,
		WriteMode: WriteModeOverwrite,
		ConfigPaths: []string{
			"~/.claude/settings.json",
		},
		KeyFields: []string{
			"env.ANTHROPIC_BASE_URL",
			"env.ANTHROPIC_AUTH_TOKEN",
			"env.ANTHROPIC_MODEL",
		},
		ConfigMethod:  "写入 ~/.claude/settings.json：合并 env 中的 ANTHROPIC_BASE_URL / ANTHROPIC_AUTH_TOKEN / ANTHROPIC_MODEL 指向 Centag（对齐 cc-switch）。首次覆盖会备份为 .centag-bak。",
		InstallURL:    "https://code.claude.com/docs/en/install",
		InstallHint:   "macOS/Linux: curl -fsSL https://claude.ai/install.sh | bash；或 brew install --cask claude-code",
		AccessMethods: []AccessMethod{AccessWriteConfig, AccessWrapCLI},
		VerifiedWrap:  true, // wrap 方式已验证
		CompanionCLI:  NewCLICompanion("claude", "https://code.claude.com/docs/en/install", "macOS/Linux: curl -fsSL https://claude.ai/install.sh | bash；或 brew install --cask claude-code"),
	}
}

// claudeBaseURL 返回 Claude Code 专用 base URL（不带 /v1）。
// Claude Code 自己拼 /v1/messages，所以 ANTHROPIC_BASE_URL 不能带 /v1。
func claudeBaseURL(info *BackendInfo) string {
	host := info.Host
	if host == "" {
		host = "localhost"
	}
	port := info.Port
	if port == 0 {
		port = 20060
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

func (t *ClaudeCodeTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := claudeBaseURL(info)
	model := defaultModel(info)
	content := fmt.Sprintf(`{
  "env": {
    "ANTHROPIC_BASE_URL": "%s",
    "ANTHROPIC_AUTH_TOKEN": "%s"
  }
}`, url, info.APIKey)
	// transparent-proxy / pipeline 模式：不写 ANTHROPIC_MODEL，让 Claude Code
	// 使用默认模型（pipeline 会根据路由规则转发，不需要客户端指定 centag 虚拟模型）。
	if model != "" && !isVirtualModel(model) {
		content = fmt.Sprintf(`{
  "env": {
    "ANTHROPIC_BASE_URL": "%s",
    "ANTHROPIC_AUTH_TOKEN": "%s",
    "ANTHROPIC_MODEL": "%s"
  }
}`, url, info.APIKey, model)
	}
	return []ConfigFile{
		{Path: "~/.claude/settings.json", Content: content},
	}, nil
}

func (t *ClaudeCodeTemplate) SetupCommand(info *BackendInfo) string {
	url := claudeBaseURL(info)
	model := defaultModel(info)
	return fmt.Sprintf(`# Claude Code（settings.json env）
# 编辑 ~/.claude/settings.json，合并：
# env.ANTHROPIC_BASE_URL="%s"
# env.ANTHROPIC_AUTH_TOKEN="%s"
# env.ANTHROPIC_MODEL="%s"
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
	url := claudeBaseURL(info)
	return []ConfigStep{
		{Title: "合并 settings.json", Description: fmt.Sprintf("设置 env.ANTHROPIC_BASE_URL 指向 Centag: %s", url)},
		{Title: "启动 Claude Code", Description: "在终端中运行 claude 命令", Code: "claude"},
	}
}

func (t *ClaudeCodeTemplate) WriteConfig(info *BackendInfo) error {
	url := claudeBaseURL(info)
	model := defaultModel(info)
	env := map[string]string{
		"ANTHROPIC_BASE_URL":  url,
		"ANTHROPIC_AUTH_TOKEN": info.APIKey,
	}
	// transparent-proxy / pipeline 模式：不写 ANTHROPIC_MODEL，让 Claude Code
	// 使用默认模型（pipeline 会根据路由规则转发，不需要客户端指定 centag 虚拟模型）。
	if model != "" && !isVirtualModel(model) {
		env["ANTHROPIC_MODEL"] = model
	}
	return mergeClaudeSettingsEnv("~/.claude/settings.json", env)
}
