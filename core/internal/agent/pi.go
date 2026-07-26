package agent

import "fmt"

// PiTemplate Pi coding agent 配置模板（earendil-works/pi）
// 合并写入 ~/.pi/agent/models.json 的 providers.centag，并设置 settings.json 默认模型。
type PiTemplate struct{}

func (t *PiTemplate) AgentType() AgentType { return AgentPi }
func (t *PiTemplate) DisplayName() string  { return "Pi" }
func (t *PiTemplate) Description() string {
	return "AI agent toolkit / coding agent CLI (pi.dev)"
}

func (t *PiTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryCLI,
		WriteMode: WriteModeMerge,
		ConfigPaths: []string{
			"~/.pi/agent/models.json",
			"~/.pi/agent/settings.json",
		},
		KeyFields: []string{
			"providers.centag.baseUrl",
			"providers.centag.apiKey",
			"providers.centag.api",
			"providers.centag.models",
			"defaultProvider",
			"defaultModel",
		},
		ConfigMethod: "合并写入 ~/.pi/agent/models.json：在 providers 中累加/更新 centag（baseUrl/apiKey/api=openai-completions）；并合并 settings.json 的 defaultProvider/defaultModel。不覆盖其它 provider。",
		InstallURL:   "https://github.com/earendil-works/pi",
		InstallHint:  "curl -fsSL https://pi.dev/install.sh | sh；或 npm i -g --ignore-scripts @earendil-works/pi-coding-agent",
		Verified:     true,
	}
}

func (t *PiTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	apiModel := centagAPIModelID(defaultModel(info))
	modelsJSON := fmt.Sprintf(`{
  "providers": {
    "centag": {
      "baseUrl": "%s",
      "apiKey": "%s",
      "api": "openai-completions",
      "models": [
        {
          "id": "%s",
          "name": "%s",
          "reasoning": false,
          "input": ["text"],
          "contextWindow": 128000,
          "maxTokens": 16384
        }
      ]
    }
  }
}`, url, info.APIKey, apiModel, apiModel)
	settingsJSON := fmt.Sprintf(`{
  "defaultProvider": "centag",
  "defaultModel": "%s"
}`, apiModel)
	return []ConfigFile{
		{Path: "~/.pi/agent/models.json", Content: modelsJSON},
		{Path: "~/.pi/agent/settings.json", Content: settingsJSON},
	}, nil
}

func (t *PiTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	apiModel := centagAPIModelID(defaultModel(info))
	return fmt.Sprintf(`# Pi: 合并编辑 ~/.pi/agent/models.json
# 在 providers 中添加/更新 "centag": { "baseUrl": "%s", "apiKey": "<key>", "api": "openai-completions" }
# 并设置 ~/.pi/agent/settings.json: defaultProvider="centag", defaultModel="%s"
`, url, apiModel)
}

func (t *PiTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *PiTemplate) VerifyCommand(info *BackendInfo) string {
	modelRef := centagModelRef(defaultModel(info))
	return fmt.Sprintf(`pi -p --model %s "Hello, can you hear me?"`, modelRef)
}

func (t *PiTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	apiModel := centagAPIModelID(defaultModel(info))
	return []ConfigStep{
		{Title: "合并 providers.centag", Description: fmt.Sprintf("在 models.json 中累加 Centag（model=%s，baseUrl=%s）", apiModel, url)},
		{Title: "设置默认模型", Description: fmt.Sprintf("settings.json → defaultProvider=centag, defaultModel=%s", apiModel)},
		{Title: "启动 Pi", Code: "pi"},
	}
}

func (t *PiTemplate) WriteConfig(info *BackendInfo) error {
	url := proxyURL(info.Host, info.Port)
	apiModel := centagAPIModelID(defaultModel(info))
	if err := mergePiProvider("~/.pi/agent/models.json", url, info.APIKey, apiModel); err != nil {
		return err
	}
	return mergePiSettings("~/.pi/agent/settings.json", apiModel)
}
