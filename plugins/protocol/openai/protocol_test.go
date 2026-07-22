
package openai

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

func TestMessageContent_UnmarshalString(t *testing.T) {
	var mc MessageContent
	if err := mc.UnmarshalJSON([]byte(`"Hello"`)); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if s, ok := mc.Value.(string); !ok || s != "Hello" {
		t.Errorf("expected Hello, got %v", mc.Value)
	}
}

func TestMessageContent_UnmarshalArray(t *testing.T) {
	var mc MessageContent
	err := mc.UnmarshalJSON([]byte(`[{"type":"text","text":"Hi"},{"type":"image_url","image_url":{"url":"x"}}]`))
	if err != nil {
		t.Fatalf("unmarshal array: %v", err)
	}
	arr, ok := mc.Value.([]map[string]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("expected 2-element array, got %v", mc.Value)
	}
}

func TestMessageContent_UnmarshalObject(t *testing.T) {
	var mc MessageContent
	err := mc.UnmarshalJSON([]byte(`{"type":"text","text":"obj"}`))
	if err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	obj, ok := mc.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", mc.Value)
	}
	if obj["text"] != "obj" {
		t.Errorf("unexpected text: %v", obj["text"])
	}
}

func TestMessageContent_String_Plain(t *testing.T) {
	mc := MessageContent{Value: "plain string"}
	if mc.String() != "plain string" {
		t.Errorf("expected plain string, got %s", mc.String())
	}
}

func TestMessageContent_String_Array(t *testing.T) {
	mc := MessageContent{Value: []map[string]interface{}{
		{"type": "text", "text": "hello"},
		{"type": "image_url"},
		{"type": "text", "text": " world"},
	}}
	if mc.String() != "hello[image] world" {
		t.Errorf("expected hello[image] world, got %q", mc.String())
	}
}

func TestMessageContent_String_Object(t *testing.T) {
	mc := MessageContent{Value: map[string]interface{}{"type": "text", "text": "obj"}}
	if mc.String() != "obj" {
		t.Errorf("expected obj, got %q", mc.String())
	}
}

func TestMessageContent_String_ImageOnly(t *testing.T) {
	mc := MessageContent{Value: []map[string]interface{}{
		{"type": "image_url", "image_url": map[string]interface{}{"url": "http://i"}},
	}}
	if mc.String() != "[image]" {
		t.Errorf("expected [image], got %q", mc.String())
	}
}

func TestMessageContent_MarshalJSON(t *testing.T) {
	mc := MessageContent{Value: "test value"}
	data, err := mc.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `"test value"` {
		t.Errorf("expected quoted, got %s", string(data))
	}
}

func TestConvertMessages_TextOnly(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: MessageContent{Value: "hello"}},
		{Role: "assistant", Content: MessageContent{Value: "hi"}},
	}
	result := convertMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if result[0].Role != "user" || result[0].Content != "hello" {
		t.Errorf("msg0 mismatch: role=%s content=%s", result[0].Role, result[0].Content)
	}
	if result[1].Role != "assistant" || result[1].Content != "hi" {
		t.Errorf("msg1 mismatch: role=%s content=%s", result[1].Role, result[1].Content)
	}
}

func TestConvertMessages_WithToolCalls(t *testing.T) {
	msgs := []Message{{
		Role:    "assistant",
		Content: MessageContent{Value: ""},
		ToolCalls: []ToolCall{{
			ID:   "t1",
			Type: "function",
			Function: FunctionCall{Name: "get_weather", Arguments: "{}"},
		}},
	}}
	result := convertMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if len(result[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result[0].ToolCalls))
	}
	if result[0].ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", result[0].ToolCalls[0].Function.Name)
	}
}

func TestConvertMessages_ReasoningContent(t *testing.T) {
	msgs := []Message{{
		Role:             "assistant",
		Content:          MessageContent{Value: "answer"},
		ReasoningContent: "thinking step 1",
	}}
	result := convertMessages(msgs)
	if result[0].ReasoningContent != "thinking step 1" {
		t.Errorf("expected thinking step 1, got %s", result[0].ReasoningContent)
	}
}

func TestConvertMessages_Empty(t *testing.T) {
	result := convertMessages(nil)
	if len(result) != 0 {
		t.Errorf("expected empty, got %d", len(result))
	}
}

