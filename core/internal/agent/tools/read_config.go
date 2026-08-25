package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
)

// ReadConfigTool 读取配置文件工具
type ReadConfigTool struct {
	dataDir string
}

// NewReadConfigTool 创建读取配置文件工具
func NewReadConfigTool(dataDir string) agentcore.Tool {
	return &ReadConfigTool{
		dataDir: dataDir,
	}
}

// Name 返回工具名称
func (t *ReadConfigTool) Name() string {
	return "read_config"
}

// Description 返回工具描述
func (t *ReadConfigTool) Description() string {
	return "读取centag配置文件"
}

// Parameters 返回参数定义
func (t *ReadConfigTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "配置文件路径（相对于centag数据目录）",
			},
		},
		"required": []string{"path"},
	}
}

// IsReadOnly 返回是否为只读工具
func (t *ReadConfigTool) IsReadOnly() bool {
	return true
}

// ParamSchema 返回参数模式
func (t *ReadConfigTool) ParamSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "配置文件路径（相对于centag数据目录）",
			},
		},
		"required": []string{"path"},
	}
}

// Execute 执行工具
func (t *ReadConfigTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	path, _ := params["path"].(string)

	// path 为空时列出配置文件候选
	if path == "" {
		return listConfigCandidates(t.dataDir)
	}

	// 路径隔离校验（任务9 / R03）：拒绝逃逸 dataDir 的路径
	fullPath, err := secureResolve(t.dataDir, path)
	if err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("读取配置文件失败: %v", err)}, nil
	}
	
	// 读取文件
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("读取配置文件失败: %v", err)}, nil
	}
	
	// 尝试解析JSON
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		// 如果不是JSON，直接返回文本
		return &agentcore.ToolResult{Content: string(data)}, nil
	}
	
	// 格式化JSON输出
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &agentcore.ToolResult{Content: string(data)}, nil
	}
	
	return &agentcore.ToolResult{Content: string(jsonData)}, nil
}

// listConfigCandidates 列出数据目录下的配置文件候选
func listConfigCandidates(dataDir string) (*agentcore.ToolResult, error) {
	var b strings.Builder
	b.WriteString("未指定配置文件路径。请指定 path 参数，可选配置文件：\n")

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > 3 {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				if strings.Contains(e.Name(), "config") || strings.Contains(e.Name(), "data") || strings.Contains(e.Name(), "var") {
					walk(filepath.Join(dir, e.Name()), depth+1)
				}
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".toml") {
				rel, _ := filepath.Rel(dataDir, filepath.Join(dir, e.Name()))
				fmt.Fprintf(&b, "- %s\n", rel)
			}
		}
	}
	walk(dataDir, 0)

	if !strings.Contains(b.String(), "- ") {
		b.WriteString("（未找到配置文件，请检查数据目录）\n")
	}
	return &agentcore.ToolResult{Content: b.String()}, nil
}