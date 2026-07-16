package proxy

import "strings"

// ExtractMessageText 从 OpenAI 消息 content 字段提取纯文本。
// 支持 string、{"type":"text","text":"..."} 以及 [{"type":"text","text":"..."}]。
func ExtractMessageText(content interface{}) (string, bool) {
	switch v := content.(type) {
	case string:
		return v, true
	case map[string]interface{}:
		return textFromPart(v)
	case []interface{}:
		var parts []string
		for _, item := range v {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := textFromPart(part); ok {
				parts = append(parts, text)
			}
		}
		if len(parts) == 0 {
			return "", false
		}
		return strings.Join(parts, ""), true
	default:
		return "", false
	}
}

func textFromPart(part map[string]interface{}) (string, bool) {
	typ, _ := part["type"].(string)
	if typ != "" && typ != "text" && typ != "input_text" {
		return "", false
	}
	if text, ok := part["text"].(string); ok {
		return text, true
	}
	if text, ok := part["input_text"].(string); ok {
		return text, true
	}
	return "", false
}

// SetMessageText 写回消息 content；快捷码剥离后统一为 string，便于下游解析。
func SetMessageText(msg map[string]interface{}, text string) {
	if msg != nil {
		msg["content"] = text
	}
}