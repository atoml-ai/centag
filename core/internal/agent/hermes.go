package agent

import "fmt"

// HermesTemplate Hermes Agent 配置模板（对齐 cc-switch：累加 custom_providers）
type HermesTemplate struct{}

func (t *HermesTemplate) AgentType() AgentType { return AgentHermes }
func (t *HermesTemplate) DisplayName() string  { return "Hermes Agent" }
func (t *HermesTemplate) Description() string  { return "面向任务的 AI Agent 框架" }

func (t *HermesTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryCLI,
		WriteMode: WriteModeMerge,
		ConfigPaths: []string{
			"~/.hermes/config.yaml",
		},
		KeyFields: []string{
			"model.default",
			"model.provider",
			"model.base_url",
			"custom_providers[].name",
			"custom_providers[].api_key",
			"custom_providers[].api_mode",
		},
		ConfigMethod:  "合并写入 ~/.hermes/config.yaml：更新 model.default/provider/base_url，并在 custom_providers 中按 name=centag 累加/更新条目（api_mode=chat_completions）。不删除其它 custom_providers。",
		InstallURL:    "https://github.com/NousResearch/hermes-agent",
		InstallHint:   "curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash；或 pip install \"hermes-agent[web]\"",
		AccessMethods: []AccessMethod{AccessWriteConfig, AccessWrapCLI},
		CompanionCLI:  NewCLICompanion("hermes", "https://github.com/NousResearch/hermes-agent", "curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash；或 pip install \"hermes-agent[web]\""),
	}
}

func (t *HermesTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	content := fmt.Sprintf(`model:
  default: "%s"
  provider: "centag"
  base_url: "%s"

custom_providers:
  - name: centag
    base_url: "%s"
    api_key: "%s"
    api_mode: "chat_completions"
    model: "%s"
`, model, url, url, info.APIKey, model)
	return []ConfigFile{
		{Path: "~/.hermes/config.yaml", Content: content},
	}, nil
}

func (t *HermesTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return fmt.Sprintf(`# Hermes Agent：合并编辑 ~/.hermes/config.yaml
# model.default="%s" / model.provider="centag" / model.base_url="%s"
# custom_providers 中更新 name=centag 条目（api_key / api_mode=chat_completions）
`, model, url)
}

func (t *HermesTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *HermesTemplate) VerifyCommand(info *BackendInfo) string {
	return `hermes "Hello, can you hear me?"`
}

func (t *HermesTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "合并 config.yaml", Description: fmt.Sprintf("在 ~/.hermes/config.yaml 中累加 Centag: %s", url)},
		{Title: "启动 Hermes", Code: "hermes"},
	}
}

func (t *HermesTemplate) WriteConfig(info *BackendInfo) error {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return mergeHermesProvider("~/.hermes/config.yaml", url, info.APIKey, model)
}