func TestChatCompletionRequest_BasicFields(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}],"temperature":0.7,"max_tokens":100}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Model != "gpt-4" {
		t.Errorf("model: %s", req.Model)
	}
	if req.Temperature != 0.7 {
		t.Errorf("temperature: %f", req.Temperature)
	}
	if req.MaxTokens != 100 {
		t.Errorf("max_tokens: %d", req.MaxTokens)
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}
}

func TestChatCompletionRequest_Stream(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !req.Stream {
		t.Error("expected stream=true")
	}
}

func TestChatCompletionRequest_WithTools(t *testing.T) {
	reqJSON := `{"model":"gpt-4o","messages":[{"role":"user","content":"weather?"}],"tools":[{"type":"function","function":{"name":"get_weather","description":"Get weather","parameters":{"type":"object","properties":{"location":{"type":"string"}}}}}]}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(req.Tools))
	}
	if req.Tools[0].Function.Name != "get_weather" {
		t.Errorf("tool name: %s", req.Tools[0].Function.Name)
	}
}

func TestChatCompletionRequest_ToolChoice(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"tool_choice":"auto"}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s, ok := req.ToolChoice.(string); !ok || s != "auto" {
		t.Errorf("expected auto, got %v", req.ToolChoice)
	}
}

func TestChatCompletionRequest_ResponseFormat(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"response_format":{"type":"json_object"}}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
		t.Errorf("expected json_object, got %v", req.ResponseFormat)
	}
}

func TestChatCompletionRequest_Seed(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"seed":42}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Seed == nil || *req.Seed != 42 {
		t.Errorf("expected seed=42, got %v", req.Seed)
	}
}

func TestChatCompletionRequest_N(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"n":3}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.N == nil || *req.N != 3 {
		t.Errorf("expected n=3, got %v", req.N)
	}
}

func TestChatCompletionRequest_User(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"user":"test-user-123"}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.User != "test-user-123" {
		t.Errorf("expected test-user-123, got %s", req.User)
	}
}

func TestChatCompletionRequest_ParallelToolCalls(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"parallel_tool_calls":false}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.ParallelToolCalls == nil || *req.ParallelToolCalls != false {
		t.Errorf("expected false, got %v", req.ParallelToolCalls)
	}
}

func TestChatCompletionRequest_ReasoningEffort(t *testing.T) {
	reqJSON := `{"model":"o1","messages":[{"role":"user","content":"Hi"}],"reasoning_effort":"high"}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.ReasoningEffort != "high" {
		t.Errorf("expected high, got %s", req.ReasoningEffort)
	}
}

func TestChatCompletionRequest_ServiceTier(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"service_tier":"default"}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.ServiceTier != "default" {
		t.Errorf("expected default, got %s", req.ServiceTier)
	}
}

func TestChatCompletionRequest_Store(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"store":true}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Store == nil || *req.Store != true {
		t.Errorf("expected store=true, got %v", req.Store)
	}
}

func TestChatCompletionRequest_ImageContent(t *testing.T) {
	reqJSON := `{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"text","text":"describe this"},{"type":"image_url","image_url":{"url":"https://example.com/img.png"}}]}]}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	mc := req.Messages[0].Content
	if mc.String() != "describe this[image]" {
		t.Errorf("expected describe this[image], got %q", mc.String())
	}
}

func TestChatCompletionRequest_Metadata(t *testing.T) {
	reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"metadata":{"env":"prod"}}`
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Metadata["env"] != "prod" {
		t.Errorf("expected prod, got %s", req.Metadata["env"])
	}
}

