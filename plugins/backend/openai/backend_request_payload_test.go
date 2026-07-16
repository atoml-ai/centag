package openai

import (
	"testing"

	"centag/core/pkg/plugin"
)

func TestBuildOpenAIRequestPayload_PreservesRawToolFields(t *testing.T) {
	req := &plugin.ProxyRequest{
		Model: "new-model",
		RawBody: map[string]interface{}{
			"model":       "old-model",
			"stream":      false,
			"tool_choice": "required",
			"tools": []interface{}{
				map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name": "exec",
					},
				},
			},
			"messages": []interface{}{
				map[string]interface{}{
					"role":    "user",
					"content": "hi",
				},
			},
		},
	}

	payload := buildOpenAIRequestPayload(req, true)

	if got, ok := payload["model"].(string); !ok || got != "new-model" {
		t.Fatalf("payload model = %#v, want new-model", payload["model"])
	}
	if got, ok := payload["stream"].(bool); !ok || !got {
		t.Fatalf("payload stream = %#v, want true", payload["stream"])
	}
	if got, ok := payload["tool_choice"].(string); !ok || got != "required" {
		t.Fatalf("payload tool_choice = %#v, want required", payload["tool_choice"])
	}
	if _, exists := payload["tools"]; !exists {
		t.Fatalf("payload missing tools field")
	}
}

func TestConvertToOpenAIMessages_PreservesToolCallFields(t *testing.T) {
	in := []plugin.Message{
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []plugin.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: plugin.FunctionCall{
						Name:      "exec",
						Arguments: "{\"command\":\"pwd\"}",
					},
				},
			},
		},
		{
			Role:       "tool",
			Content:    "done",
			ToolCallID: "call_1",
		},
	}

	out := convertToOpenAIMessages(in)
	if len(out) != 2 {
		t.Fatalf("messages length = %d, want 2", len(out))
	}
	if len(out[0].ToolCalls) != 1 {
		t.Fatalf("assistant tool calls = %d, want 1", len(out[0].ToolCalls))
	}
	if out[0].ToolCalls[0].Function.Name != "exec" {
		t.Fatalf("assistant tool call name = %s, want exec", out[0].ToolCalls[0].Function.Name)
	}
	if out[1].ToolCallID != "call_1" {
		t.Fatalf("tool message tool_call_id = %s, want call_1", out[1].ToolCallID)
	}
}
