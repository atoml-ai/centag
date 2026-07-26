package pipeline

import (
	"encoding/json"
	"strings"
)

// looksLikeAnthropicMessagesBody reports whether body is Anthropic Messages API shaped
// and must be rewritten before POSTing to /chat/completions.
func looksLikeAnthropicMessagesBody(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	if looksLikeResponsesBody(body) {
		return false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return false
	}
	if _, hasSystem := raw["system"]; hasSystem {
		return true
	}
	msgs, ok := raw["messages"].([]interface{})
	if !ok || len(msgs) == 0 {
		return false
	}
	for _, item := range msgs {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if contentLooksAnthropicBlocks(m["content"]) {
			return true
		}
	}
	if tools, ok := raw["tools"].([]interface{}); ok {
		for _, t := range tools {
			tm, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			if _, hasSchema := tm["input_schema"]; hasSchema {
				if _, hasFn := tm["function"]; !hasFn {
					return true
				}
			}
		}
	}
	return false
}

func contentLooksAnthropicBlocks(content interface{}) bool {
	arr, ok := content.([]interface{})
	if !ok {
		return false
	}
	for _, part := range arr {
		pm, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		switch strings.TrimSpace(stringMeta(pm, "type")) {
		case "tool_use", "tool_result", "image", "thinking":
			return true
		}
	}
	return false
}

// convertAnthropicMessagesBodyToChatCompletions rewrites Anthropic Messages JSON
// (top-level system / content blocks / input_schema tools) into a chat/completions body.
// Returns (original, false) when no conversion is needed.
func convertAnthropicMessagesBodyToChatCompletions(body []byte) ([]byte, bool) {
	if !looksLikeAnthropicMessagesBody(body) {
		return body, false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return body, false
	}

	messages := make([]map[string]interface{}, 0, 8)
	if sys := anthropicSystemToPlainText(raw["system"]); sys != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": sys,
		})
	}
	messages = append(messages, anthropicMessagesToChatMessages(raw["messages"])...)
	if len(messages) == 0 {
		return body, false
	}

	out := map[string]interface{}{
		"messages": messages,
	}
	if model, ok := raw["model"]; ok {
		out["model"] = model
	}
	for _, k := range []string{
		"stream", "temperature", "top_p", "user",
		"presence_penalty", "frequency_penalty",
		"stop", "n", "response_format",
	} {
		if v, ok := raw[k]; ok {
			out[k] = v
		}
	}
	if maxTok, ok := raw["max_tokens"]; ok {
		out["max_tokens"] = maxTok
	}
	if tools, ok := raw["tools"]; ok {
		normalized := normalizeResponsesTools(tools)
		if arr, ok := normalized.([]interface{}); ok && len(arr) == 0 {
			// drop empty tools
		} else {
			out["tools"] = normalized
			if tc, ok := raw["tool_choice"]; ok {
				out["tool_choice"] = normalizeAnthropicToolChoice(tc)
			}
		}
	} else if tc, ok := raw["tool_choice"]; ok {
		out["tool_choice"] = normalizeAnthropicToolChoice(tc)
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return body, false
	}
	return encoded, true
}

func anthropicSystemToPlainText(system interface{}) string {
	switch s := system.(type) {
	case string:
		return s
	case []interface{}:
		parts := make([]string, 0, len(s))
		for _, item := range s {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			typ := strings.TrimSpace(stringMeta(m, "type"))
			if typ != "" && typ != "text" {
				continue
			}
			if t, ok := m["text"].(string); ok && t != "" {
				parts = append(parts, t)
			}
		}
		return strings.Join(parts, "\n\n")
	default:
		return ""
	}
}