func TestValidateRequest_Success(t *testing.T) {
	p := &Protocol{}
	req := &plugin.ProxyRequest{
		Model:    "gpt-4",
		Messages: []plugin.Message{{Role: "user", Content: "hi"}},
	}
	if err := p.ValidateRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRequest_EmptyModel(t *testing.T) {
	p := &Protocol{}
	req := &plugin.ProxyRequest{
		Messages: []plugin.Message{{Role: "user", Content: "hi"}},
	}
	if err := p.ValidateRequest(req); err == nil {
		t.Error("expected error for empty model")
	}
}

func TestValidateRequest_EmptyMessages(t *testing.T) {
	p := &Protocol{}
	req := &plugin.ProxyRequest{Model: "gpt-4"}
	if err := p.ValidateRequest(req); err == nil {
		t.Error("expected error for empty messages")
	}
}

func TestFormatStreamDone(t *testing.T) {
	p := &Protocol{}
	if p.FormatStreamDone() != "[DONE]" {
		t.Errorf("expected [DONE], got %s", p.FormatStreamDone())
	}
}

func TestFormatStreamChunk_FirstChunkWithRole(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{Content: "Hello"}
	result := p.FormatStreamChunk("gpt-4", chunk, 0)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	choices := obj["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	if delta["role"] != "assistant" {
		t.Error("first chunk should have role=assistant")
	}
	if delta["content"] != "Hello" {
		t.Errorf("expected content Hello, got %v", delta["content"])
	}
}

func TestFormatStreamChunk_MiddleChunkNoRole(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{Content: " world"}
	result := p.FormatStreamChunk("gpt-4", chunk, 1)
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	choices := obj["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	if _, ok := delta["role"]; ok {
		t.Error("middle chunk should not have role")
	}
	if delta["content"] != " world" {
		t.Errorf("expected content world, got %v", delta["content"])
	}
}

func TestFormatStreamChunk_FinishReason(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{
		Content:      "done",
		FinishReason: "stop",
	}
	result := p.FormatStreamChunk("gpt-4", chunk, 0)
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	choices := obj["choices"].([]interface{})
	finish, ok := choices[0].(map[string]interface{})["finish_reason"].(string)
	if !ok || finish != "stop" {
		t.Errorf("expected finish_reason stop, got %v", choices[0].(map[string]interface{})["finish_reason"])
	}
}

func TestFormatStreamChunk_DoneWithUsage(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{
		Done:                 true,
		Content:              "",
		UsagePromptTokens:     20,
		UsageCompletionTokens: 50,
	}
	result := p.FormatStreamChunk("gpt-4", chunk, 2)
	if result == "" {
		t.Fatal("expected usage chunk, got empty")
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	usage := obj["usage"].(map[string]interface{})
	if int(usage["prompt_tokens"].(float64)) != 20 {
		t.Errorf("expected prompt 20, got %v", usage["prompt_tokens"])
	}
	if int(usage["completion_tokens"].(float64)) != 50 {
		t.Errorf("expected completion 50, got %v", usage["completion_tokens"])
	}
	if int(usage["total_tokens"].(float64)) != 70 {
		t.Errorf("expected total 70, got %v", usage["total_tokens"])
	}
}

func TestFormatStreamChunk_DoneWithCompletionOnly(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{
		Done:                 true,
		Content:              "",
		UsagePromptTokens:     0,
		UsageCompletionTokens: 50,
	}
	result := p.FormatStreamChunk("gpt-4", chunk, 2)
	if result == "" {
		t.Fatal("should emit usage when completion > 0")
	}
	var obj map[string]interface{}
	json.Unmarshal([]byte(result), &obj)
	usage := obj["usage"].(map[string]interface{})
	if int(usage["prompt_tokens"].(float64)) != 0 {
		t.Errorf("expected prompt 0, got %v", usage["prompt_tokens"])
	}
	if int(usage["total_tokens"].(float64)) != 50 {
		t.Errorf("expected total 50, got %v", usage["total_tokens"])
	}
}

func TestFormatStreamChunk_DoneWithoutUsage(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{
		Done:                 true,
		Content:              "",
		UsagePromptTokens:     0,
		UsageCompletionTokens: 0,
	}
	result := p.FormatStreamChunk("gpt-4", chunk, 2)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestFormatStreamChunk_ToolCalls(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{
		Content:   "",
		ToolCalls: []plugin.ToolCall{{
			ID:       "t1",
			Type:     "function",
			Function: plugin.FunctionCall{Name: "get_weather", Arguments: `{"location":"NYC"}`},
		}},
	}
	result := p.FormatStreamChunk("gpt-4", chunk, 0)
	if result == "" {
		t.Fatal("expected non-empty")
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		t.Fatalf("invalid JSON: %v | %s", err, result)
	}
	choices := obj["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	tcs := delta["tool_calls"].([]interface{})
	if len(tcs) != 1 {
		t.Fatalf("expected 1 tool_call, got %d", len(tcs))
	}
	fn := tcs[0].(map[string]interface{})["function"].(map[string]interface{})
	if fn["name"] != "get_weather" {
		t.Errorf("expected get_weather, got %v", fn["name"])
	}
}

func TestFormatStreamChunk_ReasoningContent(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{
		Content:          "final",
		ReasoningContent: "let me think",
	}
	result := p.FormatStreamChunk("o1", chunk, 0)
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	choices := obj["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	if delta["reasoning_content"] != "let me think" {
		t.Errorf("expected reasoning, got %v", delta["reasoning_content"])
	}
	if delta["content"] != "final" {
		t.Errorf("expected content final, got %v", delta["content"])
	}
}

func TestFormatStreamChunk_NilChunk(t *testing.T) {
	p := &Protocol{}
	result := p.FormatStreamChunk("gpt-4", nil, 0)
	if result != "" {
		t.Errorf("expected empty for nil chunk, got %q", result)
	}
}

func TestFormatStreamChunk_ModelInResponse(t *testing.T) {
	p := &Protocol{}
	chunk := &plugin.StreamChunk{Content: "test"}
	result := p.FormatStreamChunk("gpt-4-turbo", chunk, 0)
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if obj["model"] != "gpt-4-turbo" {
		t.Errorf("expected gpt-4-turbo, got %v", obj["model"])
	}
}

func TestHandleResponse_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Content:      "Hello from GPT",
		Model:        "gpt-4",
		TokensUsed:   50,
		FinishReason: "stop",
		Metadata:     map[string]any{"prompt_tokens": 20},
	}
	if err := p.HandleResponse(c, resp); err != nil {
		t.Fatalf("HandleResponse: %v", err)
	}
	if w.Code != 200 {
		t.Errorf("status: %d", w.Code)
	}
	var body ChatCompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Object != "chat.completion" {
		t.Errorf("object: %s", body.Object)
	}
	if len(body.Choices) != 1 {
		t.Fatalf("choices: %d", len(body.Choices))
	}
	if body.Choices[0].Message.Content.String() != "Hello from GPT" {
		t.Errorf("content: %s", body.Choices[0].Message.Content.String())
	}
	if body.Choices[0].FinishReason != "stop" {
		t.Errorf("finish: %s", body.Choices[0].FinishReason)
	}
	if body.Model != "gpt-4" {
		t.Errorf("model: %s", body.Model)
	}
	if body.Usage.TotalTokens != 70 {
		t.Errorf("total: %d", body.Usage.TotalTokens)
	}
}

func TestHandleResponse_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Error: &plugin.ErrorResponse{
			Message: "invalid request",
			Type:    "invalid_request_error",
			Code:    "400",
		},
	}
	if err := p.HandleResponse(c, resp); err != nil {
		t.Fatalf("HandleResponse: %v", err)
	}
	if w.Code != 500 {
		t.Errorf("status: %d", w.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	errObj := body["error"].(map[string]interface{})
	if errObj["message"] != "invalid request" {
		t.Errorf("message: %v", errObj["message"])
	}
	if errObj["type"] != "invalid_request_error" {
		t.Errorf("type: %v", errObj["type"])
	}
}

func TestHandleResponse_ErrorWithParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Error: &plugin.ErrorResponse{
			Message: "param error",
			Type:    "invalid_request_error",
			Code:    "400",
			Param:   "max_tokens",
		},
	}
	if err := p.HandleResponse(c, resp); err != nil {
		t.Fatalf("HandleResponse: %v", err)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	errObj := body["error"].(map[string]interface{})
	if errObj["param"] != "max_tokens" {
		t.Errorf("param: %v", errObj["param"])
	}
}

func TestHandleResponse_ErrorWithoutParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Error: &plugin.ErrorResponse{
			Message: "error",
			Type:    "server_error",
			Code:    "500",
		},
	}
	p.HandleResponse(c, resp)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	errObj := body["error"].(map[string]interface{})
	if _, exists := errObj["param"]; exists {
		t.Error("param should not be present when empty")
	}
}

