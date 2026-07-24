package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// proxyURL 构建 Centag 访问地址
func proxyURL(host string, port int) string {
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 20060
	}
	return fmt.Sprintf("http://%s:%d/v1", host, port)
}

// defaultModel 返回默认模型名称
func defaultModel(info *BackendInfo) string {
	if info.Model != "" {
		return info.Model
	}
	return "gpt-4o"
}

// centagAPIModelID 返回发给 Centag API 的 model id（与 /v1/models 的 id 一致）。
func centagAPIModelID(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "gpt-4o"
	}
	if strings.HasPrefix(model, "centag/") {
		return model
	}
	// 兼容旧的 pipeline.<id>：归一为 centag/<id>
	if strings.HasPrefix(model, "pipeline.") {
		return "centag/" + strings.TrimPrefix(model, "pipeline.")
	}
	return model
}

// centagModelRef 生成 OpenCode/OpenClaw 的 provider/model 引用。
// API model 若已是 centag/<id>，则写成 centag/centag/<id>（provider=centag，发给 API 的为 centag/<id>）。
func centagModelRef(model string) string {
	apiModel := centagAPIModelID(model)
	return "centag/" + apiModel
}

// expandPath 将 ~ 展开为实际 home 目录
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// generateShellCommands 生成 macOS/Linux 的 shell 写入命令
func generateShellCommands(files []ConfigFile) string {
	var cmds []string
	for _, f := range files {
		path := expandPath(f.Path)
		dir := filepath.Dir(path)
		cmds = append(cmds, fmt.Sprintf("mkdir -p %s", dir))
		if f.Append {
			cmds = append(cmds, fmt.Sprintf("cat >> %s << 'AGENTEOF'\n%s\nAGENTEOF", path, f.Content))
		} else {
			cmds = append(cmds, fmt.Sprintf("cat > %s << 'AGENTEOF'\n%s\nAGENTEOF", path, f.Content))
		}
	}
	return strings.Join(cmds, "\n")
}

// generatePowerShellCommands 生成 Windows 的 PowerShell 写入命令
func generatePowerShellCommands(files []ConfigFile) string {
	var cmds []string
	cmds = append(cmds, "# PowerShell 配置写入")
	for _, f := range files {
		// 将 ~ 转换为 $env:USERPROFILE
		path := strings.Replace(f.Path, "~", "$env:USERPROFILE", 1)
		if f.Append {
			cmds = append(cmds, fmt.Sprintf("Add-Content -Path '%s' -Value @'\n%s\n'@", path, f.Content))
		} else {
			cmds = append(cmds, fmt.Sprintf("Set-Content -Path '%s' -Value @'\n%s\n'@", path, f.Content))
		}
	}
	return strings.Join(cmds, "\n\n")
}

const centagBackupSuffix = ".centag-bak"

func backupPath(path string) string {
	return path + centagBackupSuffix
}

// writeFiles 将配置文件写入本地文件系统。
// 首次覆盖已有文件时会另存 .centag-bak，供「恢复默认」还原。
func writeFiles(files []ConfigFile) error {
	for _, f := range files {
		path := expandPath(f.Path)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", dir, err)
		}

		if err := ensureCentagBackup(path); err != nil {
			return err
		}

		var data []byte
		if f.Append && fileExists(path) {
			existing, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("读取文件失败 %s: %w", path, err)
			}
			data = append(existing, '\n')
			data = append(data, []byte(f.Content)...)
		} else {
			data = []byte(f.Content)
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("写入文件失败 %s: %w", path, err)
		}
	}
	return nil
}

func ensureCentagBackup(path string) error {
	if !fileExists(path) || fileExists(backupPath(path)) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("备份读取失败 %s: %w", path, err)
	}
	if err := os.WriteFile(backupPath(path), data, 0644); err != nil {
		return fmt.Errorf("备份写入失败 %s: %w", backupPath(path), err)
	}
	return nil
}

// RestoreResult 单个文件的恢复结果
type RestoreResult struct {
	Path   string `json:"path"`
	Action string `json:"action"` // restored | removed | skipped
}

// RestoreConfigFiles 恢复 Centag 写入前的配置：
// - 存在 .centag-bak → 还原并删除备份
// - 无备份但目标文件存在 → 视为 Centag 新建，删除该文件
// - 都不存在 → skipped
func RestoreConfigFiles(files []ConfigFile) ([]RestoreResult, error) {
	results := make([]RestoreResult, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		path := expandPath(f.Path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		bak := backupPath(path)
		switch {
		case fileExists(bak):
			data, err := os.ReadFile(bak)
			if err != nil {
				return results, fmt.Errorf("读取备份失败 %s: %w", bak, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return results, fmt.Errorf("创建目录失败 %s: %w", filepath.Dir(path), err)
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				return results, fmt.Errorf("还原文件失败 %s: %w", path, err)
			}
			_ = os.Remove(bak)
			results = append(results, RestoreResult{Path: path, Action: "restored"})
		case fileExists(path):
			if err := os.Remove(path); err != nil {
				return results, fmt.Errorf("删除 Centag 配置失败 %s: %w", path, err)
			}
			results = append(results, RestoreResult{Path: path, Action: "removed"})
		default:
			results = append(results, RestoreResult{Path: path, Action: "skipped"})
		}
	}
	return results, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
