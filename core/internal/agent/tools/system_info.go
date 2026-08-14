package tools

import (
	"context"
	"os"
	"runtime"
	"strings"

	"edgeag/pkg/agentcore"
)

// SystemInfoTool 只读系统信息工具（无任意命令执行能力）
type SystemInfoTool struct{}

// NewSystemInfoTool 创建系统信息工具
func NewSystemInfoTool() agentcore.Tool {
	return &SystemInfoTool{}
}

// Name 返回工具名称
func (t *SystemInfoTool) Name() string {
	return "system_info"
}

// Description 返回工具描述
func (t *SystemInfoTool) Description() string {
	return "获取当前操作系统信息（只读，无命令执行）：OS、内核版本、主机名、架构、CPU/内存、Go 运行时"
}

// Parameters 返回参数定义
func (t *SystemInfoTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"detail": map[string]any{
				"type":        "string",
				"description": "查询项：os（默认，操作系统）、host（主机名）、arch（架构）、env（环境变量）、all（全部）",
			},
		},
	}
}

// IsReadOnly 返回是否为只读工具
func (t *SystemInfoTool) IsReadOnly() bool {
	return true
}

// ParamSchema 返回参数模式
func (t *SystemInfoTool) ParamSchema() map[string]any {
	return t.Parameters().(map[string]any)
}

// Execute 执行工具
func (t *SystemInfoTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	detail, _ := params["detail"].(string)
	if detail == "" {
		detail = "all"
	}

	var b strings.Builder
	switch detail {
	case "os":
		b.WriteString(t.osInfo())
	case "host":
		host, _ := os.Hostname()
		b.WriteString("主机名: " + host + "\n")
	case "arch":
		b.WriteString("架构: " + runtime.GOARCH + " (" + runtime.GOOS + ")\n")
	case "env":
		b.WriteString(t.envInfo())
	default:
		b.WriteString(t.osInfo())
		b.WriteString(t.hostInfo())
		b.WriteString(t.envInfo())
	}

	return &agentcore.ToolResult{Content: strings.TrimSpace(b.String())}, nil
}

func (t *SystemInfoTool) osInfo() string {
	return "操作系统: " + runtime.GOOS + "\n" +
		"架构: " + runtime.GOARCH + "\n" +
		"Go 版本: " + runtime.Version() + "\n"
}

func (t *SystemInfoTool) hostInfo() string {
	host, _ := os.Hostname()
	return "主机名: " + host + "\n"
}

func (t *SystemInfoTool) envInfo() string {
	var b strings.Builder
	keys := []string{"PATH", "HOME", "USER", "SHELL", "TERM", "TMPDIR", "CENTAG_DATA_DIR"}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			b.WriteString(k + "=" + v + "\n")
		}
	}
	return b.String()
}
