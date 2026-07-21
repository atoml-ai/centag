package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
)

// looksLikeResponsesBody reports whether body is Responses API shaped (has input,
// no usable messages) and must be rewritten before POSTing to /chat/completions.
func looksLikeResponsesBody(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return false
	}
	if _, hasInput := raw["input"]; !hasInput {
		return false
	}
	if msgs, ok := raw["messages"].([]interface{}); ok && len(msgs) > 0 {
		return false
	}
	return true
}

// convertResponsesBodyToChatCompletions rewrites OpenAI Responses JSON
// (input / instructions / max_output_tokens / flat tools) into a chat/completions body.
// Returns (original, false) when no conversion is needed.
func convertResponsesBodyToChatCompletions(body []byte) ([]byte, bool) {
	if !looksLikeResponsesBody(body) {
		return body, false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return body, false
	}

	messages := make([]map[string]interface{}, 0, 4)
	if instr, ok := raw["instructions"].(string); ok {
		if s := strings.TrimSpace(instr); s != "" {
			messages = append(messages, map[string]interface{}{
				"role":    "system",
				"content": s,
			})
		}
	}
	messages = append(messages, responsesInputToMessages(raw["input"])...)
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
	if tools, ok := raw["tools"]; ok {
		out["tools"] = normalizeResponsesTools(tools)
	}
	if tc, ok := raw["tool_choice"]; ok {
		out["tool_choice"] = normalizeResponsesToolChoice(tc)
	}
	if maxOut, ok := raw["max_output_tokens"]; ok {
		out["max_tokens"] = maxOut
	} else if maxTok, ok := raw["max_tokens"]; ok {
		out["max_tokens"] = maxTok
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return body, false
	}
	return encoded, true
}

// normalizeResponsesTools converts OpenCode/Responses flat tools
// ({type,name,description,parameters}) into Chat Completions nested form
// ({type:"function", function:{name,description,parameters}}).
func normalizeResponsesTools(tools interface{}) interface{} {
	arr, ok := tools.([]interface{})
	if !ok {
		return tools
	}
	out := make([]interface{}, 0, len(arr))
	for _, t := range arr {
		m, ok := t.(map[string]interface{})
		if !ok {
			out = append(out, t)
			continue
		}
		if fn, ok := m["function"].(map[string]interface{}); ok && fn != nil {
			// Already Chat-shaped.
			out = append(out, m)
			continue
		}
		typ, _ := m["type"].(string)
		typ = strings.TrimSpace(typ)
		// web_search / file_search 等 Responses 原生工具不能改写成 function，
		// 原样透传（上游 chat 若不支持会自行报错，避免静默错类型）。
		if typ != "" && typ != "function" {
			out = append(out, m)
			continue
		}
		name, _ := m["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			out = append(out, m)
			continue
		}
		if typ == "" {
			typ = "function"
		}
		fn := map[string]interface{}{"name": name}
		if d, ok := m["description"]; ok {
			fn["description"] = d
		}
		if p, ok := m["parameters"]; ok {
			fn["parameters"] = p
		} else if p, ok := m["input_schema"]; ok {
			fn["parameters"] = p
		}
		if s, ok := m["strict"]; ok {
			fn["strict"] = s
		}
		out = append(out, map[string]interface{}{
			"type":     typ,
			"function": fn,
		})
	}
	return out
}

// normalizeResponsesToolChoice maps Responses {type:"function", name} to
// Chat {type:"function", function:{name}}.
func normalizeResponsesToolChoice(tc interface{}) interface{} {
	m, ok := tc.(map[string]interface{})
	if !ok {
		return tc
	}
	if _, hasFn := m["function"]; hasFn {
		return tc
	}
	typ, _ := m["type"].(string)
	name, _ := m["name"].(string)
	name = strings.TrimSpace(name)
	if typ == "function" && name != "" {
		return map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": name,
			},
		}
	}
	return tc
}

