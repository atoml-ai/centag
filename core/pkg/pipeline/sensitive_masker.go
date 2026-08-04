package pipeline

import (
	"regexp"
	"strings"
)

// SensitiveDataMasker 敏感数据脱敏器
type SensitiveDataMasker struct {
	rules []MaskingRule
}

// MaskingRule 脱敏规则
type MaskingRule struct {
	Name        string
	Pattern     *regexp.Regexp
	ReplaceFunc func(match string) string
}

// NewSensitiveDataMasker 创建脱敏器
func NewSensitiveDataMasker() *SensitiveDataMasker {
	return &SensitiveDataMasker{
		rules: []MaskingRule{
			{
				Name:    "API Key",
				Pattern: regexp.MustCompile(`(sk-|pk-)[a-zA-Z0-9]{20,}`),
				ReplaceFunc: func(match string) string {
					return maskString(match, "*")
				},
			},
			{
				Name:    "Bearer Token",
				Pattern: regexp.MustCompile(`Bearer\s+[a-zA-Z0-9_\-\.]{20,}`),
				ReplaceFunc: func(match string) string {
					return "Bearer " + maskString(strings.TrimPrefix(match, "Bearer "), "*")
				},
			},
			{
				Name:    "Secret Reference",
				Pattern: regexp.MustCompile(`(?i)(secrets?_ref|api[\s_-]?key|token|password|authorization)(["'\s:=]+)([^"',\s}]{6,})`),
				ReplaceFunc: func(match string) string {
					parts := regexp.MustCompile(`(["'\s:=]+)`).Split(match, 2)
					if len(parts) == 0 {
						return "[REDACTED]"
					}
					key := parts[0]
					sep := strings.TrimPrefix(match, key)
					return key + sep[:len(sep)-len(strings.TrimLeft(sep, "\"' :=\t"))] + "[REDACTED]"
				},
			},
			{
				Name:    "Phone Number",
				Pattern: regexp.MustCompile(`1[3-9]\d{9}`),
				ReplaceFunc: func(match string) string {
					return maskString(match, "*")
				},
			},
			{
				Name:    "Email",
				Pattern: regexp.MustCompile(`\S+@\S+\.\S+`),
				ReplaceFunc: func(match string) string {
					parts := strings.Split(match, "@")
					if len(parts) == 2 {
						return maskString(parts[0], "*") + "@" + parts[1]
					}
					return maskString(match, "*")
				},
			},
			{
				Name:    "ID Card",
				Pattern: regexp.MustCompile(`\d{17}[\dXx]|\d{15}`),
				ReplaceFunc: func(match string) string {
					return maskString(match, "*")
				},
			},
		},
	}
}

// Mask 对文本进行脱敏
func (m *SensitiveDataMasker) Mask(text string) string {
	for _, rule := range m.rules {
		text = rule.Pattern.ReplaceAllStringFunc(text, rule.ReplaceFunc)
	}
	return text
}

// MaskJSON 对 JSON 字符串进行脱敏（保留结构）
func (m *SensitiveDataMasker) MaskJSON(jsonStr string) string {
	// 简单实现：直接对字符串脱敏
	// 更高级的实现可以解析 JSON，只脱敏 value
	return m.Mask(jsonStr)
}

// GlobalMasker 全局脱敏器实例
var GlobalMasker = NewSensitiveDataMasker()

// MaskSensitiveData 全局脱敏函数
func MaskSensitiveData(text string) string {
	return GlobalMasker.Mask(text)
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
