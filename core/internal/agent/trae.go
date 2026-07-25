package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// TraeTemplate TRAE IDE 自定义模型接入
// 官方优先 UI：设置 → 模型 → 自定义配置（OpenAI + Base URL）
// 文档：https://docs.trae.ai/ide/models?_lang=zh / https://docs.trae.cn/ide/models
// 文件写入：尽力合并 User/settings.json 的 trae.customModels（社区路径）。
type TraeTemplate struct{}

func (t *TraeTemplate) AgentType() AgentType { return AgentTrae }
func (t *TraeTemplate) DisplayName() string  { return "TRAE" }
func (t *TraeTemplate) Description() string {
	return "字节跳动 TRAE IDE（自定义 OpenAI 兼容模型）"
}

func (t *TraeTemplate) Meta() AgentSetupMeta {
	paths := traeSettingsLogicalPaths()
	return AgentSetupMeta{
		Category:    AgentCategoryDesktop,
		WriteMode:   WriteModeMerge,
		ConfigPaths: paths,
		KeyFields: []string{
			"trae.customModels[].modelId",
			"trae.customModels[].baseUrl",
			"trae.customModels[].apiKey",
			"trae.customModels[].provider",
		},
		ConfigMethod: "推荐：设置 → 模型 → 添加模型 → 自定义配置；API 格式选 OpenAI；关闭「完整 URL」时 Base URL 填 http://host:port/v1（勿带 /chat/completions，否则会重复拼接）；模型 ID 填 centag/<pipeline>；API Key 填 Centag 密钥。一键写入会尽力合并到 Trae/Trae CN 的 User/settings.json → trae.customModels（需重启 IDE）。",
		InstallURL:   "https://docs.trae.ai/ide/models?_lang=zh",
		InstallHint:  "从 trae.ai / trae.cn 下载 IDE；模型配置见「内置模型 & 自定义模型」",
	}
}

func traeSettingsLogicalPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/Library/Application Support/Trae/User/settings.json",
			"~/Library/Application Support/Trae CN/User/settings.json",
		}
	case "windows":
		return []string{
			"%APPDATA%/Trae/User/settings.json",
			"%APPDATA%/Trae CN/User/settings.json",
		}
	default:
		return []string{
			"~/.config/Trae/User/settings.json",
			"~/.config/Trae CN/User/settings.json",
		}
	}
}

func expandAppDataPath(p string) string {
	if strings.HasPrefix(p, "%APPDATA%") {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, _ := os.UserHomeDir()
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		rest := strings.TrimPrefix(p, "%APPDATA%")
		rest = strings.TrimPrefix(rest, `\`)
		rest = strings.TrimPrefix(rest, `/`)
		parts := strings.FieldsFunc(rest, func(r rune) bool { return r == '\\' || r == '/' })
		return filepath.Join(append([]string{appData}, parts...)...)
	}
	return expandPath(p)
}

func traeModelID(info *BackendInfo) string {
	return centagAPIModelID(defaultModel(info))
}

func (t *TraeTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	id := traeModelID(info)
	base := proxyURL(info.Host, info.Port)
	snippet := fmt.Sprintf(`{
  "trae.customModels": [
    {
      "id": "%s",
      "name": "Centag",
      "displayName": "Centag",
      "modelId": "%s",
      "provider": "openai",
      "apiProtocol": "openai",
      "baseUrl": "%s",
      "url": "%s",
      "apiKey": "%s",
      "useFullUrl": false
    }
  ]
}`, id, id, base, base, info.APIKey)

	paths := traeSettingsLogicalPaths()
	files := make([]ConfigFile, 0, len(paths))
	for _, p := range paths {
		files = append(files, ConfigFile{Path: p, Content: snippet})
	}
	return files, nil
}

func (t *TraeTemplate) SetupCommand(info *BackendInfo) string {
	id := traeModelID(info)
	base := proxyURL(info.Host, info.Port)
	return fmt.Sprintf(`# TRAE 自定义模型（推荐 UI）
# 设置 → 模型 → 添加模型 → 自定义配置
# API 格式: OpenAI
# 完整 URL: 关闭
# 请求地址(Base): %s
# 模型 ID: %s
# API Key: <Centag API Key>
#
# 或合并 User/settings.json 的 trae.customModels（见配置预览）
`, base, id)
}

func (t *TraeTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *TraeTemplate) VerifyCommand(info *BackendInfo) string {
	return `# 在 TRAE 对话输入框右下角选择模型「Centag」并发起一次简单对话`
}

func (t *TraeTemplate) Steps(info *BackendInfo) []ConfigStep {
	id := traeModelID(info)
	base := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "UI 添加自定义模型", Description: fmt.Sprintf("OpenAI 格式；Base URL=%s（关闭完整 URL）；模型 ID=%s", base, id)},
		{Title: "或合并 settings.json", Description: "写入 Trae/Trae CN 的 User/settings.json → trae.customModels 后重启 IDE"},
	}
}

func (t *TraeTemplate) WriteConfig(info *BackendInfo) error {
	id := traeModelID(info)
	base := proxyURL(info.Host, info.Port)
	paths := traeSettingsLogicalPaths()

	var wrote int
	var lastErr error
	for _, logical := range paths {
		abs := expandAppDataPath(logical)
		// 若父目录已存在（已安装对应版 IDE），则合并写入；否则跳过以免造空目录误导
		parent := filepath.Dir(filepath.Dir(abs)) // .../Trae or .../Trae CN
		if !fileExists(parent) && !fileExists(filepath.Dir(abs)) {
			continue
		}
		if err := mergeTraeCustomModel(abs, id, "Centag", info.APIKey, base); err != nil {
			lastErr = err
			continue
		}
		wrote++
	}
	if wrote == 0 {
		// 未检测到已安装目录时，写入国际版默认路径，便于用户首次安装后生效
		fallback := expandAppDataPath(paths[0])
		if err := mergeTraeCustomModel(fallback, id, "Centag", info.APIKey, base); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}
	}
	return nil
}