func TestHandleResponse_WithToolCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "gpt-4",
		FinishReason: "tool_calls",
		TokensUsed:   100,
		ToolCalls: []plugin.ToolCall{{
			ID:       "call-1",
			Type:     "function",
			Function: plugin.FunctionCall{Name: "get_weather", Arguments: `{"city":"tokyo"}`},
		}},
		Metadata: map[string]any{"prompt_tokens": 30},
	}
	p.HandleResponse(c, resp)
	if w.Code != 200 {
		t.Errorf("status: %d", w.Code)
	}
	var body ChatCompletionResponse
	json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("tool calls: %d", len(body.Choices[0].Message.ToolCalls))
	}
	tc := body.Choices[0].Message.ToolCalls[0]
	if tc.Function.Name != "get_weather" {
		t.Errorf("name: %s", tc.Function.Name)
	}
	if body.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish: %s", body.Choices[0].FinishReason)
	}
}

func TestHandleResponse_WithRefusal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "gpt-4",
		FinishReason: "stop",
		TokensUsed:   10,
		Metadata: map[string]any{
			"prompt_tokens": 5,
			"refusal":       "I cannot answer that",
		},
	}
	p.HandleResponse(c, resp)
	if w.Code != 200 {
		t.Errorf("status: %d", w.Code)
	}
	var body ChatCompletionResponse
	json.Unmarshal(w.Body.Bytes(), &body)
	msg := body.Choices[0].Message
	if msg.Refusal != "I cannot answer that" {
		t.Errorf("refusal: %s", msg.Refusal)
	}
}

