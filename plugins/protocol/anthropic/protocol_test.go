package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	"centag/core/pkg/plugin"
)

func TestFormatStreamChunk_TextContent(t *testing.T) {
	p := &Protocol{}

	// 第一个 chunk（带内容）
	chunk1 := &plugin.StreamChunk{
		Content: "Hello",
		Done:    false,
	}
	result1 := p.FormatStreamChunk("claude-3-5-sonnet-20241022", chunk1, 0)
	if result1 == "" {
		t.Fatal("expected non-empty result for first chunk")
	}
	if !strings.Contains(result1, "event: message_start") {
		t.Error("first chunk should contain message_start event")
	}
	if !strings.Contains(result1, "event: content_block_start") {
		t.Error("first chunk should contain content_block_start event")
	}
	if !strings.Contains(result1, "event: content_block_delta") {
		t.Error("first chunk should contain content_block_delta event")
	}
	if !strings.Contains(result1, `"text_delta"`) {
		t.Error("chunk should contain text_delta")
	}
	if !strings.Contains(result1, `"Hello"`) {
		t.Error("chunk should contain the text content")
	}

	// 中间 chunk
	chunk2 := &plugin.StreamChunk{
		Content: " world",
		Done:    false,
	}
	result2 := p.FormatStreamChunk("claude-3-5-sonnet-20241022", chunk2, 1)
	if result2 == "" {
		t.Fatal("expected non-empty result for middle chunk")
	}
	if strings.Contains(result2, "message_start") {
		t.Error("middle chunk should not contain message_start")
	}
	if !strings.Contains(result2, "content_block_delta") {
		t.Error("middle chunk should contain content_block_delta")
	}

	// 结束 chunk
	chunk3 := &plugin.StreamChunk{
		Content:      "",
		Done:         true,
		FinishReason: "stop",
		TokensUsed:   100,
	}
	result3 := p.FormatStreamChunk("claude-3-5-sonnet-20241022", chunk3, 2)
	if result3 == "" {
		t.Fatal("expected non-empty result for done chunk")
	}
	if !strings.Contains(result3, "content_block_stop") {
		t.Error("done chunk should contain content_block_stop")
	}
	if !strings.Contains(result3, "message_delta") {
		t.Error("done chunk should contain message_delta")
	}
	if !strings.Contains(result3, "message_stop") {
		t.Error("done chunk should contain message_stop")
	}

	// 验证 stop_reason 通过 JSON 解析
	if !containsJSONField(result3, "stop_reason", "end_turn") {
		t.Error("done chunk should have stop_reason = end_turn")
	}
}

func TestFormatStreamChunk_ToolUseFinishReason(t *testing.T) {
	p := &Protocol{}

	chunk := &plugin.StreamChunk{
		Content:      "",
		Done:         true,
		FinishReason: "tool_calls",
		TokensUsed:   50,
	}
	result := p.FormatStreamChunk("claude-3-5-sonnet-20241022", chunk, 0)
	if !containsJSONField(result, "stop_reason", "tool_use") {
		t.Error("tool_calls finish_reason should map to tool_use stop_reason")
	}
}

func TestFormatStreamChunk_NilChunk(t *testing.T) {
	p := &Protocol{}
	result := p.FormatStreamChunk("model", nil, 0)
	if result != "" {
		t.Error("nil chunk should return empty string")
	}
}

func TestFormatStreamChunk_DoneWithNoContent(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{
		Content: "",
		Done:    true,
	}
	result := p.FormatStreamChunk("model", chunk, 0)
	if !strings.Contains(result, "message_stop") {
		t.Error("done chunk with no content should still generate message_stop")
	}
}

func TestFormatStreamDone(t *testing.T) {
	p := &Protocol{}
	result := p.FormatStreamDone()
	if result != "" {
		t.Error("Anthropic FormatStreamDone should return empty string")
	}
}

func TestBuildContentBlocks_TextOnly(t *testing.T) {
	resp := &plugin.ProxyResponse{
		Content:      "Hello world",
		TokensUsed:   100,
		FinishReason: "stop",
	}
	blocks := buildContentBlocks(resp)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text != "Hello world" {
		t.Errorf("expected text block with 'Hello world', got %+v", blocks[0])
	}
}

