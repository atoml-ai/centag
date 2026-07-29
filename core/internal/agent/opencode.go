package agent

import "fmt"

// OpenCodeTemplate OpenCode 配置模板（对齐 cc-switch：累加 provider.centag）
type OpenCodeTemplate struct{}

func (t *OpenCodeTemplate) AgentType() AgentType { return AgentOpenCode }
func (t *OpenCodeTemplate) DisplayName() string  { return "OpenCode（CLI/Desktop）" }
func (t *OpenCodeTemplate) Description() string  { return "AI 编程助手，支持 CLI 与 Desktop 模式 (opencode.ai)" }

func (t *OpenCodeTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryCLI,
		Vendor:    VendorOpenCode,
		WriteMode: WriteModeMerge,
		ConfigPaths: []string{
			"~/.config/opencode/opencode.json",
		},
		KeyFields: []string{
			"provider.centag.npm",
			"provider.centag.options.baseURL",
			"provider.centag.options.apiKey",
			"provider.centag.models",
			"model",
		},
		ConfigMethod:  "合并写入 ~/.config/opencode/opencode.json：在 provider 中累加/更新 centag（npm=@ai-sdk/openai-compatible，options.baseURL/apiKey），并设置默认 model=centag/<apiModel>。不覆盖其它 provider。",
		InstallURL:    "https://opencode.ai",
		InstallHint:   "curl -fsSL https://opencode.ai/install | bash；或 npm i -g opencode-ai",
		AccessMethods: []AccessMethod{AccessWriteConfig, AccessWrapCLI},
		CompanionCLI:  NewCLICompanion("opencode", "https://opencode.ai", "curl -fsSL https://opencode.ai/install | bash；或 npm i -g opencode-ai"),
		VerifiedWrite: true,
		VerifiedWrap:  true,
	}
}

func (t *OpenCodeTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	apiModel := centagAPIModelID(model)
	modelRef := centagModelRef(model)
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
  "model": "%s"
}`, url, info.APIKey, apiModel, apiModel, modelRef)
	return []ConfigFile{
		{Path: "~/.config/opencode/opencode.json", Content: content},
	}, nil
}

func (t *OpenCodeTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	modelRef := centagModelRef(defaultModel(info))
	return fmt.Sprintf(`# OpenCode: 合并编辑 ~/.config/opencode/opencode.json
# 在 provider 中添加/更新 "centag"（npm=@ai-sdk/openai-compatible）
# options.baseURL 设置为 "%s"
# 并设置 model="%s"
`, url, modelRef)
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
	modelRef := centagModelRef(defaultModel(info))
	return fmt.Sprintf(`opencode run -m %s "Hello, can you hear me?"`, modelRef)
}

func (t *OpenCodeTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	modelRef := centagModelRef(defaultModel(info))
	return []ConfigStep{
		{Title: "合并 provider.centag", Description: fmt.Sprintf("在 opencode.json 中累加 Centag（model=%s，baseURL=%s）", modelRef, url)},
		{Title: "启动 OpenCode", Code: "opencode"},
	}
}

func (t *OpenCodeTemplate) WriteConfig(info *BackendInfo) error {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return mergeOpenCodeProvider(
		"~/.config/opencode/opencode.json",
		url,
		info.APIKey,
		centagAPIModelID(model),
		centagModelRef(model),
	)
}
