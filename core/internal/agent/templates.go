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

// writeFiles 将配置文件写入本地文件系统
func writeFiles(files []ConfigFile) error {
	for _, f := range files {
		path := expandPath(f.Path)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", dir, err)
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