func TestBuildContentBlocks_WithToolCalls(t *testing.T) {
	resp := &plugin.ProxyResponse{
		Content:      "",
		TokensUsed:   100,
		FinishReason: "tool_calls",
		ToolCalls: []plugin.ToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: plugin.FunctionCall{
					Name:      "get_weather",
					Arguments: `{"location":"Beijing"}`,
				},
			},
		},
	}
	blocks := buildContentBlocks(resp)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "tool_use" {
		t.Errorf("expected tool_use block, got %s", blocks[0].Type)
	}
	if blocks[0].ID != "call_123" {
		t.Errorf("expected ID call_123, got %s", blocks[0].ID)
	}
	if blocks[0].Name != "get_weather" {
		t.Errorf("expected Name get_weather, got %s", blocks[0].Name)
	}
}

func TestBuildContentBlocks_TextAndToolCalls(t *testing.T) {
	resp := &plugin.ProxyResponse{
		Content:      "Let me check",
		TokensUsed:   100,
		FinishReason: "tool_calls",
		ToolCalls: []plugin.ToolCall{
			{
				ID:   "call_456",
				Type: "function",
				Function: plugin.FunctionCall{
					Name:      "search",
					Arguments: `{"query":"test"}`,
				},
			},
		},
	}
	blocks := buildContentBlocks(resp)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "text" {
		t.Errorf("first block should be text, got %s", blocks[0].Type)
	}
	if blocks[1].Type != "tool_use" {
		t.Errorf("second block should be tool_use, got %s", blocks[1].Type)
	}
}

func TestBuildContentBlocks_Empty(t *testing.T) {
	resp := &plugin.ProxyResponse{
		Content: "",
	}
	blocks := buildContentBlocks(resp)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 fallback block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" {
		t.Errorf("fallback block should be text, got %s", blocks[0].Type)
	}
}

func TestMapFinishReason(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"stop", "end_turn"},
		{"", "end_turn"},
		{"tool_calls", "tool_use"},
		{"length", "max_tokens"},
		{"unknown", "end_turn"},
	}
	for _, tt := range tests {
		result := mapFinishReason(tt.input)
		if result != tt.expected {
			t.Errorf("mapFinishReason(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestConvertAnthropicMessages_TextOnly(t *testing.T) {
	messages := []Message{
		{
			Role: "user",
			Content: []ContentBlock{
				{Type: "text", Text: "Hello"},
			},
		},
	}
	result := convertAnthropicMessages(messages)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if result[0].Role != "user" || result[0].Content != "Hello" {
		t.Errorf("expected user message with 'Hello', got %+v", result[0])
	}
}

func TestConvertAnthropicMessages_WithToolUse(t *testing.T) {
	messages := []Message{
		{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "text", Text: "Let me search"},
				{Type: "tool_use", ID: "call_1", Name: "search", Input: map[string]interface{}{"q": "test"}},
			},
		},
	}
	result := convertAnthropicMessages(messages)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if len(result[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result[0].ToolCalls))
	}
	if result[0].ToolCalls[0].ID != "call_1" {
		t.Errorf("expected tool call ID 'call_1', got %s", result[0].ToolCalls[0].ID)
	}
	if result[0].ToolCalls[0].Function.Name != "search" {
		t.Errorf("expected function name 'search', got %s", result[0].ToolCalls[0].Function.Name)
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(result[0].ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Errorf("arguments should be valid JSON: %v", err)
	}
}

func TestParseRequest_BasicMessage(t *testing.T) {
	reqJSON := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]}
		],
		"stream": true
	}`
	var req MessagesRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if req.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected model claude-3-5-sonnet-20241022, got %s", req.Model)
	}
	if !req.Stream {
		t.Error("expected stream=true")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "user" {
		t.Errorf("expected role user, got %s", req.Messages[0].Role)
	}
}

func TestParseRequest_WithTools(t *testing.T) {
	reqJSON := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "What is the weather?"}]}
		],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get weather info",
				"input_schema": {"type": "object", "properties": {"location": {"type": "string"}}}
			}
		]
	}`
	var req MessagesRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(req.Tools))
	}
	if req.Tools[0].Name != "get_weather" {
		t.Errorf("expected tool name get_weather, got %s", req.Tools[0].Name)
	}
}

// containsJSONField 检查字符串中是否包含指定 JSON 字段和值（支持嵌套）
func containsJSONField(s, field, expectedValue string) bool {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过 "data: " 前缀
		line = strings.TrimPrefix(line, "data: ")
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		// 检查顶层
		if v, ok := obj[field]; ok {
			if vs, ok := v.(string); ok && vs == expectedValue {
				return true
			}
		}
		// 检查 delta 嵌套
		if delta, ok := obj["delta"].(map[string]interface{}); ok {
			if v, ok := delta[field]; ok {
				if vs, ok := v.(string); ok && vs == expectedValue {
					return true
				}
			}
		}
	}
	return false
}