func responsesInputToMessages(input interface{}) []map[string]interface{} {
	switch v := input.(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			return []map[string]interface{}{{"role": "user", "content": s}}
		}
	case []interface{}:
		msgs := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			typ, _ := m["type"].(string)
			typ = strings.TrimSpace(typ)
			switch typ {
			case "function_call":
				if msg := functionCallItemToChatMessage(m); msg != nil {
					msgs = append(msgs, msg)
				}
			case "function_call_output":
				if msg := functionCallOutputItemToChatMessage(m); msg != nil {
					msgs = append(msgs, msg)
				}
			default:
				// "message", "", or OpenCode items that only set role/content
				if msg := roleContentItemToChatMessage(m); msg != nil {
					msgs = append(msgs, msg)
				}
			}
		}
		return msgs
	}
	return nil
}

func functionCallItemToChatMessage(m map[string]interface{}) map[string]interface{} {
	callID := firstNonEmptyString(m, "call_id", "id")
	name, _ := m["name"].(string)
	name = strings.TrimSpace(name)
	if callID == "" || name == "" {
		return nil
	}
	args := anyToJSONString(m["arguments"])
	if args == "" {
		args = "{}"
	}
	return map[string]interface{}{
		"role":    "assistant",
		"content": nil,
		"tool_calls": []interface{}{
			map[string]interface{}{
				"id":   callID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      name,
					"arguments": args,
				},
			},
		},
	}
}

func functionCallOutputItemToChatMessage(m map[string]interface{}) map[string]interface{} {
	callID := firstNonEmptyString(m, "call_id")
	if callID == "" {
		return nil
	}
	return map[string]interface{}{
		"role":         "tool",
		"tool_call_id": callID,
		"content":      anyToJSONString(m["output"]),
	}
}

func roleContentItemToChatMessage(m map[string]interface{}) map[string]interface{} {
	role, _ := m["role"].(string)
	role = strings.TrimSpace(role)
	if role == "" {
		role = "user"
	}
	if role == "developer" {
		role = "system"
	}
	text := flexibleContentPlainText(m["content"])
	if text == "" {
		return nil
	}
	return map[string]interface{}{
		"role":    role,
		"content": text,
	}
}

func flexibleContentPlainText(content interface{}) string {
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
			typ, _ := pm["type"].(string)
			switch typ {
			case "input_text", "output_text", "text", "":
				if t, ok := pm["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		return b.String()
	default:
		return ""
	}
}

func firstNonEmptyString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			if t := strings.TrimSpace(s); t != "" {
				return t
			}
		}
	}
	return ""
}

func anyToJSONString(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(b)
	}
}

// chatCompletionExtract is the structured view of a buffered chat/completions
// response (SSE or JSON), used after Responses→Chat rewrite.
type chatCompletionExtract struct {
	Text         string
	ToolCalls    []ToolCall
	FinishReason string
}

// extractChatCompletionResult pulls assistant text + tool_calls from a buffered
// chat/completions SSE body or a non-stream JSON completion.
func extractChatCompletionResult(body []byte) chatCompletionExtract {
	trim := strings.TrimSpace(string(body))
	if trim == "" {
		return chatCompletionExtract{}
	}
	if strings.HasPrefix(trim, "data:") || strings.Contains(trim, "\ndata:") {
		return extractChatCompletionResultFromSSE(body)
	}
	return extractChatCompletionResultFromJSON(body)
}

// extractChatCompletionText pulls assistant text only (compat helper).
func extractChatCompletionText(body []byte) string {
	return extractChatCompletionResult(body).Text
}

func extractChatCompletionResultFromJSON(body []byte) chatCompletionExtract {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return chatCompletionExtract{}
	}
	choices, _ := raw["choices"].([]interface{})
	if len(choices) == 0 {
		return chatCompletionExtract{}
	}
	first, _ := choices[0].(map[string]interface{})
	if first == nil {
		return chatCompletionExtract{}
	}
	out := chatCompletionExtract{}
	if fr, ok := first["finish_reason"].(string); ok {
		out.FinishReason = strings.TrimSpace(fr)
	}
	if msg, ok := first["message"].(map[string]interface{}); ok && msg != nil {
		if c, ok := msg["content"].(string); ok {
			out.Text = c
		}
		out.ToolCalls = parseChatToolCalls(msg["tool_calls"])
	}
	if out.Text == "" {
		if delta, ok := first["delta"].(map[string]interface{}); ok {
			if c, ok := delta["content"].(string); ok {
				out.Text = c
			}
			if len(out.ToolCalls) == 0 {
				out.ToolCalls = parseChatToolCalls(delta["tool_calls"])
			}
		}
	}
	if out.Text == "" {
		if c, ok := first["text"].(string); ok {
			out.Text = c
		}
	}
	if out.FinishReason == "" && len(out.ToolCalls) > 0 {
		out.FinishReason = "tool_calls"
	}
	return out
}

