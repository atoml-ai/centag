package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"edgeag/pkg/agentcore"
)

// WriteConfigTool 写入配置文件工具
type WriteConfigTool struct {
	dataDir string
}

// NewWriteConfigTool 创建写入配置文件工具
func NewWriteConfigTool(dataDir string) agentcore.Tool {
	return &WriteConfigTool{
		dataDir: dataDir,
	}
}

// Name 返回工具名称
func (t *WriteConfigTool) Name() string {
	return "write_config"
}

// Description 返回工具描述
func (t *WriteConfigTool) Description() string {
	return "写入centag配置文件"
}

// Parameters 返回参数定义
func (t *WriteConfigTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "配置文件路径（相对于centag数据目录）",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "配置内容（JSON格式）",
			},
		},
		"required": []string{"path", "content"},
	}
}

// IsReadOnly 返回是否为只读工具
func (t *WriteConfigTool) IsReadOnly() bool {
	return false
}

// ParamSchema 返回参数模式
func (t *WriteConfigTool) ParamSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "配置文件路径（相对于centag数据目录）",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "配置内容（JSON格式）",
			},
		},
		"required": []string{"path", "content"},
	}
}

// Execute 执行工具
func (t *WriteConfigTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	path, ok := params["path"].(string)
	if !ok {
		return &agentcore.ToolResult{IsError: true, Content: "missing 'path' parameter"}, nil
	}
	
	content, ok := params["content"].(string)
	if !ok {
		return &agentcore.ToolResult{IsError: true, Content: "missing 'content' parameter"}, nil
	}
	
	// 验证JSON格式
	var jsonContent interface{}
	if err := json.Unmarshal([]byte(content), &jsonContent); err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("内容不是有效的JSON格式: %v", err)}, nil
	}
	
	// 路径隔离校验（任务9 / R03）：拒绝逃逸 dataDir 的路径
	fullPath, err := secureResolve(t.dataDir, path)
	if err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("写入配置文件失败: %v", err)}, nil
	}
	
	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("创建目录失败: %v", err)}, nil
	}
	
	// 格式化JSON
	jsonData, err := json.MarshalIndent(jsonContent, "", "  ")
	if err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("格式化JSON失败: %v", err)}, nil
	}
	
	// 写入文件
	if err := os.WriteFile(fullPath, jsonData, 0644); err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("写入配置文件失败: %v", err)}, nil
	}
	
	return &agentcore.ToolResult{Content: fmt.Sprintf("成功写入配置文件: %s", path)}, nil
}