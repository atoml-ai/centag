package anthropic

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
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



func TestHandleResponse_Success(t *testing.T) {
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "claude-3-5-sonnet-20240620",
		Content:      "Hello from Claude",
		FinishReason: "stop",
		TokensUsed:   15,
	}

	err := p.HandleResponse(c, resp)
	if err != nil {
		t.Fatalf("HandleResponse returned error: %v", err)
	}

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	if body["role"] != "assistant" {
		t.Errorf("expected role assistant, got %v", body["role"])
	}
	if body["model"] != "claude-3-5-sonnet-20240620" {
		t.Errorf("expected model claude-3-5-sonnet-20240620, got %v", body["model"])
	}
	if body["stop_reason"] != "end_turn" {
		t.Errorf("expected stop_reason end_turn, got %v", body["stop_reason"])
	}
	content, _ := body["content"].([]interface{})
	if len(content) != 1 {
		t.Errorf("expected 1 content block, got %d", len(content))
	}
	if cb, ok := content[0].(map[string]interface{}); ok {
		if cb["text"] != "Hello from Claude" {
			t.Errorf("expected text Hello from Claude, got %v", cb["text"])
		}
	}
	usage, _ := body["usage"].(map[string]interface{})
	if outTokens, _ := usage["output_tokens"].(float64); int(outTokens) != 15 {
		t.Errorf("expected output_tokens 15, got %v", usage["output_tokens"])
	}
}

func TestHandleResponse_Error(t *testing.T) {
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Error: &plugin.ErrorResponse{
			Type:    "invalid_request_error",
			Message: "model not found",
		},
	}

	err := p.HandleResponse(c, resp)
	if err != nil {
		t.Fatalf("HandleResponse returned error: %v", err)
	}

	if w.Code != 500 {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	if body["type"] != "error" {
		t.Errorf("expected type error, got %v", body["type"])
	}
	errObj, _ := body["error"].(map[string]interface{})
	if errObj["message"] != "model not found" {
		t.Errorf("expected error message model not found, got %v", errObj["message"])
	}
}

func TestHandleResponse_ErrorDefaultType(t *testing.T) {
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Error: &plugin.ErrorResponse{
			Message: "something went wrong",
		},
	}

	p.HandleResponse(c, resp)
	if w.Code != 500 {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	errObj, _ := body["error"].(map[string]interface{})
	if errObj["type"] != "invalid_request_error" {
		t.Errorf("expected default type invalid_request_error, got %v", errObj["type"])
	}
}

func TestHandleResponse_WithToolCalls(t *testing.T) {
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "claude-3-5-sonnet",
		Content:      "",
		FinishReason: "tool_calls",
		TokensUsed:   42,
		ToolCalls: []plugin.ToolCall{
			{
				ID:   "toolu_01ABC123",
				Type: "function",
				Function: plugin.FunctionCall{
					Name:      "get_weather",
					Arguments: "{\"location\": \"Paris\"}",
				},
			},
		},
	}

	err := p.HandleResponse(c, resp)
	if err != nil {
		t.Fatalf("HandleResponse returned error: %v", err)
	}

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["stop_reason"] != "tool_use" {
		t.Errorf("expected stop_reason tool_use, got %v", body["stop_reason"])
	}
	content, _ := body["content"].([]interface{})
	if len(content) != 1 {
		t.Errorf("expected 1 content block, got %d", len(content))
	}
	if cb, ok := content[0].(map[string]interface{}); ok {
		if cb["type"] != "tool_use" {
			t.Errorf("expected type tool_use, got %v", cb["type"])
		}
		if cb["name"] != "get_weather" {
			t.Errorf("expected name get_weather, got %v", cb["name"])
		}
	}
}

func TestHandleResponse_StopSequence(t *testing.T) {
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	stopSeq := "\n\nHuman:"
	resp := &plugin.ProxyResponse{
		Model:        "claude-3-haiku",
		Content:      "done",
		FinishReason: "stop",
		Metadata: map[string]interface{}{
			"stop_sequence": stopSeq,
		},
	}

	p.HandleResponse(c, resp)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["stop_sequence"] != stopSeq {
		t.Errorf("expected stop_sequence, got %v", body["stop_sequence"])
	}
}

