package json_validator

import (
	"context"
	"encoding/json"
	"fmt"

	"centag/core/pkg/pipeline"
)

// RegisterJSONValidatorPlugin 注册 JSON 校验插件
func RegisterJSONValidatorPlugin(registry *pipeline.NodeRegistry) {
	registry.RegisterPlugin(&JSONValidatorPlugin{})
}

// JSONValidatorPlugin 结构化 JSON 校验插件
type JSONValidatorPlugin struct{}

func (p *JSONValidatorPlugin) Descriptor() pipeline.NodePluginDescriptor {
	return pipeline.NodePluginDescriptor{
		Name:           "JSON Validator",
		Implementation: "example.json-validator",
		Kind:           "validate.json",
		Version:        "1.0.0",
		Description:    "校验 JSON 结构是否符合指定的 JSON Schema",
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"schema": map[string]interface{}{
					"type":        "object",
					"description": "JSON Schema 定义",
				},
				"strict": map[string]interface{}{
					"type":        "boolean",
					"description": "是否严格模式（额外字段报错）",
					"default":     false,
				},
			},
			"required": []string{"schema"},
		},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "需要校验的 JSON 字符串",
				},
			},
			"required": []string{"content"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"valid": map[string]interface{}{
					"type":        "boolean",
					"description": "是否通过校验",
				},
				"errors": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"normalized": map[string]interface{}{
					"type":        "object",
					"description": "标准化后的对象",
				},
			},
		},
		Permissions: []string{},
	}
}

func (p *JSONValidatorPlugin) ValidateConfig(config pipeline.NodeConfig) error {
	return nil
}

func (p *JSONValidatorPlugin) Execute(ctx context.Context, req *pipeline.NodeExecutionRequest) (*pipeline.NodeExecutionResponse, error) {
	if req == nil || req.Input == nil {
		return nil, fmt.Errorf("invalid input")
	}

	content := req.Input.Content
	if content == "" {
		return &pipeline.NodeExecutionResponse{
			Output: &pipeline.NodeOutput{
				Content: "{}",
				Metadata: map[string]interface{}{
					"valid":   false,
					"errors": []string{"empty content"},
				},
			},
		}, nil
	}

	// 尝试解析 JSON
	var data interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return &pipeline.NodeExecutionResponse{
			Output: &pipeline.NodeOutput{
				Content: content,
				Metadata: map[string]interface{}{
					"valid":   false,
					"errors": []string{err.Error()},
				},
			},
		}, nil
	}

	// 获取 schema 配置
	var schema map[string]interface{}
	if req.Config.CustomConfig != nil {
		if schemaObj, ok := req.Config.CustomConfig["schema"]; ok && schemaObj != nil {
			schema, _ = schemaObj.(map[string]interface{})
		}
	}
	
	if schema == nil {
		return &pipeline.NodeExecutionResponse{
			Output: &pipeline.NodeOutput{
				Content: content,
				Metadata: map[string]interface{}{
					"valid":  false,
					"errors": []string{"schema is required in config"},
				},
			},
		}, nil
	}

	// 执行 JSON Schema 校验
	errors := p.validateJSON(data, schema, "")

	if len(errors) > 0 {
		return &pipeline.NodeExecutionResponse{
			Output: &pipeline.NodeOutput{
				Content: content,
				Metadata: map[string]interface{}{
					"valid":        false,
					"errors":       errors,
					"normalized":   data,
				},
			},
		}, nil
	}

	return &pipeline.NodeExecutionResponse{
		Output: &pipeline.NodeOutput{
			Content: content,
			Metadata: map[string]interface{}{
				"valid":        true,
				"errors":       []string{},
				"normalized":   data,
			},
		},
	}, nil
}

// validateJSON 执行 JSON Schema 校验
func (p *JSONValidatorPlugin) validateJSON(data interface{}, schema map[string]interface{}, path string) []string {
	var errors []string

	// 检查类型
	if schemaType, ok := schema["type"].(string); ok {
		if err := p.checkType(data, schemaType, path); err != "" {
			errors = append(errors, err)
		}
	}

	// 根据数据类型进行进一步校验
	switch d := data.(type) {
	case map[string]interface{}:
		errors = append(errors, p.validateObject(d, schema, path)...)
	case []interface{}:
		errors = append(errors, p.validateArray(d, schema, path)...)
	}

	return errors
}

// checkType 检查数据类型
func (p *JSONValidatorPlugin) checkType(data interface{}, expectedType, path string) string {
	actualType := p.getJSONType(data)
	if actualType != expectedType {
		return fmt.Sprintf("%s: expected type %s, got %s", path, expectedType, actualType)
	}
	return ""
}

// getJSONType 获取 JSON 数据类型
func (p *JSONValidatorPlugin) getJSONType(data interface{}) string {
	switch data.(type) {
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

// validateObject 校验对象
func (p *JSONValidatorPlugin) validateObject(obj map[string]interface{}, schema map[string]interface{}, path string) []string {
	var errors []string

	// 检查必需字段
	if required, ok := schema["required"].([]interface{}); ok {
		for _, req := range required {
			if fieldName, ok := req.(string); ok {
				if _, exists := obj[fieldName]; !exists {
					fieldPath := fieldName
					if path != "" {
						fieldPath = path + "." + fieldName
					}
					errors = append(errors, fmt.Sprintf("%s: required field missing", fieldPath))
				}
			}
		}
	}

	// 检查属性
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for fieldName, fieldSchemaObj := range properties {
			fieldPath := fieldName
			if path != "" {
				fieldPath = path + "." + fieldName
			}

			fieldSchema, ok := fieldSchemaObj.(map[string]interface{})
			if !ok {
				continue
			}

			// 如果字段存在，校验它
			if fieldValue, exists := obj[fieldName]; exists {
				errors = append(errors, p.validateJSON(fieldValue, fieldSchema, fieldPath)...)
			}
		}
	}

	return errors
}

// validateArray 校验数组
func (p *JSONValidatorPlugin) validateArray(arr []interface{}, schema map[string]interface{}, path string) []string {
	var errors []string

	// 检查最小项数
	if minItems, ok := schema["minItems"].(float64); ok {
		if float64(len(arr)) < minItems {
			errors = append(errors, fmt.Sprintf("%s: array has %d items, minimum is %v", path, len(arr), minItems))
		}
	}

	// 检查最大项数
	if maxItems, ok := schema["maxItems"].(float64); ok {
		if float64(len(arr)) > maxItems {
			errors = append(errors, fmt.Sprintf("%s: array has %d items, maximum is %v", path, len(arr), maxItems))
		}
	}

	// 校验数组项
	if items, ok := schema["items"].(map[string]interface{}); ok {
		for i, item := range arr {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			errors = append(errors, p.validateJSON(item, items, itemPath)...)
		}
	}

	return errors
}
