package tool_call

import (
	"context"
	"encoding/json"
	"fmt"

	"centag/core/pkg/pipeline"
)

// RegisterToolCallPlugin 注册工具调用插件
func RegisterToolCallPlugin(registry *pipeline.NodeRegistry) {
	registry.RegisterPlugin(&ToolCallPlugin{})
}

// ToolCallPlugin 工具调用插件
type ToolCallPlugin struct{}

func (p *ToolCallPlugin) Descriptor() pipeline.NodePluginDescriptor {
	return pipeline.NodePluginDescriptor{
		Name:           "Tool Call",
		Implementation: "example.tool-call",
		Kind:           "tool.call",
		Version:        "1.0.0",
		Description:    "调用外部工具或API，并将结果注入流水线",
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tool_name": map[string]interface{}{
					"type":        "string",
					"description": "工具名称",
				},
				"parameters": map[string]interface{}{
					"type":                 "object",
					"description":          "工具调用参数",
					"additionalProperties": true,
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "超时时间（秒）",
					"default":     30,
				},
			},
			"required": []string{"tool_name"},
		},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"context": map[string]interface{}{
					"type":        "string",
					"description": "调用上下文",
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result": map[string]interface{}{
					"type":        "object",
					"description": "工具调用结果",
				},
				"raw_response": map[string]interface{}{
					"type":        "string",
					"description": "原始响应",
				},
			},
		},
		Permissions: []string{"network.outbound", "llm.call"},
	}
}

func (p *ToolCallPlugin) ValidateConfig(config pipeline.NodeConfig) error {
	return nil
}

func (p *ToolCallPlugin) Execute(ctx context.Context, req *pipeline.NodeExecutionRequest) (*pipeline.NodeExecutionResponse, error) {
	if req == nil || req.Input == nil {
		return nil, fmt.Errorf("invalid input")
	}

	// 获取配置
	toolName := ""
	parameters := make(map[string]interface{})
	timeout := 30
	
	if req.Config.CustomConfig != nil {
		toolName, _ = req.Config.CustomConfig["tool_name"].(string)
		if params, ok := req.Config.CustomConfig["parameters"].(map[string]interface{}); ok {
			parameters = params
		}
		if t, ok := req.Config.CustomConfig["timeout"].(float64); ok {
			timeout = int(t)
		}
	}
	
	if toolName == "" {
		return nil, fmt.Errorf("tool_name is required in config")
	}
	
	// 执行工具调用
	result, err := p.executeTool(ctx, toolName, parameters, timeout)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	
	// 构建结果
	output := map[string]interface{}{
		"status":       "success",
		"tool_name":    toolName,
		"result":       result,
		"executed_at":  fmt.Sprintf("%v", req.Input.Content),
	}
	
	// 如果有 CapabilityBroker，可以尝试使用它来获取工具
	if req.CapabilityBroker != nil {
		// TODO: 使用 CapabilityBroker 执行工具
		// 这里可以扩展为从 broker 获取工具并执行
	}
	
	resultJSON, _ := json.Marshal(output)
	
	return &pipeline.NodeExecutionResponse{
		Output: &pipeline.NodeOutput{
			Content: string(resultJSON),
			Metadata: map[string]interface{}{
				"tool_name":  toolName,
				"status":      "success",
			},
		},
	}, nil
}

// executeTool 执行工具
func (p *ToolCallPlugin) executeTool(ctx context.Context, toolName string, params map[string]interface{}, timeout int) (interface{}, error) {
	// 根据工具名称执行不同的操作
	switch toolName {
	case "http_request":
		return p.executeHTTPRequest(ctx, params, timeout)
	case "echo":
		return p.executeEcho(params), nil
	case "calculate":
		return p.executeCalculate(params), nil
	default:
		// 默认返回模拟结果
		return map[string]interface{}{
			"message": fmt.Sprintf("Tool %s executed successfully", toolName),
			"params":  params,
		}, nil
	}
}

// executeHTTPRequest 执行 HTTP 请求
func (p *ToolCallPlugin) executeHTTPRequest(ctx context.Context, params map[string]interface{}, timeout int) (interface{}, error) {
	url, _ := params["url"].(string)
	method, _ := params["method"].(string)
	if method == "" {
		method = "GET"
	}
	
	if url == "" {
		return nil, fmt.Errorf("url is required for http_request tool")
	}
	
	// 这里简化实现，实际需要发送 HTTP 请求
	return map[string]interface{}{
		"url":     url,
		"method":  method,
		"status":  "simulated",
		"message": "HTTP request would be sent here",
	}, nil
}

// executeEcho 回显参数
func (p *ToolCallPlugin) executeEcho(params map[string]interface{}) interface{} {
	return map[string]interface{}{
		"echo":   true,
		"params": params,
	}
}

// executeCalculate 执行简单计算
func (p *ToolCallPlugin) executeCalculate(params map[string]interface{}) interface{} {
	// 模拟计算操作
	operation, _ := params["operation"].(string)
	a, _ := params["a"].(float64)
	b, _ := params["b"].(float64)
	
	var result float64
	switch operation {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b != 0 {
			result = a / b
		}
	default:
		result = 0
	}
	
	return map[string]interface{}{
		"operation": operation,
		"a":         a,
		"b":         b,
		"result":    result,
	}
}
