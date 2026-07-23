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
		normalized := normalizeResponsesTools(tools)
		if arr, ok := normalized.([]interface{}); ok && len(arr) == 0 {
			// 全部为托管工具被丢弃时不要带空 tools，避免上游对 tools[0] 校验失败
		} else {
			out["tools"] = normalized
			if tc, ok := raw["tool_choice"]; ok {
				out["tool_choice"] = normalizeResponsesToolChoice(tc)
			}
		}
	} else if tc, ok := raw["tool_choice"]; ok {
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
//
// 非 function 托管工具（web_search / file_search 等）在 /chat/completions 上无效，
// 原样透传会导致智谱等报「tools[0].function 不能为空」，因此直接丢弃。
func normalizeResponsesTools(tools interface{}) interface{} {
	arr, ok := tools.([]interface{})
	if !ok {
		return tools
	}
	out := make([]interface{}, 0, len(arr))
	for _, t := range arr {
		m, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		typ, _ := m["type"].(string)
		typ = strings.TrimSpace(typ)
		if typ != "" && typ != "function" {
			continue
		}

		fn, _ := m["function"].(map[string]interface{})
		name := ""
		if fn != nil {
			name = firstNonEmptyString(fn, "name")
		}
		if name == "" {
			name = firstNonEmptyString(m, "name")
		}
		if name == "" {
			continue
		}
		if typ == "" {
			typ = "function"
		}

		normalizedFn := map[string]interface{}{"name": name}
		if fn != nil {
			for k, v := range fn {
				if k == "name" {
					continue
				}
				if v != nil && fmt.Sprint(v) != "" {
					normalizedFn[k] = v
				}
			}
		}
		if _, ok := normalizedFn["description"]; !ok {
			if d, ok := m["description"]; ok {
				normalizedFn["description"] = d
			}
		}
		if _, ok := normalizedFn["parameters"]; !ok {
			if p, ok := m["parameters"]; ok {
				normalizedFn["parameters"] = p
			} else if p, ok := m["input_schema"]; ok {
				normalizedFn["parameters"] = p
			} else if p, ok := m["inputSchema"]; ok {
				normalizedFn["parameters"] = p
			}
		}
		if _, ok := normalizedFn["strict"]; !ok {
			if s, ok := m["strict"]; ok {
				normalizedFn["strict"] = s
			}
		}
		out = append(out, map[string]interface{}{
			"type":     typ,
			"function": normalizedFn,
		})
	}
	return out
}

// sanitizeChatCompletionsTools 清洗已是 chat/completions 形态的 body 中的 tools，
// 把 flat/hosted 工具改成智谱等上游可接受的 nested function 形态。
func sanitizeChatCompletionsTools(body []byte) ([]byte, bool) {
	if len(body) == 0 {
		return body, false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return body, false
	}
	tools, hasTools := raw["tools"]
	if !hasTools {
		return body, false
	}
	normalized := normalizeResponsesTools(tools)
	if arr, ok := normalized.([]interface{}); ok && len(arr) == 0 {
		delete(raw, "tools")
		delete(raw, "tool_choice")
	} else {
		raw["tools"] = normalized
		if tc, ok := raw["tool_choice"]; ok {
			raw["tool_choice"] = normalizeResponsesToolChoice(tc)
		}
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return body, false
	}
	if string(encoded) == string(body) {
		return body, false
	}
	return encoded, true
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
		// pendingCalls 记录 function_call 已发出但尚未被 function_call_output 配对的 call_id，
		// 用于给缺 call_id 的 function_call_output 回填配对 ID，避免 OpenAI 报
		// "An assistant message with 'tool_calls' must be followed by tool messages"。
		var pendingCalls []string
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			typ, _ := m["type"].(string)
			typ = strings.TrimSpace(typ)
			switch typ {
			case "function_call":
				msg, callID := functionCallItemToChatMessage(m)
				if msg == nil {
					continue
				}
				msgs = append(msgs, msg)
				if callID != "" && !sliceContainsString(pendingCalls, callID) {
					pendingCalls = append(pendingCalls, callID)
				}
			case "function_call_output":
				msg := functionCallOutputItemToChatMessage(m, pendingCalls)
				if msg == nil {
					continue
				}
				msgs = append(msgs, msg)
				if id, _ := msg["tool_call_id"].(string); id != "" {
					pendingCalls = sliceRemoveString(pendingCalls, id)
				}
			default:
				// "message", "", or OpenCode items that only set role/content
				if msg := roleContentItemToChatMessage(m); msg != nil {
					msgs = append(msgs, msg)
				}
			}
		}
		return enforceToolCallPairing(msgs)
	}
	return nil
}