func anthropicMessagesToChatMessages(messages interface{}) []map[string]interface{} {
	arr, ok := messages.([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role := strings.TrimSpace(stringMeta(m, "role"))
		if role == "" {
			role = "user"
		}
		content := m["content"]

		switch c := content.(type) {
		case string:
			out = append(out, map[string]interface{}{
				"role":    role,
				"content": c,
			})
		case []interface{}:
			out = append(out, anthropicContentBlocksToChatMessages(role, c)...)
		default:
			if content == nil {
				out = append(out, map[string]interface{}{
					"role":    role,
					"content": "",
				})
			}
		}
	}
	return out
}

func anthropicContentBlocksToChatMessages(role string, blocks []interface{}) []map[string]interface{} {
	var textParts []string
	var toolCalls []interface{}
	var toolResults []map[string]interface{}

	for _, part := range blocks {
		pm, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		switch strings.TrimSpace(stringMeta(pm, "type")) {
		case "text", "":
			if t, ok := pm["text"].(string); ok && t != "" {
				textParts = append(textParts, t)
			}
		case "tool_use":
			id := firstNonEmptyString(pm, "id")
			name := firstNonEmptyString(pm, "name")
			if name == "" {
				continue
			}
			if id == "" {
				id = "toolu_synthetic"
			}
			args := anyToJSONString(pm["input"])
			if args == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   id,
				"type": "function",
				"function": map[string]interface{}{
					"name":      name,
					"arguments": args,
				},
			})
		case "tool_result":
			callID := firstNonEmptyString(pm, "tool_use_id")
			if callID == "" {
				continue
			}
			toolResults = append(toolResults, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": callID,
				"content":      anthropicToolResultToPlainText(pm["content"]),
			})
		case "image":
			textParts = append(textParts, "[image]")
		case "thinking":
			// skip thinking blocks for upstream chat
		}
	}

	out := make([]map[string]interface{}, 0, 1+len(toolResults))
	if len(toolResults) > 0 && role == "user" {
		out = append(out, toolResults...)
		if len(textParts) > 0 {
			out = append(out, map[string]interface{}{
				"role":    "user",
				"content": strings.Join(textParts, ""),
			})
		}
		return out
	}

	msg := map[string]interface{}{"role": role}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
		if len(textParts) > 0 {
			msg["content"] = strings.Join(textParts, "")
		} else {
			msg["content"] = nil
		}
	} else {
		msg["content"] = strings.Join(textParts, "")
	}
	out = append(out, msg)
	return out
}

func anthropicToolResultToPlainText(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var b strings.Builder
		for _, part := range c {
			pm, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			typ := strings.TrimSpace(stringMeta(pm, "type"))
			if typ != "" && typ != "text" {
				continue
			}
			if t, ok := pm["text"].(string); ok {
				b.WriteString(t)
			}
		}
		return b.String()
	default:
		return anyToJSONString(content)
	}
}

// normalizeAnthropicToolChoice maps Anthropic tool_choice to Chat Completions form.
func normalizeAnthropicToolChoice(tc interface{}) interface{} {
	if s, ok := tc.(string); ok {
		return s
	}
	m, ok := tc.(map[string]interface{})
	if !ok {
		return tc
	}
	typ := strings.TrimSpace(stringMeta(m, "type"))
	switch typ {
	case "auto":
		return "auto"
	case "any":
		return "required"
	case "none":
		return "none"
	case "tool", "function":
		name := firstNonEmptyString(m, "name")
		if name == "" {
			if fn, ok := m["function"].(map[string]interface{}); ok {
				name = firstNonEmptyString(fn, "name")
			}
		}
		if name != "" {
			return map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name": name,
				},
			}
		}
	}
	return tc
}

// isMessagesAPIPath 判断请求路径是否为 Anthropic Messages API（/v1/messages 等）。
func isMessagesAPIPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	path = strings.TrimRight(path, "/")
	return strings.HasSuffix(path, "/messages")
}

// applyChatCompletionsRequestBridges converts Responses / Anthropic Messages bodies
// into chat/completions when the outbound target is /chat/completions.
func applyChatCompletionsRequestBridges(body []byte, requestPath string) (rewritten []byte, bridgeToChat bool, anthropicBridge bool) {
	if rewritten, ok := convertResponsesBodyToChatCompletions(body); ok {
		return rewritten, true, false
	}
	if rewritten, ok := convertAnthropicMessagesBodyToChatCompletions(body); ok {
		return rewritten, true, true
	}
	if isResponsesAPIPath(requestPath) {
		return body, true, false
	}
	if isMessagesAPIPath(requestPath) {
		return body, true, true
	}
	return body, false, false
}