func TestHandleResponse_CacheTokens(t *testing.T) {
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "claude-3-5-sonnet",
		Content:      "cached response",
		FinishReason: "stop",
		TokensUsed:   100,
		Metadata: map[string]interface{}{
			"prompt_tokens":                int(500),
			"cache_creation_input_tokens": int(200),
			"cache_read_input_tokens":     int(300),
		},
	}

	p.HandleResponse(c, resp)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	usage, _ := body["usage"].(map[string]interface{})
	if input, _ := usage["input_tokens"].(float64); int(input) != 500 {
		t.Errorf("expected input_tokens 500, got %v", usage["input_tokens"])
	}
	if cacheCreate, _ := usage["cache_creation_input_tokens"].(float64); int(cacheCreate) != 200 {
		t.Errorf("expected cache_creation_input_tokens 200, got %v", usage["cache_creation_input_tokens"])
	}
	if cacheRead, _ := usage["cache_read_input_tokens"].(float64); int(cacheRead) != 300 {
		t.Errorf("expected cache_read_input_tokens 300, got %v", usage["cache_read_input_tokens"])
	}
}

func TestHandleResponse_EmptyContentDefaultStopReason(t *testing.T) {
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:   "claude-3-haiku",
		Content: "",
	}

	p.HandleResponse(c, resp)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	content, _ := body["content"].([]interface{})
	if len(content) != 1 {
		t.Errorf("expected 1 empty content block, got %d", len(content))
	}
}

func TestValidateRequest_Success(t *testing.T) {
	p := &Protocol{}
	req := &plugin.ProxyRequest{
		Model:    "claude-3-5-sonnet",
		Messages: []plugin.Message{{Role: "user", Content: "hello"}},
	}
	if err := p.ValidateRequest(req); err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
}

func TestValidateRequest_EmptyModel(t *testing.T) {
	p := &Protocol{}
	req := &plugin.ProxyRequest{
		Messages: []plugin.Message{{Role: "user", Content: "hello"}},
	}
	if err := p.ValidateRequest(req); err == nil {
		t.Errorf("expected error for empty model")
	}
}

func TestValidateRequest_EmptyMessages(t *testing.T) {
	p := &Protocol{}
	req := &plugin.ProxyRequest{
		Model: "claude-3-5-sonnet",
	}
	if err := p.ValidateRequest(req); err == nil {
		t.Errorf("expected error for empty messages")
	}
}

// [v0.2.8 G3] tool_use_id 提取测试
func TestToolUseID_Extraction(t *testing.T) {
	// 模拟 Anthropic 消息：assistant 发出 tool_use，user 回复 tool_result
	messages := []Message{
		{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "text", Text: "Let me check"},
				{Type: "tool_use", ID: "toolu_01ABC123", Name: "get_weather", Input: map[string]interface{}{"location": "Paris"}},
			},
		},
		{
			Role: "user",
			Content: []ContentBlock{
				{Type: "tool_result", ToolUseID: "toolu_01ABC123", Text: "Sunny, 25°C"},
			},
		},
	}

	result := convertAnthropicMessages(messages)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	// assistant 消息应包含 tool_calls
	if len(result[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result[0].ToolCalls))
	}
	if result[0].ToolCalls[0].ID != "toolu_01ABC123" {
		t.Errorf("expected tool call ID toolu_01ABC123, got %s", result[0].ToolCalls[0].ID)
	}

	// user 消息的 ToolCallID 应正确提取自 tool_result.tool_use_id
	if result[1].ToolCallID != "toolu_01ABC123" {
		t.Errorf("expected ToolCallID toolu_01ABC123, got %s", result[1].ToolCallID)
	}
}

// [v0.2.8 R04] thinking 流式事件测试
func TestFormatStreamChunk_Thinking(t *testing.T) {
	p := &Protocol{}

	// 模拟一个包含 reasoning_content 的 chunk
	chunk := &plugin.StreamChunk{
		ReasoningContent: "Let me think about this...",
		Content:          "",
		Done:             false,
	}

	result := p.FormatStreamChunk("claude-3-5-sonnet-20241022", chunk, 0)
	if result == "" {
		t.Fatal("expected non-empty result for thinking chunk")
	}

	// 应包含 thinking 事件（index=1）
	if !strings.Contains(result, "event: content_block_start") {
		t.Error("thinking chunk should contain content_block_start event")
	}
	if !strings.Contains(result, `"thinking"`) {
		t.Error("thinking chunk should contain thinking type")
	}
	if !strings.Contains(result, "thinking_delta") {
		t.Error("thinking chunk should contain thinking_delta event")
	}
	if !strings.Contains(result, "Let me think about this...") {
		t.Error("thinking chunk should contain the reasoning content")
	}
	if !strings.Contains(result, `"index":1`) {
		t.Error("thinking chunk should use index=1 (separate from text index=0)")
	}
}

