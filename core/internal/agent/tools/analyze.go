package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
)

// AnalyzeTool 分析工具
type AnalyzeTool struct{}

// NewAnalyzeTool 创建分析工具
func NewAnalyzeTool() agentcore.Tool {
	return &AnalyzeTool{}
}

// Name 返回工具名称
func (t *AnalyzeTool) Name() string {
	return "analyze"
}

// Description 返回工具描述
func (t *AnalyzeTool) Description() string {
	return "分析数据并生成报告"
}

// Parameters 返回参数定义
func (t *AnalyzeTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "string",
				"description": "要分析的数据",
			},
			"type": map[string]any{
				"type":        "string",
				"description": "分析类型（status, config, error, log, strategy）",
				"enum":        []string{"status", "config", "error", "log", "strategy"},
			},
		},
		"required": []string{"data", "type"},
	}
}

// IsReadOnly 返回是否为只读工具
func (t *AnalyzeTool) IsReadOnly() bool {
	return true
}

// ParamSchema 返回参数模式
func (t *AnalyzeTool) ParamSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "string",
				"description": "要分析的数据",
			},
			"type": map[string]any{
				"type":        "string",
				"description": "分析类型（status, config, error, log, strategy）",
				"enum":        []string{"status", "config", "error", "log", "strategy"},
			},
		},
		"required": []string{"data", "type"},
	}
}

// Execute 执行工具
func (t *AnalyzeTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	data, ok := params["data"].(string)
	if !ok {
		return &agentcore.ToolResult{IsError: true, Content: "missing 'data' parameter"}, nil
	}
	
	analysisType, ok := params["type"].(string)
	if !ok {
		return &agentcore.ToolResult{IsError: true, Content: "missing 'type' parameter"}, nil
	}
	
	var result string
	
	switch analysisType {
	case "status":
		result = t.analyzeStatus(data)
	case "config":
		result = t.analyzeConfig(data)
	case "error":
		result = t.analyzeError(data)
	case "log":
		result = t.analyzeLog(data)
	case "strategy":
		result = t.analyzeStrategy(data)
	default:
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("不支持的分析类型: %s", analysisType)}, nil
	}
	
	return &agentcore.ToolResult{Content: result}, nil
}

// analyzeStatus 分析状态数据
func (t *AnalyzeTool) analyzeStatus(data string) string {
	var analysis []string
	
	analysis = append(analysis, "=== 状态分析报告 ===")
	analysis = append(analysis, "")
	
	// 简单分析
	if strings.Contains(data, "error") || strings.Contains(data, "Error") {
		analysis = append(analysis, "⚠️ 发现错误信息")
	}
	
	if strings.Contains(data, "warning") || strings.Contains(data, "Warning") {
		analysis = append(analysis, "⚠️ 发现警告信息")
	}
	
	if strings.Contains(data, "success") || strings.Contains(data, "ok") {
		analysis = append(analysis, "✅ 系统运行正常")
	}
	
	analysis = append(analysis, "")
	analysis = append(analysis, "建议：")
	analysis = append(analysis, "1. 检查配置文件是否正确")
	analysis = append(analysis, "2. 查看日志文件获取详细信息")
	analysis = append(analysis, "3. 确认数据库连接正常")
	
	return strings.Join(analysis, "\n")
}

// analyzeConfig 分析配置数据
func (t *AnalyzeTool) analyzeConfig(data string) string {
	var analysis []string
	
	analysis = append(analysis, "=== 配置分析报告 ===")
	analysis = append(analysis, "")
	
	// 简单分析
	if strings.Contains(data, "backend") {
		analysis = append(analysis, "✅ 检测到后端配置")
	}
	
	if strings.Contains(data, "model") {
		analysis = append(analysis, "✅ 检测到模型配置")
	}
	
	if strings.Contains(data, "pipeline") {
		analysis = append(analysis, "✅ 检测到流水线配置")
	}
	
	analysis = append(analysis, "")
	analysis = append(analysis, "建议：")
	analysis = append(analysis, "1. 确保后端配置正确")
	analysis = append(analysis, "2. 检查模型设置是否合理")
	analysis = append(analysis, "3. 验证流水线配置")
	
	return strings.Join(analysis, "\n")
}

// analyzeError 分析错误数据
func (t *AnalyzeTool) analyzeError(data string) string {
	var analysis []string
	
	analysis = append(analysis, "=== 错误分析报告 ===")
	analysis = append(analysis, "")
	
	// 简单分析
	if strings.Contains(data, "timeout") || strings.Contains(data, "Timeout") {
		analysis = append(analysis, "⚠️ 检测到超时错误")
		analysis = append(analysis, "建议：增加超时时间或优化性能")
	}
	
	if strings.Contains(data, "connection") || strings.Contains(data, "connect") {
		analysis = append(analysis, "⚠️ 检测到连接错误")
		analysis = append(analysis, "建议：检查网络连接和服务状态")
	}
	
	if strings.Contains(data, "permission") || strings.Contains(data, "access") {
		analysis = append(analysis, "⚠️ 检测到权限错误")
		analysis = append(analysis, "建议：检查文件权限和用户权限")
	}
	
	analysis = append(analysis, "")
	analysis = append(analysis, "建议：")
	analysis = append(analysis, "1. 查看详细错误日志")
	analysis = append(analysis, "2. 检查系统资源使用情况")
	analysis = append(analysis, "3. 验证配置文件设置")
	
	return strings.Join(analysis, "\n")
}

// analyzeLog 分析日志数据
func (t *AnalyzeTool) analyzeLog(data string) string {
	var analysis []string
	
	analysis = append(analysis, "=== 日志分析报告 ===")
	analysis = append(analysis, "")
	
	// 统计错误数量
	errorCount := strings.Count(data, "error") + strings.Count(data, "Error")
	warningCount := strings.Count(data, "warning") + strings.Count(data, "Warning")
	
	analysis = append(analysis, fmt.Sprintf("错误数量: %d", errorCount))
	analysis = append(analysis, fmt.Sprintf("警告数量: %d", warningCount))
	
	analysis = append(analysis, "")
	analysis = append(analysis, "建议：")
	analysis = append(analysis, "1. 关注高频错误")
	analysis = append(analysis, "2. 处理重要警告")
	analysis = append(analysis, "3. 优化日志输出")
	
	return strings.Join(analysis, "\n")
}

// analyzeStrategy 分析策略数据
func (t *AnalyzeTool) analyzeStrategy(data string) string {
	var analysis []string
	
	analysis = append(analysis, "=== 策略分析报告 ===")
	analysis = append(analysis, "")
	
	// 简单分析
	if strings.Contains(data, "routing") || strings.Contains(data, "route") {
		analysis = append(analysis, "✅ 检测到路由策略")
	}
	
	if strings.Contains(data, "cache") {
		analysis = append(analysis, "✅ 检测到缓存策略")
	}
	
	if strings.Contains(data, "fallback") {
		analysis = append(analysis, "✅ 检测到降级策略")
	}
	
	analysis = append(analysis, "")
	analysis = append(analysis, "建议：")
	analysis = append(analysis, "1. 优化路由策略")
	analysis = append(analysis, "2. 调整缓存设置")
	analysis = append(analysis, "3. 完善降级方案")
	
	return strings.Join(analysis, "\n")
}