func TestHandleResponse_SystemFingerprint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "gpt-4",
		FinishReason: "stop",
		TokensUsed:   10,
		Metadata: map[string]any{
			"prompt_tokens":      5,
			"system_fingerprint": "fp_abc123",
		},
	}
	p.HandleResponse(c, resp)
	var body ChatCompletionResponse
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.SystemFingerprint != "fp_abc123" {
		t.Errorf("fingerprint: %s", body.SystemFingerprint)
	}
}

func TestHandleResponse_ServiceTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "gpt-4",
		FinishReason: "stop",
		TokensUsed:   10,
		Metadata: map[string]any{
			"prompt_tokens": 5,
			"service_tier":  "default",
		},
	}
	p.HandleResponse(c, resp)
	var body ChatCompletionResponse
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.ServiceTier != "default" {
		t.Errorf("service_tier: %s", body.ServiceTier)
	}
}

func TestHandleResponse_NoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "gpt-4",
		FinishReason: "stop",
		TokensUsed:   10,
		Metadata:     map[string]any{"prompt_tokens": 5},
	}
	p.HandleResponse(c, resp)
	if w.Code != 200 {
		t.Errorf("status: %d", w.Code)
	}
	var body ChatCompletionResponse
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Choices[0].Message.Content.String() != "" {
		t.Errorf("expected empty content, got %s", body.Choices[0].Message.Content.String())
	}
}

func TestHandleResponse_UsageDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "gpt-4",
		FinishReason: "stop",
		TokensUsed:   80,
		Metadata: map[string]any{
			"prompt_tokens":            20,
			"prompt_tokens_details":    &TokenDetails{CachedTokens: 5},
			"completion_tokens_details": &CompletionTokenDetails{ReasoningTokens: 10},
		},
	}
	p.HandleResponse(c, resp)
	var body ChatCompletionResponse
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Usage.PromptTokens != 20 {
		t.Errorf("prompt: %d", body.Usage.PromptTokens)
	}
	if body.Usage.CompletionTokens != 80 {
		t.Errorf("completion: %d", body.Usage.CompletionTokens)
	}
	if body.Usage.TotalTokens != 100 {
		t.Errorf("total: %d", body.Usage.TotalTokens)
	}
	if body.Usage.PromptTokensDetails == nil || body.Usage.PromptTokensDetails.CachedTokens != 5 {
		t.Errorf("prompt details: %v", body.Usage.PromptTokensDetails)
	}
	if body.Usage.CompletionTokensDetails == nil || body.Usage.CompletionTokensDetails.ReasoningTokens != 10 {
		t.Errorf("completion details: %v", body.Usage.CompletionTokensDetails)
	}
}

func TestHandleResponse_FinishReasonToolCallsAutoDetect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &Protocol{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &plugin.ProxyResponse{
		Model:        "gpt-4",
		FinishReason: "stop",
		TokensUsed:   50,
		ToolCalls: []plugin.ToolCall{{
			ID:       "c1",
			Type:     "function",
			Function: plugin.FunctionCall{Name: "search", Arguments: `{}`},
		}},
		Metadata: map[string]any{"prompt_tokens": 10},
	}
	p.HandleResponse(c, resp)
	var body ChatCompletionResponse
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("expected tool_calls, got %s", body.Choices[0].FinishReason)
	}
}
