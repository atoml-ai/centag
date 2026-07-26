package pipeline

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertAnthropicMessagesBodyToChatCompletions_SystemArray(t *testing.T) {
	in := `{
		"model": "claude-fable-5",
		"max_tokens": 1024,
		"stream": true,
		"system": [
			{"type": "text", "text": "You are OpenCode"},
			{"type": "text", "text": "Be concise", "cache_control": {"type": "ephemeral"}}
		],
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "你好"}]}
		]
	}`
	out, ok := convertAnthropicMessagesBodyToChatCompletions([]byte(in))
	if !ok {
		t.Fatal("expected conversion")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, hasSystem := raw["system"]; hasSystem {
		t.Fatal("chat body must not keep top-level system")
	}
	msgs, _ := raw["messages"].([]interface{})
	if len(msgs) < 2 {
		t.Fatalf("messages len=%d, want >=2 (system+user)", len(msgs))
	}
	sys, _ := msgs[0].(map[string]interface{})
	if sys["role"] != "system" {
		t.Fatalf("first role=%v, want system", sys["role"])
	}
	if got, _ := sys["content"].(string); !strings.Contains(got, "You are OpenCode") || !strings.Contains(got, "Be concise") {
		t.Fatalf("system content=%q", got)
	}
	user, _ := msgs[1].(map[string]interface{})
	if user["role"] != "user" || user["content"] != "你好" {
		t.Fatalf("user msg=%v", user)
	}
	if raw["model"] != "claude-fable-5" {
		t.Fatalf("model=%v", raw["model"])
	}
	if raw["stream"] != true {
		t.Fatalf("stream=%v", raw["stream"])
	}
	if raw["max_tokens"] != float64(1024) {
		t.Fatalf("max_tokens=%v", raw["max_tokens"])
	}
}

func TestConvertAnthropicMessagesBodyToChatCompletions_ToolsAndToolUse(t *testing.T) {
	in := `{
		"model": "claude-3-5-sonnet",
		"max_tokens": 512,
		"messages": [
			{"role": "user", "content": "weather?"},
			{"role": "assistant", "content": [
				{"type": "text", "text": "checking"},
				{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": {"city": "SF"}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_1", "content": "72F"}
			]}
		],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get weather",
				"input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}
			}
		],
		"tool_choice": {"type": "auto"}
	}`
	out, ok := convertAnthropicMessagesBodyToChatCompletions([]byte(in))
	if !ok {
		t.Fatal("expected conversion")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tools, _ := raw["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("tools len=%d", len(tools))
	}
	tool, _ := tools[0].(map[string]interface{})
	fn, _ := tool["function"].(map[string]interface{})
	if tool["type"] != "function" || fn["name"] != "get_weather" {
		t.Fatalf("tool=%v", tool)
	}
	if _, ok := fn["parameters"]; !ok {
		t.Fatal("expected parameters from input_schema")
	}
	if raw["tool_choice"] != "auto" {
		t.Fatalf("tool_choice=%v", raw["tool_choice"])
	}

	msgs, _ := raw["messages"].([]interface{})
	var foundAssistant, foundTool bool
	for _, m := range msgs {
		mm, _ := m.(map[string]interface{})
		switch mm["role"] {
		case "assistant":
			tcs, _ := mm["tool_calls"].([]interface{})
			if len(tcs) == 1 {
				foundAssistant = true
			}
		case "tool":
			if mm["tool_call_id"] == "toolu_1" && mm["content"] == "72F" {
				foundTool = true
			}
		}
	}
	if !foundAssistant || !foundTool {
		t.Fatalf("assistant/tool conversion missing: assistant=%v tool=%v msgs=%v", foundAssistant, foundTool, msgs)
	}
}

func TestConvertAnthropicMessagesBodyToChatCompletions_NoOpForChat(t *testing.T) {
	in := `{"model":"gpt","messages":[{"role":"user","content":"hi"}],"stream":true}`
	_, ok := convertAnthropicMessagesBodyToChatCompletions([]byte(in))
	if ok {
		t.Fatal("chat body should not convert")
	}
}

func TestApplyChatCompletionsRequestBridges_AnthropicPath(t *testing.T) {
	in := `{
		"model": "m",
		"max_tokens": 10,
		"system": "sys",
		"messages": [{"role":"user","content":"hi"}]
	}`
	out, bridge, anthropic := applyChatCompletionsRequestBridges([]byte(in), "/v1/messages")
	if !bridge || !anthropic {
		t.Fatalf("bridge=%v anthropic=%v", bridge, anthropic)
	}
	if looksLikeAnthropicMessagesBody(out) {
		t.Fatal("converted body should no longer look anthropic")
	}
}

func TestIsMessagesAPIPath(t *testing.T) {
	if !isMessagesAPIPath("/v1/messages") || !isMessagesAPIPath("/zen/v1/messages/") {
		t.Fatal("expected messages path match")
	}
	if isMessagesAPIPath("/v1/chat/completions") || isMessagesAPIPath("/v1/responses") {
		t.Fatal("unexpected messages path match")
	}
}