// enforceToolCallPairing 强制 assistant.tool_calls 与 tool 角色消息严格配对，
// 避免上游 OpenAI 报 "An assistant message with 'tool_calls' must be followed
// by tool messages responding to each 'tool_call_id'"。规则：
//   - 收集所有 tool 消息的 tool_call_id（setToolIDs）。
//   - 遍历 assistant 消息：若其 tool_calls 中某 id 不在 setToolIDs 里，则移除该 tool_call；
//     若某个 tool 消息的 tool_call_id 不在任何 assistant.tool_calls 里，则移除该 tool 消息。
//   - 若 assistant 消息的 tool_calls 被清空且 content 为空/nil，整条 assistant 消息删除。
func enforceToolCallPairing(msgs []map[string]interface{}) []map[string]interface{} {
	if len(msgs) == 0 {
		return msgs
	}
	toolIDSet := make(map[string]struct{})
	for _, m := range msgs {
		if m["role"] != "tool" {
			continue
		}
		id, _ := m["tool_call_id"].(string)
		if id == "" {
			continue
		}
		toolIDSet[id] = struct{}{}
	}

	assistantCallIDSet := make(map[string]struct{})
	for _, m := range msgs {
		if m["role"] != "assistant" {
			continue
		}
		tcs, _ := m["tool_calls"].([]interface{})
		if len(tcs) == 0 {
			continue
		}
		for _, tc := range tcs {
			tm, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}
			if id, _ := tm["id"].(string); id != "" {
				assistantCallIDSet[id] = struct{}{}
			}
		}
	}

	out := make([]map[string]interface{}, 0, len(msgs))
	for _, m := range msgs {
		role, _ := m["role"].(string)
		switch role {
		case "assistant":
			tcs, _ := m["tool_calls"].([]interface{})
			if len(tcs) == 0 {
				out = append(out, m)
				continue
			}
			filtered := make([]interface{}, 0, len(tcs))
			for _, tc := range tcs {
				tm, ok := tc.(map[string]interface{})
				if !ok {
					continue
				}
				id, _ := tm["id"].(string)
				if id == "" {
					continue
				}
				if _, ok := toolIDSet[id]; ok {
					filtered = append(filtered, tc)
				}
			}
			if len(filtered) == 0 {
				// 没有任何 tool_call 被配对，且 content 为空/nil → 丢弃整条 assistant 消息
				content, hasContent := m["content"]
				if !hasContent || content == nil || content == "" {
					continue
				}
				// 有正文，保留为普通文本 assistant 消息（去掉 tool_calls 字段）
				delete(m, "tool_calls")
				out = append(out, m)
				continue
			}
			m["tool_calls"] = filtered
			out = append(out, m)
		case "tool":
			id, _ := m["tool_call_id"].(string)
			if id == "" {
				// 没有配对 ID 的 tool 消息直接丢弃，避免上游报 "tool message following tool_calls" 不匹配
				continue
			}
			if _, ok := assistantCallIDSet[id]; !ok {
				continue
			}
			out = append(out, m)
		default:
			out = append(out, m)
		}
	}
	return out
}

func sliceContainsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func sliceRemoveString(s []string, v string) []string {
	for i, x := range s {
		if x == v {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// functionCallItemToChatMessage 将 Responses 的 function_call 项转换为 Chat
// assistant.tool_calls 消息。返回的 callID 是实际写入消息的 ID（缺失时会合成
// 一个稳定 ID），供调用方记录到 pendingCalls 用于回填给后续无 call_id 的
// function_call_output，避免上游报 tool_calls/tool 配对失败。
func functionCallItemToChatMessage(m map[string]interface{}) (map[string]interface{}, string) {
	callID := firstNonEmptyString(m, "call_id", "id")
	name, _ := m["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		// 没有 name 时无法构造 tool_call，直接放弃（不会留下半截 assistant.tool_calls）
		return nil, ""
	}
	if callID == "" {
		// 缺失 call_id 时生成稳定 ID，保证后续 function_call_output 能引用。
		callID = fmt.Sprintf("call_%d_synthetic", timeHash(m))
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
	}, callID
}

// functionCallOutputItemToChatMessage 将 Responses 的 function_call_output 项
// 转换为 Chat tool 角色消息。call_id 缺失时从 pendingCalls 末尾取最近一个未配对
// 的 function_call 的 ID 进行回填；若 pendingCalls 为空则返回 nil（交给
// enforceToolCallPairing 兜底丢弃）。
func functionCallOutputItemToChatMessage(m map[string]interface{}, pendingCalls []string) map[string]interface{} {
	callID := firstNonEmptyString(m, "call_id")
	if callID == "" && len(pendingCalls) > 0 {
		callID = pendingCalls[len(pendingCalls)-1]
	}
	if callID == "" {
		return nil
	}
	return map[string]interface{}{
		"role":         "tool",
		"tool_call_id": callID,
		"content":      anyToJSONString(m["output"]),
	}
}

// timeHash 返回一个对 map 内容稳定的 int64，用于在缺 call_id 时合成可复现的 ID。
func timeHash(m map[string]interface{}) int64 {
	var h int64 = 1469598103934665603
	add := func(b []byte) {
		for _, c := range b {
			h ^= int64(c)
			h *= 1099511628211
		}
	}
	if id, ok := m["id"].(string); ok {
		add([]byte(id))
	}
	if name, ok := m["name"].(string); ok {
		add([]byte(name))
	}
	if args, ok := m["arguments"]; ok {
		add([]byte(anyToJSONString(args)))
	}
	if h < 0 {
		h = -h
	}
	return h
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
	Reasoning    string
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
		if r, ok := msg["reasoning_content"].(string); ok {
			out.Reasoning = r
		}
		out.ToolCalls = parseChatToolCalls(msg["tool_calls"])
	}
	if out.Text == "" {
		if delta, ok := first["delta"].(map[string]interface{}); ok {
			if c, ok := delta["content"].(string); ok {
				out.Text = c
			}
			if out.Reasoning == "" {
				if r, ok := delta["reasoning_content"].(string); ok {
					out.Reasoning = r
				}
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
	var reasoning strings.Builder
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
			if r, ok := delta["reasoning_content"].(string); ok && r != "" {
				reasoning.WriteString(r)
			}
			mergeStreamingToolCallDeltas(delta["tool_calls"], byIndex, &order)
		}
		if msg, ok := first["message"].(map[string]interface{}); ok && msg != nil {
			if c, ok := msg["content"].(string); ok && c != "" {
				text.WriteString(c)
			}
			if r, ok := msg["reasoning_content"].(string); ok && r != "" {
				reasoning.WriteString(r)
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
		Reasoning:    reasoning.String(),
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
