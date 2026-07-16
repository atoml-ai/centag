package agent

import "fmt"

// OpenCodeTemplate OpenCode 配置模板
type OpenCodeTemplate struct{}

func (t *OpenCodeTemplate) AgentType() AgentType { return AgentOpenCode }
func (t *OpenCodeTemplate) DisplayName() string  { return "OpenCode" }
func (t *OpenCodeTemplate) Description() string  { return "AI 编程助手 (opencode.ai)" }

func (t *OpenCodeTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	content := fmt.Sprintf(`{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "centag": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Centag",
      "options": {
        "baseURL": "%s",
        "apiKey": "%s"
      },
      "models": {
        "%s": {
          "name": "%s",
          "limit": { "context": 128000, "output": 16384 }
        }
      }
    }
  },
  "model": "centag/%s"
}`, url, info.APIKey, model, model, model)
	return []ConfigFile{
		{Path: "~/.config/opencode/opencode.json", Content: content},
		{Path: "~/.config/opencode/opencode.jsonc", Content: content},
	}, nil
}

func (t *OpenCodeTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return fmt.Sprintf(`# OpenCode: 编辑 ~/.config/opencode/opencode.json
# 在 provider 中添加 "centag"（npm=@ai-sdk/openai-compatible）
# options.baseURL 设置为 "%s"
# 并设置 model="centag/%s"
`, url, model)
}

func (t *OpenCodeTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *OpenCodeTemplate) VerifyCommand(info *BackendInfo) string {
	return `opencode -m "Hello" --dry-run`
}

func (t *OpenCodeTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return []ConfigStep{
		{Title: "配置 provider", Description: fmt.Sprintf("在 opencode.json 中添加 Centag（model=centag/%s，baseURL=%s）", model, url)},
		{Title: "启动 OpenCode", Code: "opencode"},
	}
}

func (t *OpenCodeTemplate) WriteConfig(info *BackendInfo) error {
	files, err := t.ConfigFiles(info)
	if err != nil {
		return err
	}
	return writeFiles(files)
}
