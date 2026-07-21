package pipeline

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertResponsesBodyToChatCompletions_OpenCodeShape(t *testing.T) {
	in := `{
		"model":"gpt-5.6-luna",
		"stream":true,
		"max_output_tokens":32000,
		"input":[
			{"role":"system","content":"You are OpenCode"},
			{"role":"user","content":"你使用的是什么大模型"}
		]
	}`
	out, ok := convertResponsesBodyToChatCompletions([]byte(in))
	if !ok {
		t.Fatal("expected conversion")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	if _, has := raw["input"]; has {
		t.Fatal("input must be removed")
	}
	msgs, _ := raw["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Fatalf("messages len=%d want 2, body=%s", len(msgs), string(out))
	}
	if raw["max_tokens"] == nil {
		t.Fatal("max_output_tokens should map to max_tokens")
	}
	if stream, _ := raw["stream"].(bool); !stream {
		t.Fatal("stream must be preserved")
	}
	u := msgs[1].(map[string]interface{})
	if u["content"] != "你使用的是什么大模型" {
		t.Fatalf("user content = %v", u["content"])
	}
}

func TestConvertResponsesBodyToChatCompletions_PartsContent(t *testing.T) {
	in := `{"model":"m","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`
	out, ok := convertResponsesBodyToChatCompletions([]byte(in))
	if !ok {
		t.Fatal("expected conversion")
	}
	if !strings.Contains(string(out), `"hi"`) {
		t.Fatalf("missing hi: %s", string(out))
	}
}

func TestConvertResponsesBodyToChatCompletions_SkipChatBody(t *testing.T) {
	in := `{"model":"m","messages":[{"role":"user","content":"hi"}]}`
	_, ok := convertResponsesBodyToChatCompletions([]byte(in))
	if ok {
		t.Fatal("chat body should not convert")
	}
}

func TestConvertResponsesBodyToChatCompletions_ToolsAndFunctionRoundTrip(t *testing.T) {
	in := `{
		"model":"gpt-5.6-luna",
		"stream":true,
		"tool_choice":{"type":"function","name":"bash"},
		"tools":[{
			"type":"function",
			"name":"bash",
			"description":"Run a shell command",
			"parameters":{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}
		}],
		"input":[
			{"role":"user","content":"列出目录"},
			{"type":"function_call","call_id":"call_abc","name":"bash","arguments":"{\"command\":\"ls\"}"},
			{"type":"function_call_output","call_id":"call_abc","output":"README.md\n"},
			{"role":"user","content":"继续"}
		]
	}`
	out, ok := convertResponsesBodyToChatCompletions([]byte(in))
	if !ok {
		t.Fatal("expected conversion")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}

	tools, _ := raw["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("tools len=%d, body=%s", len(tools), string(out))
	}
	tool0 := tools[0].(map[string]interface{})
	fn := tool0["function"].(map[string]interface{})
	if fn["name"] != "bash" {
		t.Fatalf("tool function.name=%v", fn["name"])
	}
	if _, hasTopName := tool0["name"]; hasTopName {
		t.Fatal("flat name should be nested under function")
	}

	tc := raw["tool_choice"].(map[string]interface{})
	tcFn := tc["function"].(map[string]interface{})
	if tcFn["name"] != "bash" {
		t.Fatalf("tool_choice.function.name=%v", tcFn["name"])
	}

	msgs, _ := raw["messages"].([]interface{})
	if len(msgs) != 4 {
		t.Fatalf("messages len=%d want 4, body=%s", len(msgs), string(out))
	}
	asst := msgs[1].(map[string]interface{})
	if asst["role"] != "assistant" {
		t.Fatalf("msg1 role=%v", asst["role"])
	}
	tcs, _ := asst["tool_calls"].([]interface{})
	if len(tcs) != 1 {
		t.Fatalf("assistant tool_calls=%v", asst["tool_calls"])
	}
	toolMsg := msgs[2].(map[string]interface{})
	if toolMsg["role"] != "tool" || toolMsg["tool_call_id"] != "call_abc" {
		t.Fatalf("tool message=%v", toolMsg)
	}
	if toolMsg["content"] != "README.md\n" {
		t.Fatalf("tool content=%v", toolMsg["content"])
	}
}

func TestNormalizeResponsesTools_SkipsHostedTypes(t *testing.T) {
	tools := []interface{}{
		map[string]interface{}{"type": "web_search"},
		map[string]interface{}{
			"type":        "function",
			"name":        "bash",
			"description": "shell",
			"parameters":   map[string]interface{}{"type": "object"},
		},
	}
	out := normalizeResponsesTools(tools).([]interface{})
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	hosted := out[0].(map[string]interface{})
	if hosted["type"] != "web_search" {
		t.Fatalf("hosted rewritten: %v", hosted)
	}
	fnTool := out[1].(map[string]interface{})
	if _, ok := fnTool["function"].(map[string]interface{}); !ok {
		t.Fatalf("function tool not nested: %v", fnTool)
	}
}

func TestExtractChatCompletionTextFromSSE(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n" +
		"data: [DONE]\n\n"
	got := extractChatCompletionText([]byte(sse))
	if got != "你好" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractChatCompletionResult_ToolCallsFromSSE(t *testing.T) {
	sse := "" +
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"bash\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"command\\\":\\\"ls\\\"}\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: [DONE]\n\n"
	got := extractChatCompletionResult([]byte(sse))
	if got.FinishReason != "tool_calls" {
		t.Fatalf("finish=%q", got.FinishReason)
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("tool_calls=%v", got.ToolCalls)
	}
	tc := got.ToolCalls[0]
	if tc.ID != "call_1" || tc.Function.Name != "bash" {
		t.Fatalf("tc=%+v", tc)
	}
	if tc.Function.Arguments != `{"command":"ls"}` {
		t.Fatalf("args=%q", tc.Function.Arguments)
	}
}