func extractChatCompletionResultFromSSE(body []byte) chatCompletionExtract {
	var text strings.Builder
	byIndex := map[int]*pendingToolCall{}
	var order []int
	finishReason := ""

	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk map[string]interface{}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		choices, _ := chunk["choices"].([]interface{})
		if len(choices) == 0 {
			continue
		}
		first, _ := choices[0].(map[string]interface{})
		if first == nil {
			continue
		}
		if fr, ok := first["finish_reason"].(string); ok && strings.TrimSpace(fr) != "" {
			finishReason = strings.TrimSpace(fr)
		}
		if delta, ok := first["delta"].(map[string]interface{}); ok && delta != nil {
			if c, ok := delta["content"].(string); ok && c != "" {
				text.WriteString(c)
			}
			mergeStreamingToolCallDeltas(delta["tool_calls"], byIndex, &order)
		}
		if msg, ok := first["message"].(map[string]interface{}); ok && msg != nil {
			if c, ok := msg["content"].(string); ok && c != "" {
				text.WriteString(c)
			}
			for _, tc := range parseChatToolCalls(msg["tool_calls"]) {
				idx := len(order)
				order = append(order, idx)
				byIndex[idx] = &pendingToolCall{
					id:   tc.ID,
					name: tc.Function.Name,
					args: tc.Function.Arguments,
				}
			}
		}
	}

	toolCalls := make([]ToolCall, 0, len(order))
	for _, idx := range order {
		p := byIndex[idx]
		if p == nil || strings.TrimSpace(p.name) == "" {
			continue
		}
		id := strings.TrimSpace(p.id)
		if id == "" {
			id = fmt.Sprintf("call_%d", idx)
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:   id,
			Type: "function",
			Function: FunctionCall{
				Name:      p.name,
				Arguments: p.args,
			},
		})
	}
	if finishReason == "" && len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	return chatCompletionExtract{
		Text:         text.String(),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
	}
}

func mergeStreamingToolCallDeltas(raw interface{}, byIndex map[int]*pendingToolCall, order *[]int) {
	arr, ok := raw.([]interface{})
	if !ok {
		return
	}
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		idx := 0
		switch v := m["index"].(type) {
		case float64:
			idx = int(v)
		case int:
			idx = v
		}
		p, ok := byIndex[idx]
		if !ok {
			p = &pendingToolCall{}
			byIndex[idx] = p
			*order = append(*order, idx)
		}
		if id, ok := m["id"].(string); ok && strings.TrimSpace(id) != "" {
			p.id = strings.TrimSpace(id)
		}
		if fn, ok := m["function"].(map[string]interface{}); ok && fn != nil {
			if name, ok := fn["name"].(string); ok && name != "" {
				p.name += name
			}
			if args, ok := fn["arguments"].(string); ok {
				p.args += args
			}
		}
	}
}

// pendingToolCall accumulates streamed tool_call deltas by index.
type pendingToolCall struct {
	id, name, args string
}

func parseChatToolCalls(raw interface{}) []ToolCall {
	arr, ok := raw.([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		typ, _ := m["type"].(string)
		if strings.TrimSpace(typ) == "" {
			typ = "function"
		}
		fn, _ := m["function"].(map[string]interface{})
		name := ""
		args := ""
		if fn != nil {
			name, _ = fn["name"].(string)
			args = anyToJSONString(fn["arguments"])
			if s, ok := fn["arguments"].(string); ok {
				args = s
			}
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out = append(out, ToolCall{
			ID:   id,
			Type: typ,
			Function: FunctionCall{
				Name:      name,
				Arguments: args,
			},
		})
	}
	return out
}
