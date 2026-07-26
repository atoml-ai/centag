package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Centag Claude Desktop 3P profile（独立于 cc-switch 的 PROFILE_ID，避免互相覆盖）
const (
	claudeDesktopProfileID   = "00000000-0000-4000-8000-0000000c3a7a"
	claudeDesktopProfileName = "Centag"
	claudeDesktopConfigFile  = "claude_desktop_config.json"
	claudeDesktopLibraryDir  = "configLibrary"
)

// ClaudeDesktopTemplate Claude Desktop 配置模板（对齐 cc-switch gateway profile）
type ClaudeDesktopTemplate struct{}

func (t *ClaudeDesktopTemplate) AgentType() AgentType { return AgentClaudeDesktop }
func (t *ClaudeDesktopTemplate) DisplayName() string  { return "Claude Desktop" }
func (t *ClaudeDesktopTemplate) Description() string  { return "Anthropic 官方的桌面客户端" }

func (t *ClaudeDesktopTemplate) Meta() AgentSetupMeta {
	paths := claudeDesktopLogicalPaths()
	method := "macOS/Windows：写入 Claude + Claude-3p 的 deploymentMode=3p，并在 configLibrary 写入 Centag gateway profile（inferenceGatewayBaseUrl / ApiKey / AuthScheme=bearer / inferenceProvider=gateway）。需重启 Desktop。Linux 不支持写入。"
	if paths == nil {
		method = "当前平台不支持 Claude Desktop 本地写入（仅 macOS / Windows）。请在支持的系统上使用一键配置。"
	}
	var pathList []string
	if paths != nil {
		pathList = []string{paths.normalConfig, paths.threepConfig, paths.profile, paths.meta}
	}
	return AgentSetupMeta{
		Category:      AgentCategoryDesktop,
		WriteMode:     WriteModeOverwrite,
		ConfigPaths:   pathList,
		KeyFields:     []string{"inferenceGatewayBaseUrl", "inferenceGatewayApiKey", "inferenceGatewayAuthScheme", "inferenceProvider", "deploymentMode"},
		ConfigMethod:  method,
		InstallURL:    "https://claude.ai/download",
		InstallHint:   "从官网下载 Claude Desktop 桌面应用（非 CLI）",
		AccessMethods: []AccessMethod{AccessWriteConfig},
	}
}

// claudeDesktopLogicalPaths 返回可展示 / 可 expand 的逻辑路径（含 ~ 或 %LOCALAPPDATA%）。
type claudeDesktopPaths struct {
	normalConfig string
	threepConfig string
	profile      string
	meta         string
}

func claudeDesktopLogicalPaths() *claudeDesktopPaths {
	switch runtime.GOOS {
	case "darwin":
		base := "~/Library/Application Support"
		lib := base + "/Claude-3p/" + claudeDesktopLibraryDir
		return &claudeDesktopPaths{
			normalConfig: base + "/Claude/" + claudeDesktopConfigFile,
			threepConfig: base + "/Claude-3p/" + claudeDesktopConfigFile,
			profile:      lib + "/" + claudeDesktopProfileID + ".json",
			meta:         lib + "/_meta.json",
		}
	case "windows":
		base := "%LOCALAPPDATA%"
		lib := base + "\\Claude-3p\\" + claudeDesktopLibraryDir
		return &claudeDesktopPaths{
			normalConfig: base + "\\Claude\\" + claudeDesktopConfigFile,
			threepConfig: base + "\\Claude-3p\\" + claudeDesktopConfigFile,
			profile:      lib + "\\" + claudeDesktopProfileID + ".json",
			meta:         lib + "\\_meta.json",
		}
	default:
		return nil
	}
}

