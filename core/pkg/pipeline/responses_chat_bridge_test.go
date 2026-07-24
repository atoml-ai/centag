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

func TestSanitizeChatCompletionsTools_RepairsFlatAndDropsHosted(t *testing.T) {
	in := `{
		"model":"glm-4-flash",
		"messages":[{"role":"user","content":"hi"}],
		"tools":[
			{"type":"web_search"},
			{"type":"function","name":"bash","description":"shell","parameters":{"type":"object"}},
			{"type":"function","function":{}}
		]
	}`
	out, ok := sanitizeChatCompletionsTools([]byte(in))
	if !ok {
		t.Fatal("expected sanitize change")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	tools := raw["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("tools=%v, want only bash (hosted dropped, empty function dropped)", tools)
	}
	fn := tools[0].(map[string]interface{})["function"].(map[string]interface{})
	if fn["name"] != "bash" {
		t.Fatalf("function=%v", fn)
	}
}

func TestSanitizeChatCompletionsTools_PreservesBytesWhenAlreadyNested(t *testing.T) {
	in := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"get_weather"}}],"tool_choice":"auto","stream":true}`
	out, ok := sanitizeChatCompletionsTools([]byte(in))
	if ok {
		t.Fatalf("already-nested tools must not rewrite body, got: %s", string(out))
	}
	if string(out) != in {
		t.Fatalf("body bytes changed:\n got:  %s\n want: %s", string(out), in)
	}
}

func TestNormalizeResponsesTools_DropsHostedTypes(t *testing.T) {
	tools := []interface{}{
		map[string]interface{}{"type": "web_search"},
		map[string]interface{}{
			"type":        "function",
			"name":        "bash",
			"description": "shell",
			"parameters":  map[string]interface{}{"type": "object"},
		},
		map[string]interface{}{
			"type":     "function",
			"name":     "read",
			"function": map[string]interface{}{}, // 空 function：需用顶层 name 回填
		},
	}
	out := normalizeResponsesTools(tools).([]interface{})
	if len(out) != 2 {
		t.Fatalf("len=%d want 2 (hosted dropped), out=%v", len(out), out)
	}
	fnTool := out[0].(map[string]interface{})
	fn := fnTool["function"].(map[string]interface{})
	if fn["name"] != "bash" {
		t.Fatalf("function tool not nested: %v", fnTool)
	}
	fn2 := out[1].(map[string]interface{})["function"].(map[string]interface{})
	if fn2["name"] != "read" {
		t.Fatalf("empty function should be repaired from top-level name: %v", out[1])
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

// 缺 call_id 的 function_call_output 应当回填最近未配对的 function_call ID，
// 避免上传到 /chat/completions 后报 "An assistant message with 'tool_calls'
// must be followed by tool messages"。
func TestConvertResponsesBodyToChatCompletions_BackfillsMissingOutputCallID(t *testing.T) {
	in := `{
		"model":"m",
		"input":[
			{"role":"user","content":"ls"},
			{"type":"function_call","call_id":"call_xyz","name":"bash","arguments":"{\"command\":\"ls\"}"},
			{"type":"function_call_output","output":"README.md\n"},
			{"role":"user","content":"继续"}
		]
	}`
	out, ok := convertResponsesBodyToChatCompletions([]byte(in))
	if !ok {
		t.Fatalf("expected conversion")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	msgs, _ := raw["messages"].([]interface{})
	if len(msgs) != 4 {
		t.Fatalf("messages len=%d, body=%s", len(msgs), string(out))
	}
	asst := msgs[1].(map[string]interface{})
	tcs, _ := asst["tool_calls"].([]interface{})
	if len(tcs) != 1 {
		t.Fatalf("assistant tool_calls=%v", asst["tool_calls"])
	}
	tc := tcs[0].(map[string]interface{})
	if tc["id"] != "call_xyz" {
		t.Fatalf("assistant tool_call id=%v", tc["id"])
	}
	toolMsg := msgs[2].(map[string]interface{})
	if toolMsg["role"] != "tool" {
		t.Fatalf("tool role=%v", toolMsg["role"])
	}
	if toolMsg["tool_call_id"] != "call_xyz" {
		t.Fatalf("backfilled tool_call_id=%v want call_xyz, body=%s", toolMsg["tool_call_id"], string(out))
	}
}

// assistant.tool_calls 缺配对的 tool 消息时，应整体清理 tool_calls，
// 若 content 为空则丢弃整条 assistant 消息，避免上传后被上游拒绝。
func TestConvertResponsesBodyToChatCompletions_DropsUnpairedAssistantToolCalls(t *testing.T) {
	in := `{
		"model":"m",
		"input":[
			{"role":"user","content":"ls"},
			{"type":"function_call","call_id":"orphan","name":"bash","arguments":"{\"command\":\"ls\"}"},
			{"role":"user","content":"不做"}
		]
	}`
	out, ok := convertResponsesBodyToChatCompletions([]byte(in))
	if !ok {
		t.Fatalf("expected conversion")
	}
	if strings.Contains(string(out), `"tool_calls"`) {
		t.Fatalf("unpaired tool_calls must be dropped, body=%s", string(out))
	}
	if strings.Contains(string(out), `"role":"tool"`) {
		t.Fatalf("no tool message should be emitted, body=%s", string(out))
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	msgs, _ := raw["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Fatalf("messages len=%d want 2 (orphan assistant dropped), body=%s", len(msgs), string(out))
	}
	for i, m := range msgs {
		role, _ := m.(map[string]interface{})["role"].(string)
		if role == "assistant" {
			_, hasTC := m.(map[string]interface{})["tool_calls"]
			if hasTC {
				t.Fatalf("msg[%d] still has tool_calls: %s", i, string(out))
			}
		}
	}
}

// 孤儿 tool 消息（无匹配 assistant.tool_calls）应当被丢掉。
func TestConvertResponsesBodyToChatCompletions_DropsOrphanToolMessage(t *testing.T) {
	in := `{
		"model":"m",
		"input":[
			{"role":"user","content":"hi"},
			{"type":"function_call_output","call_id":"missing","output":"done"},
			{"role":"user","content":"bye"}
		]
	}`
	out, ok := convertResponsesBodyToChatCompletions([]byte(in))
	if !ok {
		t.Fatalf("expected conversion")
	}
	if strings.Contains(string(out), `"role":"tool"`) {
		t.Fatalf("orphan tool message must be dropped, body=%s", string(out))
	}
	if strings.Contains(string(out), `"tool_calls"`) {
		t.Fatalf("no assistant.tool_calls should be emitted, body=%s", string(out))
	}
}

// function_call 缺 call_id 时应合成稳定 ID，且后续无 call_id 的
// function_call_output 能引用同一 ID 完成配对。
func TestConvertResponsesBodyToChatCompletions_SynthesizesMissingCallID(t *testing.T) {
	in := `{
		"model":"m",
		"input":[
			{"role":"user","content":"ls"},
			{"type":"function_call","name":"bash","arguments":"{\"command\":\"ls\"}"},
			{"type":"function_call_output","output":"result"},
			{"role":"user","content":"continue"}
		]
	}`
	out, ok := convertResponsesBodyToChatCompletions([]byte(in))
	if !ok {
		t.Fatalf("expected conversion")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	msgs, _ := raw["messages"].([]interface{})
	var asstCallID, toolCallID string
	for _, m := range msgs {
		mm, _ := m.(map[string]interface{})
		switch mm["role"] {
		case "assistant":
			tcs, _ := mm["tool_calls"].([]interface{})
			if len(tcs) > 0 {
				tc, _ := tcs[0].(map[string]interface{})
				asstCallID, _ = tc["id"].(string)
			}
		case "tool":
			toolCallID, _ = mm["tool_call_id"].(string)
		}
	}
	if asstCallID == "" {
		t.Fatalf("assistant.tool_calls missing (id not synthesized), body=%s", string(out))
	}
	if toolCallID == "" {
		t.Fatalf("tool message missing (back-fill failed), body=%s", string(out))
	}
	if asstCallID != toolCallID {
		t.Fatalf("paired IDs differ: assistant=%q tool=%q, body=%s", asstCallID, toolCallID, string(out))
	}
}
