package agent

import "fmt"

// OpenClawTemplate OpenClaw 配置模板
type OpenClawTemplate struct{}

func (t *OpenClawTemplate) AgentType() AgentType { return AgentOpenClaw }
func (t *OpenClawTemplate) DisplayName() string  { return "OpenClaw" }
func (t *OpenClawTemplate) Description() string  { return "AI 编程助手 (github.com/anomalyco/openclaw)" }

func (t *OpenClawTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	apiModel := centagAPIModelID(model)
	modelRef := centagModelRef(model)
	content := fmt.Sprintf(`{
  "agents": {
    "defaults": {
      "model": {
        "primary": "%s"
      }
    }
  },
  "models": {
    "mode": "merge",
    "providers": {
      "centag": {
        "baseUrl": "%s",
        "apiKey": "%s",
        "api": "openai-completions",
        "models": [{ "id": "%s", "name": "%s" }]
      }
    }
  }
}`, modelRef, url, info.APIKey, apiModel, apiModel)
	return []ConfigFile{
		{Path: "~/.openclaw/openclaw.json", Content: content},
	}, nil
}

func (t *OpenClawTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	return fmt.Sprintf(`# OpenClaw: 编辑 ~/.openclaw/openclaw.json
# 在 models.providers 中添加 "centag": { "baseUrl": "%s", "apiKey": "%s" }
`, url, info.APIKey)
}

func (t *OpenClawTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *OpenClawTemplate) VerifyCommand(info *BackendInfo) string {
	return `openclaw -m "Hello"`
}

func (t *OpenClawTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "配置 OpenClaw", Description: fmt.Sprintf("在 openclaw.json 中添加 Centag: %s", url)},
		{Title: "启动 OpenClaw", Code: "openclaw"},
	}
}

func (t *OpenClawTemplate) WriteConfig(info *BackendInfo) error {
	files, err := t.ConfigFiles(info)
	if err != nil {
		return err
	}
	return writeFiles(files)
}