// [v0.2.8 G1] RawBody 存储为 map 测试
func TestParseRequest_RawBodyIsMap(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reqJSON := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]}
		],
		"thinking": {"type": "enabled", "budget_tokens": 10000},
		"custom_field": "should_be_preserved"
	}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", strings.NewReader(reqJSON))
	c.Request.Header.Set("Content-Type", "application/json")

	p := &Protocol{}
	proxyReq, err := p.ParseRequest(c)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	// RawBody 应为 map[string]interface{}
	rawBodyMap, ok := proxyReq.RawBody.(map[string]interface{})
	if !ok {
		t.Fatalf("RawBody should be map[string]interface{}, got %T", proxyReq.RawBody)
	}

	// 应保留原始字段
	if rawBodyMap["model"] != "claude-3-5-sonnet-20241022" {
		t.Errorf("RawBody should contain model field")
	}
	if rawBodyMap["custom_field"] != "should_be_preserved" {
		t.Errorf("RawBody should preserve unknown fields for backend passthrough")
	}
}

// [v0.2.8] thinking 配置解析测试
func TestParseRequest_Thinking(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reqJSON := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 4096,
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Solve this math problem"}]}
		],
		"thinking": {"type": "enabled", "budget_tokens": 10000}
	}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", strings.NewReader(reqJSON))
	c.Request.Header.Set("Content-Type", "application/json")

	p := &Protocol{}
	proxyReq, err := p.ParseRequest(c)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	// Reasoning 应被正确映射
	if !proxyReq.Reasoning.Specified {
		t.Error("Reasoning.Specified should be true when thinking is enabled")
	}
	if proxyReq.Reasoning.BudgetTokens == nil {
		t.Fatal("Reasoning.BudgetTokens should not be nil")
	}
	if *proxyReq.Reasoning.BudgetTokens != 10000 {
		t.Errorf("expected BudgetTokens 10000, got %d", *proxyReq.Reasoning.BudgetTokens)
	}
}

func TestNewProtocol(t *testing.T) {
	p, err := NewProtocol()
	if err != nil {
		t.Fatalf("NewProtocol failed: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil protocol")
	}
}

func TestProtocol_Name(t *testing.T) {
	p, _ := NewProtocol()
	if p.Name() != "anthropic-protocol" {
		t.Errorf("expected anthropic-protocol, got %s", p.Name())
	}
}

func TestProtocol_Type(t *testing.T) {
	p, _ := NewProtocol()
	if p.Type() != plugin.TypeProtocol {
		t.Errorf("expected TypeProtocol, got %v", p.Type())
	}
}

func TestProtocol_Version(t *testing.T) {
	p, _ := NewProtocol()
	if p.Version() != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", p.Version())
	}
}

func TestProtocol_Init(t *testing.T) {
	p, _ := NewProtocol()
	err := p.Init(nil)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if p.Status() != plugin.StatusStopped {
		t.Errorf("expected StatusStopped after Init, got %v", p.Status())
	}
}

func TestProtocol_Start(t *testing.T) {
	p, _ := NewProtocol()
	p.Init(nil)
	err := p.Start(nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if p.Status() != plugin.StatusRunning {
		t.Errorf("expected StatusRunning after Start, got %v", p.Status())
	}
}

func TestProtocol_Stop(t *testing.T) {
	p, _ := NewProtocol()
	p.Init(nil)
	p.Start(nil)
	err := p.Stop(nil)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if p.Status() != plugin.StatusStopped {
		t.Errorf("expected StatusStopped after Stop, got %v", p.Status())
	}
}

func TestProtocol_SupportStream(t *testing.T) {
	p, err := NewProtocol()
	if err != nil {
		t.Fatalf("NewProtocol failed: %v", err)
	}
	proto := p.(*Protocol)
	if !proto.SupportStream() {
		t.Error("expected SupportStream to return true")
	}
}

func TestProtocol_GetModels(t *testing.T) {
	p, err := NewProtocol()
	if err != nil {
		t.Fatalf("NewProtocol failed: %v", err)
	}
	proto := p.(*Protocol)
	models, err := proto.GetModels()
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}
	if len(models) == 0 {
		t.Error("expected at least one model")
	}
	for _, m := range models {
		if m.ID == "" {
			t.Error("model ID should not be empty")
		}
		if m.Name == "" {
			t.Error("model name should not be empty")
		}
	}
}