func expandClaudeDesktopPath(p string) string {
	if strings.HasPrefix(p, "%LOCALAPPDATA%") {
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			home, _ := os.UserHomeDir()
			local = filepath.Join(home, "AppData", "Local")
		}
		rest := strings.TrimPrefix(p, "%LOCALAPPDATA%")
		rest = strings.TrimPrefix(rest, `\`)
		rest = strings.TrimPrefix(rest, `/`)
		return filepath.Join(append([]string{local}, strings.FieldsFunc(rest, func(r rune) bool {
			return r == '\\' || r == '/'
		})...)...)
	}
	return expandPath(p)
}

func (t *ClaudeDesktopTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	paths := claudeDesktopLogicalPaths()
	if paths == nil {
		return nil, fmt.Errorf("Claude Desktop 配置写入仅支持 macOS / Windows")
	}
	url := proxyURL(info.Host, info.Port)
	profile := fmt.Sprintf(`{
  "coworkEgressAllowedHosts": ["*"],
  "disableDeploymentModeChooser": true,
  "inferenceGatewayApiKey": "%s",
  "inferenceGatewayAuthScheme": "bearer",
  "inferenceGatewayBaseUrl": "%s",
  "inferenceProvider": "gateway"
}`, info.APIKey, url)
	deploy := `{
  "deploymentMode": "3p"
}`
	meta := fmt.Sprintf(`{
  "appliedId": "%s",
  "entries": [{"id": "%s", "name": "%s"}]
}`, claudeDesktopProfileID, claudeDesktopProfileID, claudeDesktopProfileName)
	return []ConfigFile{
		{Path: paths.normalConfig, Content: deploy},
		{Path: paths.threepConfig, Content: deploy},
		{Path: paths.profile, Content: profile},
		{Path: paths.meta, Content: meta},
	}, nil
}

func (t *ClaudeDesktopTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	return fmt.Sprintf(`# Claude Desktop gateway profile（对齐 cc-switch Direct）
# 设置 inferenceGatewayBaseUrl="%s"
# 设置 inferenceGatewayApiKey="<Centag API Key>"
# 设置 inferenceGatewayAuthScheme="bearer"
# 设置 inferenceProvider="gateway"
# 并将 deploymentMode 设为 3p 后重启 Desktop
`, url)
}

func (t *ClaudeDesktopTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, err := t.ConfigFiles(info)
	if err != nil {
		msg := "# " + err.Error()
		return PlatformCommands{MacOS: msg, Linux: msg, Windows: msg}
	}
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "# Claude Desktop 配置写入不支持 Linux\n",
		Windows: generatePowerShellCommands(files),
	}
}

func (t *ClaudeDesktopTemplate) VerifyCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	return fmt.Sprintf(`curl -s %s/models -H "x-api-key: %s"`, url, info.APIKey)
}

func (t *ClaudeDesktopTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "写入 3P gateway profile", Description: fmt.Sprintf("Centag gateway: %s（需重启 Claude Desktop）", url)},
	}
}

func (t *ClaudeDesktopTemplate) WriteConfig(info *BackendInfo) error {
	paths := claudeDesktopLogicalPaths()
	if paths == nil {
		return fmt.Errorf("Claude Desktop 配置写入仅支持 macOS / Windows")
	}
	url := proxyURL(info.Host, info.Port)

	if err := mergeDeploymentMode(expandClaudeDesktopPath(paths.normalConfig), "3p"); err != nil {
		return err
	}
	if err := mergeDeploymentMode(expandClaudeDesktopPath(paths.threepConfig), "3p"); err != nil {
		return err
	}

	profile := map[string]interface{}{
		"coworkEgressAllowedHosts":     []string{"*"},
		"disableDeploymentModeChooser": true,
		"inferenceGatewayApiKey":       info.APIKey,
		"inferenceGatewayAuthScheme":   "bearer",
		"inferenceGatewayBaseUrl":      url,
		"inferenceProvider":            "gateway",
	}
	if err := writeJSONMap(expandClaudeDesktopPath(paths.profile), profile); err != nil {
		return err
	}
	return writeClaudeDesktopMeta(expandClaudeDesktopPath(paths.meta))
}

func mergeDeploymentMode(path, mode string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}
	root["deploymentMode"] = mode
	return writeJSONMap(path, root)
}

func writeClaudeDesktopMeta(path string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}
	entries, _ := root["entries"].([]interface{})
	filtered := make([]interface{}, 0, len(entries)+1)
	for _, e := range entries {
		m, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == claudeDesktopProfileID {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered, map[string]interface{}{
		"id":   claudeDesktopProfileID,
		"name": claudeDesktopProfileName,
	})
	root["entries"] = filtered
	root["appliedId"] = claudeDesktopProfileID
	return writeJSONMap(path, root)
}
