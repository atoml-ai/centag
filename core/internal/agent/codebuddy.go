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

// CodeBuddyTemplate 腾讯云 CodeBuddy 桌面端（~/.codebuddy/models.json）
// 文档：https://www.codebuddy.ai/docs/zh/cli/models
type CodeBuddyTemplate struct{}

func (t *CodeBuddyTemplate) AgentType() AgentType { return AgentCodeBuddy }
func (t *CodeBuddyTemplate) DisplayName() string  { return "CodeBuddy" }
func (t *CodeBuddyTemplate) Description() string {
	return "腾讯云代码助手 CodeBuddy 桌面端；另有 CodeBuddy Code CLI（codebuddy），共用 models.json"
}

func (t *CodeBuddyTemplate) Meta() AgentSetupMeta {
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
			"availableModels",
		},
		ConfigMethod:  "合并写入 ~/.codebuddy/models.json（SmartMerge：同 id 覆盖、异 id 追加）。url 必须为完整路径 …/v1/chat/completions；仅支持 OpenAI 接口格式。桌面端与 CodeBuddy Code CLI、WorkBuddy 可共用该文件。",
		InstallURL:    "https://www.codebuddy.ai/docs/zh/cli/installation",
		InstallHint:   "桌面端：官网下载。CLI（wrap 用）：npm i -g @tencent-ai/codebuddy-code 或 brew install codebuddy-code",
		AccessMethods: []AccessMethod{AccessWriteConfig, AccessWrapCLI},
		CompanionCLI:  NewDesktopCompanionCLI("codebuddy", "https://www.codebuddy.ai/docs/zh/cli/installation", "npm i -g @tencent-ai/codebuddy-code 或 brew install codebuddy-code"),
		VerifiedWrite: true,
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
	return `# 在 CodeBuddy 对话界面选择模型「Centag」并发起一次简单对话`
}

func (t *CodeBuddyTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := chatCompletionsURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "合并 models.json", Description: fmt.Sprintf("在 ~/.codebuddy/models.json 累加 Centag，url=%s", url)},
		{Title: "在 CodeBuddy 中选用", Description: "设置 → 模型 → 选择 Centag 自定义模型"},
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

// WorkBuddyTemplate 腾讯云 WorkBuddy 桌面助理
// 接入方式：在客户端「设置 → 模型」填写自定义 API（与 TRAE 相同的 UI 参数向导）。
// 文档：https://www.codebuddy.ai/docs/zh/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model
type WorkBuddyTemplate struct{}

func (t *WorkBuddyTemplate) AgentType() AgentType { return AgentWorkBuddy }
func (t *WorkBuddyTemplate) DisplayName() string  { return "WorkBuddy" }
func (t *WorkBuddyTemplate) Description() string {
	return "腾讯云 WorkBuddy 桌面助理（在设置中填写自定义模型参数）"
}

func (t *WorkBuddyTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryDesktop,
		WriteMode: WriteModeNone,
		KeyFields: []string{
			"API 格式=OpenAI",
			"请求地址=…/v1",
			"模型 ID=centag/<pipeline>",
		},
		ConfigMethod:  "在 WorkBuddy「设置 → 模型」添加自定义 API：请求地址填 …/v1（推荐）；也可填 …/v1/chat/completions。模型 ID 填 centag/<pipeline>。",
		InstallURL:    "https://www.codebuddy.ai/docs/zh/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model",
		InstallHint:   "从 CodeBuddy / WorkBuddy 官网下载桌面端；模型配置见文档「模型配置」",
		AccessMethods: []AccessMethod{AccessUIGuide},
		UIGuide: &UIGuide{
			Title:          "在 WorkBuddy 中填写自定义模型参数",
			Summary:        "打开 设置 → 模型 → 自定义 API。请求地址填 …/v1（推荐）；模型 ID 随流水线变化。",
			DocURL:         "https://www.codebuddy.ai/docs/zh/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model",
			Steps:          []string{"设置 → 模型 → 自定义 API / 添加模型"},
			RequestURLKind: RequestURLOpenAIBase,
			URLHint:        "也可填 …/v1/chat/completions（客户端一般会自动校验/补全）",
			RestartHint:    "添加成功后若列表未刷新，重启 WorkBuddy 再选用",
		},
		VerifiedUI: true,
	}
}

func (t *WorkBuddyTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	id := codeBuddyModelID(info)
	url := proxyURL(info.Host, info.Port)
	content := fmt.Sprintf(`# Centag × WorkBuddy 接入参数

在 WorkBuddy：设置 → 模型 → 自定义 API，填写：

| 项 | 值 |
|---|---|
| 请求地址（推荐 Base） | %s |
| 模型 ID | %s |
| API Key | <Centag API Key> |

> 也可填 %s/chat/completions；客户端通常会校验或补全路径。
> 也可由 CodeBuddy 写入共用的 ~/.codebuddy/models.json 后在 WorkBuddy 中选用。
`, url, id, url)
	return []ConfigFile{
		{Path: "~/CENTAG_WORKBUDDY_SETUP.md", Content: content},
	}, nil
}

func (t *WorkBuddyTemplate) SetupCommand(info *BackendInfo) string {
	id := codeBuddyModelID(info)
	url := proxyURL(info.Host, info.Port)
	return fmt.Sprintf(`# WorkBuddy：在 UI 填写自定义模型
# 设置 → 模型 → 自定义 API
# 请求地址: %s  （也可 %s/chat/completions）
# 模型 ID: %s
# API Key: <Centag API Key>
`, url, url, id)
}

func (t *WorkBuddyTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	script := t.SetupCommand(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + script,
		Linux:   "#!/bin/bash\n" + script,
		Windows: "# PowerShell\n" + script,
	}
}

func (t *WorkBuddyTemplate) VerifyCommand(info *BackendInfo) string {
	return `# 在 WorkBuddy 对话界面选择刚添加的 Centag 模型并发起一次简单对话`
}

func (t *WorkBuddyTemplate) Steps(info *BackendInfo) []ConfigStep {
	id := codeBuddyModelID(info)
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "UI 填写自定义模型", Description: fmt.Sprintf("请求地址=%s（也可 …/chat/completions）；模型 ID=%s", url, id)},
		{Title: "选用模型", Description: "在对话中选择 Centag 自定义模型"},
	}
}

func (t *WorkBuddyTemplate) WriteConfig(info *BackendInfo) error {
	// UI 指引路径：不改写本地代理配置（与 TRAE 一致，向导仅展示可复制参数）
	return nil
}
