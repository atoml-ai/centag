package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// TraeTemplate TRAE IDE 自定义模型接入
// 官方路径：设置 → 模型 → 添加模型 → 自定义配置（OpenAI）
// 文档：https://docs.trae.ai/ide/models?_lang=zh / https://docs.trae.cn/ide/models
//
// 请求地址填 OpenAI base …/v1，并关闭「完整 URL」（由 TRAE 拼接 /chat/completions）。
// TRAE 3.x 自定义模型由 IDE UI 管理，不会读取 settings.json 的 trae.customModels。
type TraeTemplate struct{}

func (t *TraeTemplate) AgentType() AgentType { return AgentTrae }
func (t *TraeTemplate) DisplayName() string  { return "TRAE" }
func (t *TraeTemplate) Description() string {
	return "字节跳动 TRAE IDE（自定义 OpenAI 兼容模型；需在 IDE 内添加）"
}

func (t *TraeTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:  AgentCategoryDesktop,
		WriteMode: WriteModeNone,
		ConfigPaths: []string{
			traeSetupGuideLogicalPath(),
		},
		KeyFields: []string{
			"API 格式=OpenAI",
			"完整 URL=关闭",
			"请求地址=…/v1",
			"模型 ID=centag/<pipeline>",
		},
		ConfigMethod:  "TRAE：设置 → 模型 → 添加模型 → 自定义配置。API 格式 OpenAI；关闭「完整 URL」；请求地址填 http://host:port/v1；模型 ID 填 centag/<pipeline>。",
		InstallURL:    "https://docs.trae.ai/ide/models?_lang=zh",
		InstallHint:   "从 trae.ai / trae.cn 下载 IDE；自定义模型必须在 IDE「设置 → 模型」中添加",
		AccessMethods: []AccessMethod{AccessUIGuide},
		UIGuide: &UIGuide{
			Title:          "在 TRAE 中填写自定义模型参数",
			Summary:        "打开 设置 → 模型 → 添加模型 → 自定义配置。请求地址填 …/v1（关闭完整 URL），模型 ID 随流水线变化。",
			DocURL:         "https://docs.trae.ai/ide/models?_lang=zh",
			Steps:          []string{"设置 → 模型 → 添加模型 → 自定义配置"},
			RequestURLKind: RequestURLOpenAIBase,
			FullURLMode:    "off",
			RestartHint:    "添加成功后若列表未刷新，完全退出并重启 TRAE",
		},
		VerifiedUI: true,
	}
}

func traeSetupGuideLogicalPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "~/Library/Application Support/Trae/CENTAG_SETUP.md"
	case "windows":
		return "%APPDATA%/Trae/CENTAG_SETUP.md"
	default:
		return "~/.config/Trae/CENTAG_SETUP.md"
	}
}

func traeAppSupportDirs() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			expandPath("~/Library/Application Support/Trae"),
			expandPath("~/Library/Application Support/Trae CN"),
		}
	case "windows":
		return []string{
			expandAppDataPath("%APPDATA%/Trae"),
			expandAppDataPath("%APPDATA%/Trae CN"),
		}
	default:
		return []string{
			expandPath("~/.config/Trae"),
			expandPath("~/.config/Trae CN"),
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

func traeSetupGuideContent(info *BackendInfo) string {
	id := traeModelID(info)
	base := proxyURL(info.Host, info.Port)
	return fmt.Sprintf(`# Centag × TRAE 接入说明

> TRAE 3.x 的自定义模型由 IDE「设置 → 模型」管理（云端同步），
> **不会**读取 User/settings.json 里的 trae.customModels。
> 请按下列步骤在 UI 中添加；本文件仅作参数备忘。

## 在 TRAE 中添加自定义模型

1. 打开 TRAE → 设置 → 模型 → 添加模型 → 自定义配置
2. API 格式：OpenAI
3. **关闭**「完整 URL」（由 TRAE 自动拼接 /chat/completions）
4. 请求地址（Base）：%s
5. 模型 ID：%s
6. API Key：你的 Centag API Key（Web → 个人资料 → API Keys）
7. 展示名称（可选）：Centag
8. 点击添加；连接检测通过后，在对话模型列表中选择 Centag
9. 若列表未刷新，请完全退出并重启 TRAE

## 参数摘要

| 项 | 值 |
|---|---|
| 完整 URL | 关闭 |
| 请求地址 | %s |
| 模型 ID | %s |

## 常见问题

- 只重启窗口不够时：菜单退出 TRAE 后再开
- 请求失败：确认 Centag 已运行，且地址为 …/v1（不要填到 /chat/completions）
- 若误开「完整 URL」，需改为 …/v1/chat/completions；推荐关闭完整 URL 只填 …/v1
- 模型列表没有 Centag：说明尚未在 UI 中添加成功（settings.json 无效）
`, base, id, base, id)
}

func (t *TraeTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	return []ConfigFile{
		{Path: traeSetupGuideLogicalPath(), Content: traeSetupGuideContent(info)},
	}, nil
}

func (t *TraeTemplate) SetupCommand(info *BackendInfo) string {
	id := traeModelID(info)
	base := proxyURL(info.Host, info.Port)
	return fmt.Sprintf(`# TRAE：必须在 IDE UI 添加自定义模型（settings.json 无效）
# 设置 → 模型 → 添加模型 → 自定义配置
# API 格式: OpenAI
# 完整 URL: 关闭
# 请求地址: %s
# 模型 ID: %s
# API Key: <Centag API Key>
# 然后完全退出并重启 TRAE
`, base, id)
}

func (t *TraeTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	guide := traeSetupGuideContent(info)
	script := "# See CENTAG_SETUP.md — configure TRAE via Settings → Models (UI)\n"
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + script + "#\n" + "# Guide preview:\n" + guide,
		Linux:   "#!/bin/bash\n" + script + "#\n" + "# Guide preview:\n" + guide,
		Windows: "# PowerShell\n" + script + "#\n" + "# Guide preview:\n" + guide,
	}
}

func (t *TraeTemplate) VerifyCommand(info *BackendInfo) string {
	return `# 在 TRAE「设置 → 模型」确认已添加 Centag，对话输入框右下角选择该模型并发起一次简单对话`
}

func (t *TraeTemplate) Steps(info *BackendInfo) []ConfigStep {
	id := traeModelID(info)
	base := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "UI 添加自定义模型", Description: fmt.Sprintf("OpenAI；关闭完整 URL；请求地址=%s；模型 ID=%s", base, id)},
		{Title: "重启 TRAE", Description: "完全退出 IDE 后重新打开，在模型列表中选择 Centag"},
	}
}

func (t *TraeTemplate) WriteConfig(info *BackendInfo) error {
	content := traeSetupGuideContent(info)
	dirs := traeAppSupportDirs()
	var wrote int
	var lastErr error
	for _, dir := range dirs {
		if !fileExists(dir) {
			continue
		}
		path := filepath.Join(dir, "CENTAG_SETUP.md")
		if err := os.MkdirAll(dir, 0755); err != nil {
			lastErr = err
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			lastErr = err
			continue
		}
		wrote++
	}
	if wrote == 0 {
		// 未检测到已安装目录时，写入国际版默认路径
		fallback := expandAppDataPath(traeSetupGuideLogicalPath())
		if err := os.MkdirAll(filepath.Dir(fallback), 0755); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}
		if err := os.WriteFile(fallback, []byte(content), 0644); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}
	}
	return nil
}
