package pii_redact

import (
	"context"
	"fmt"
	"regexp"

	"centag/core/pkg/pipeline"
)

// RegisterPIIRedactPlugin 注册 PII 脱敏插件
// 调用方需要在初始化时调用此函数
func RegisterPIIRedactPlugin(registry *pipeline.NodeRegistry) {
	registry.RegisterPlugin(&PIIRedactPlugin{})
}

// PIIRedactPlugin PII 脱敏插件
type PIIRedactPlugin struct{}

func (p *PIIRedactPlugin) Descriptor() pipeline.NodePluginDescriptor {
	return pipeline.NodePluginDescriptor{
		Name:           "PII Redact",
		Implementation: "example.pii-redact",
		Kind:           "security.pii-redact",
		Version:        "1.0.0",
		Description:    "自动识别并脱敏文本中的个人敏感信息（手机号、邮箱、身份证等）",
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"fields": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"phone", "email", "id_card", "bank_card", "address"},
					},
					"description": "需要脱敏的字段类型，空表示全部",
				},
				"mask_char": map[string]interface{}{
					"type":        "string",
					"description": "脱敏替换字符，默认 *",
					"default":     "*",
				},
			},
		},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "需要脱敏的文本",
				},
			},
			"required": []string{"content"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "脱敏后的文本",
				},
				"redacted_count": map[string]interface{}{
					"type":        "integer",
					"description": "脱敏条目数",
				},
			},
		},
		Permissions: []string{"llm.call"},
	}
}

func (p *PIIRedactPlugin) ValidateConfig(config pipeline.NodeConfig) error {
	// 验证配置
	return nil
}

func (p *PIIRedactPlugin) Execute(ctx context.Context, req *pipeline.NodeExecutionRequest) (*pipeline.NodeExecutionResponse, error) {
	if req == nil || req.Input == nil {
		return nil, fmt.Errorf("invalid input")
	}

	content := req.Input.Content
	if content == "" {
		return &pipeline.NodeExecutionResponse{
			Output: &pipeline.NodeOutput{
				Content: "",
				Metadata: map[string]interface{}{
					"redacted_count": 0,
				},
			},
		}, nil
	}

	// 获取配置
	maskChar := "*"
	if config, ok := req.Config.CustomConfig["mask_char"]; ok {
		if s, ok := config.(string); ok && s != "" {
			maskChar = s
		}
	}

	// 脱敏规则
	redactedCount := 0

	// 手机号：1[3-9]\d{9}
	phoneRe := regexp.MustCompile(`1[3-9]\d{9}`)
	content = phoneRe.ReplaceAllStringFunc(content, func(match string) string {
		redactedCount++
		return maskString(match, maskChar)
	})

	// 邮箱：\S+@\S+\.\S+
	emailRe := regexp.MustCompile(`\S+@\S+\.\S+`)
	content = emailRe.ReplaceAllStringFunc(content, func(match string) string {
		redactedCount++
		return maskString(match, maskChar)
	})

	// 身份证：18位或15位
	idCardRe := regexp.MustCompile(`\d{17}[\dXx]|\d{15}`)
	content = idCardRe.ReplaceAllStringFunc(content, func(match string) string {
		redactedCount++
		return maskString(match, maskChar)
	})

	return &pipeline.NodeExecutionResponse{
		Output: &pipeline.NodeOutput{
			Content: content,
			Metadata: map[string]interface{}{
				"redacted_count": redactedCount,
			},
		},
	}, nil
}

// maskString 将字符串中间部分替换为掩码字符
func maskString(s string, maskChar string) string {
	if len(s) <= 4 {
		return s
	}
	prefix := s[:2]
	suffix := s[len(s)-2:]
	masked := ""
	for i := 0; i < len(s)-4; i++ {
		masked += maskChar
	}
	return prefix + masked + suffix
}
