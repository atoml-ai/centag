package agent

import "fmt"

const codeBuddyModelsPath = "~/.codebuddy/models.json"

// codeBuddyModelID 使用 centag 模型 id（与网关 /v1/models 一致）。
func codeBuddyModelID(info *BackendInfo) string {
	return centagAPIModelID(defaultModel(info))
}

func codeBuddyModelEntryJSON(info *BackendInfo) string {
	id := codeBuddyModelID(info)
	url := chatCompletionsURL(info.Host, info.Port)
	return fmt.Sprintf(`{
  "models": [
    {
      "id": "%s",
      "name": "Centag",
      "vendor": "Centag",
      "apiKey": "%s",
      "url": "%s",
      "maxInputTokens": 128000,
      "maxOutputTokens": 16384,
      "supportsToolCall": true,
      "supportsImages": false
    }
  ],
  "availableModels": ["%s"]
}`, id, info.APIKey, url, id)
}

// CodeBuddyTemplate 腾讯云 CodeBuddy Code CLI（~/.codebuddy/models.json）
// 文档：https://www.codebuddy.ai/docs/zh/cli/models
type CodeBuddyTemplate struct{}

func (t *CodeBuddyTemplate) AgentType() AgentType { return AgentCodeBuddy }
func (t *CodeBuddyTemplate) DisplayName() string  { return "CodeBuddy" }
func (t *CodeBuddyTemplate) Description() string {
	return "腾讯云代码助手 CodeBuddy Code CLI（OpenAI 兼容自定义模型）"
}

func (t *CodeBuddyTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryCLI,
		WriteMode: WriteModeMerge,
		ConfigPaths: []string{
			codeBuddyModelsPath,
		},
		KeyFields: []string{
			"models[].id",
			"models[].apiKey",
			"models[].url",
			"availableModels",
		},
		ConfigMethod: "合并写入 ~/.codebuddy/models.json（SmartMerge：同 id 覆盖、异 id 追加）。url 必须为完整路径 …/v1/chat/completions；仅支持 OpenAI 接口格式。WorkBuddy 可读取同一文件。详见官方 models.json 指南。",
		InstallURL:   "https://www.codebuddy.ai/docs/zh/cli/installation",
		InstallHint:  "npm i -g @tencent-ai/codebuddy-code；或 curl -fsSL https://www.codebuddy.cn/cli/install.sh | bash",
	}
}

func (t *CodeBuddyTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	return []ConfigFile{
		{Path: codeBuddyModelsPath, Content: codeBuddyModelEntryJSON(info)},
	}, nil
}

func (t *CodeBuddyTemplate) SetupCommand(info *BackendInfo) string {
	id := codeBuddyModelID(info)
	url := chatCompletionsURL(info.Host, info.Port)
	return fmt.Sprintf(`# CodeBuddy：合并 ~/.codebuddy/models.json
# 添加模型 id="%s"
# url="%s"（须含 /chat/completions）
# apiKey="<Centag API Key>"
`, id, url)
}

func (t *CodeBuddyTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *CodeBuddyTemplate) VerifyCommand(info *BackendInfo) string {
	return fmt.Sprintf(`codebuddy --model %s -p "Hello, can you hear me?"`, codeBuddyModelID(info))
}

func (t *CodeBuddyTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := chatCompletionsURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "合并 models.json", Description: fmt.Sprintf("在 ~/.codebuddy/models.json 累加 Centag，url=%s", url)},
		{Title: "启动 CodeBuddy", Code: "codebuddy"},
	}
}

func (t *CodeBuddyTemplate) WriteConfig(info *BackendInfo) error {
	return mergeCodeBuddyModel(
		codeBuddyModelsPath,
		codeBuddyModelID(info),
		"Centag",
		info.APIKey,
		chatCompletionsURL(info.Host, info.Port),
	)
}

// WorkBuddyTemplate 腾讯云 WorkBuddy（与 CodeBuddy 共用 ~/.codebuddy/models.json）
// 文档：https://www.codebuddy.ai/docs/zh/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model
type WorkBuddyTemplate struct{}

func (t *WorkBuddyTemplate) AgentType() AgentType { return AgentWorkBuddy }
func (t *WorkBuddyTemplate) DisplayName() string  { return "WorkBuddy" }
func (t *WorkBuddyTemplate) Description() string {
	return "腾讯云 WorkBuddy 桌面助理（自定义模型可读 ~/.codebuddy/models.json）"
}

func (t *WorkBuddyTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryDesktop,
		WriteMode: WriteModeMerge,
		ConfigPaths: []string{
			codeBuddyModelsPath,
		},
		KeyFields: []string{
			"models[].id",
			"models[].apiKey",
			"models[].url",
		},
		ConfigMethod: "与 CodeBuddy 共用 ~/.codebuddy/models.json：合并写入 Centag 模型后，可在 WorkBuddy「设置 → 模型」中查看/选用该自定义模型（官方说明：models.json 配置可在 UI 中继续管理）。也可仅在 UI 中选「自定义 API」填写同等字段。",
		InstallURL:   "https://www.codebuddy.ai/docs/zh/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model",
		InstallHint:  "从 CodeBuddy / WorkBuddy 官网下载桌面端；模型配置见文档「模型配置」",
	}
}

func (t *WorkBuddyTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	return (&CodeBuddyTemplate{}).ConfigFiles(info)
}

func (t *WorkBuddyTemplate) SetupCommand(info *BackendInfo) string {
	return (&CodeBuddyTemplate{}).SetupCommand(info) + "# WorkBuddy：写入后打开「设置 → 模型」确认自定义模型已出现\n"
}

func (t *WorkBuddyTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	return (&CodeBuddyTemplate{}).PlatformCommands(info)
}

func (t *WorkBuddyTemplate) VerifyCommand(info *BackendInfo) string {
	return `# 在 WorkBuddy 对话界面选择模型「Centag」并发起一次简单对话`
}

func (t *WorkBuddyTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := chatCompletionsURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "写入 models.json", Description: fmt.Sprintf("合并 Centag 到 ~/.codebuddy/models.json（url=%s）", url)},
		{Title: "在 WorkBuddy 中选用", Description: "设置 → 模型 → 选择 Centag 自定义模型"},
	}
}

func (t *WorkBuddyTemplate) WriteConfig(info *BackendInfo) error {
	return (&CodeBuddyTemplate{}).WriteConfig(info)
}
