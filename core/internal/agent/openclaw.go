package agent

import "fmt"

// OpenClawTemplate OpenClaw 配置模板（对齐 cc-switch：累加 models.providers）
type OpenClawTemplate struct{}

func (t *OpenClawTemplate) AgentType() AgentType { return AgentOpenClaw }
func (t *OpenClawTemplate) DisplayName() string  { return "OpenClaw" }
func (t *OpenClawTemplate) Description() string {
	return "AI 编程助手 (github.com/anomalyco/openclaw)"
}

func (t *OpenClawTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryCLI,
		WriteMode: WriteModeMerge,
		ConfigPaths: []string{
			"~/.openclaw/openclaw.json",
		},
		KeyFields: []string{
			"agents.defaults.model.primary",
			"models.providers.centag.baseUrl",
			"models.providers.centag.apiKey",
			"models.providers.centag.api",
		},
		ConfigMethod:  "合并写入 ~/.openclaw/openclaw.json：在 models.providers 中累加/更新 centag（baseUrl/apiKey/api=openai-completions），并设置 agents.defaults.model.primary。不覆盖其它 provider。",
		InstallURL:    "https://www.npmjs.com/package/openclaw",
		InstallHint:   "npm i -g openclaw",
		AccessMethods: []AccessMethod{AccessWriteConfig, AccessWrapCLI},
		// LLM 实际由 LaunchAgent 网关发出；wrap 默认 argv 用同进程 tui --local，避免只包一层 TUI 却劫持不到流量。
		CompanionCLI: &CompanionCLI{
			Binary:      "openclaw",
			Argv:        []string{"openclaw", "tui", "--local"},
			InstallURL:  "https://www.npmjs.com/package/openclaw",
			InstallHint: "npm i -g openclaw",
			Note:        "OpenClaw 的 LLM 请求由 LaunchAgent 网关发出，仅 wrap `openclaw` 不会劫持。请先执行 openclaw gateway stop，再复制下方命令（同进程 tui --local）。测 wrap 时请把 baseUrl 指到非 Centag 的厂商地址。",
		},
		VerifiedWrite: true,
		VerifiedWrap:  true,
	}
}

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
	return fmt.Sprintf(`# OpenClaw: 合并编辑 ~/.openclaw/openclaw.json
# 在 models.providers 中添加/更新 "centag": { "baseUrl": "%s", "apiKey": "<key>", "api": "openai-completions" }
`, url)
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
		{Title: "合并 OpenClaw provider", Description: fmt.Sprintf("在 openclaw.json 中累加 Centag: %s", url)},
		{Title: "启动 OpenClaw", Code: "openclaw"},
	}
}

func (t *OpenClawTemplate) WriteConfig(info *BackendInfo) error {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return mergeOpenClawProvider(
		"~/.openclaw/openclaw.json",
		url,
		info.APIKey,
		centagAPIModelID(model),
		centagModelRef(model),
	)
}
