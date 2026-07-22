package anthropic

import (
	"testing"

	"centag/core/pkg/plugin"
	"centag/plugins/protocol/shared"
)

// TestAnthropicProtocolE2E 全量协议端到端测试套件
func TestAnthropicProtocolE2E(t *testing.T) {
	protocol := &Protocol{}
	runner := shared.NewProtocolTestRunner(t, protocol)

	t.Run("请求解析", func(t *testing.T) {
		runAnthropicRequestParseTests(t, runner)
	})

	t.Run("响应构建", func(t *testing.T) {
		runAnthropicResponseBuildTests(t, runner)
	})

	t.Run("流式响应", func(t *testing.T) {
		runAnthropicStreamTests(t, runner)
	})

	t.Run("边界情况", func(t *testing.T) {
		runAnthropicEdgeCaseTests(t, runner)
	})
}

// runAnthropicRequestParseTests 请求解析测试
func runAnthropicRequestParseTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	// 测试用例1: 基础字段
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "基础字段解析",
		RequestJSON: `{
			"model": "claude-3-5-sonnet-20241022",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}],
			"temperature": 0.7,
			"top_p": 0.9,
			"system": "You are helpful"
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			shared.AssertField(t, map[string]interface{}{
				"model":       req.Model,
				"max_tokens":  req.MaxTokens,
				"temperature": req.Temperature,
				"top_p":       req.TopP,
			}, "model", "claude-3-5-sonnet-20241022")
			if req.MaxTokens != 1024 {
				t.Errorf("max_tokens: got %d, want 1024", req.MaxTokens)
			}
			if req.System != "You are helpful" {
				t.Errorf("system: got %q, want 'You are helpful'", req.System)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Hello!",
			Model:        "claude-3-5-sonnet-20241022",
			TokensUsed:   10,
			FinishReason: "stop",
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "type", "message")
			shared.AssertField(t, resp, "role", "assistant")
		},
	})

	// 测试用例2: 工具调用
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "工具调用解析",
		RequestJSON: `{
			"model": "claude-3-5-sonnet-20241022",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "What is the weather?"}]}],
			"tools": [
				{
					"name": "get_weather",
					"description": "Get weather info",
					"input_schema": {"type": "object", "properties": {"location": {"type": "string"}}}
				}
			],
			"tool_choice": {"type": "auto"}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if len(req.Tools) != 1 {
				t.Fatalf("tools: got %d, want 1", len(req.Tools))
			}
			if req.Tools[0].Function.Name != "get_weather" {
				t.Errorf("tool name: got %q, want get_weather", req.Tools[0].Function.Name)
			}
			if req.ToolChoice == nil {
				t.Error("tool_choice should not be nil")
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content: "",
			ToolCalls: []plugin.ToolCall{
				{
					ID:   "toolu_123",
					Type: "function",
					Function: plugin.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"Beijing"}`,
					},
				},
			},
			TokensUsed:   20,
			FinishReason: "tool_calls",
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "type", "message")
			shared.AssertField(t, resp, "stop_reason", "tool_use")
		},
	})

	// 测试用例3: thinking配置
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "thinking配置解析",
		RequestJSON: `{
			"model": "claude-3-5-sonnet-20241022",
			"max_tokens": 4096,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Think"}]}],
			"thinking": {"type": "enabled", "budget_tokens": 10000}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if !req.Reasoning.Specified {
				t.Error("reasoning.specified should be true")
			}
			if req.Reasoning.BudgetTokens == nil {
				t.Fatal("reasoning.budget_tokens should not be nil")
			}
			if *req.Reasoning.BudgetTokens != 10000 {
				t.Errorf("budget_tokens: got %d, want 10000", *req.Reasoning.BudgetTokens)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Thinking...",
			TokensUsed:   100,
			FinishReason: "stop",
		},
	})

	// 测试用例4: metadata.user_id
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "metadata.user_id解析",
		RequestJSON: `{
			"model": "claude-3-5-sonnet-20241022",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}],
			"metadata": {"user_id": "user-456"}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.Metadata == nil {
				t.Fatal("metadata should not be nil")
			}
			if req.Metadata["anthropic_user_id"] != "user-456" {
				t.Errorf("metadata.anthropic_user_id: got %v, want user-456", req.Metadata["anthropic_user_id"])
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Tracked",
			TokensUsed:   5,
			FinishReason: "stop",
		},
	})

	// 测试用例5: stream_options
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "stream_options解析",
		RequestJSON: `{
			"model": "claude-3-5-sonnet-20241022",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}],
			"stream_options": {"include_usage": true}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.Metadata == nil {
				t.Fatal("metadata should not be nil")
			}
			if req.Metadata["stream_options"] == nil {
				t.Error("metadata.stream_options should not be nil")
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Done",
			TokensUsed:   5,
			FinishReason: "stop",
		},
	})

	// 测试用例6: RawBody是map
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "RawBody是map",
		RequestJSON: `{
			"model": "claude-3-5-sonnet-20241022",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}],
			"custom_field": "preserved"
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			rawBody, ok := req.RawBody.(map[string]interface{})
			if !ok {
				t.Fatalf("RawBody should be map, got %T", req.RawBody)
			}
			if rawBody["custom_field"] != "preserved" {
				t.Error("RawBody should preserve custom_field")
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Done",
			TokensUsed:   5,
			FinishReason: "stop",
		},
	})

	// 测试用例7: tool_result回环
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "tool_result回环",
		RequestJSON: `{
			"model": "claude-3-5-sonnet-20241022",
			"max_tokens": 1024,
			"messages": [
				{
					"role": "assistant",
					"content": [
						{"type": "text", "text": "Let me check"},
						{"type": "tool_use", "id": "toolu_789", "name": "search", "input": {"q": "test"}}
					]
				},
				{
					"role": "user",
					"content": [
						{"type": "tool_result", "tool_use_id": "toolu_789", "content": "Found results"}
					]
				}
			]
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if len(req.Messages) != 2 {
				t.Fatalf("messages: got %d, want 2", len(req.Messages))
			}
			// 第二条消息应该有 ToolCallID
			if req.Messages[1].ToolCallID != "toolu_789" {
				t.Errorf("ToolCallID: got %q, want toolu_789", req.Messages[1].ToolCallID)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Found it",
			TokensUsed:   10,
			FinishReason: "stop",
		},
	})

	// 测试用例8: Tools转换为内部格式
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "Tools转换为内部格式",
		RequestJSON: `{
			"model": "claude-3-5-sonnet-20241022",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Use tool"}]}],
			"tools": [
				{
					"name": "calculate",
					"description": "Calculate expression",
					"input_schema": {"type": "object", "properties": {"expr": {"type": "string"}}}
				}
			]
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if len(req.Tools) != 1 {
				t.Fatalf("tools: got %d, want 1", len(req.Tools))
			}
			// Anthropic tools 应转换为 OpenAI 格式
			if req.Tools[0].Type != "function" {
				t.Errorf("tool type: got %q, want function", req.Tools[0].Type)
			}
			if req.Tools[0].Function.Name != "calculate" {
				t.Errorf("tool name: got %q, want calculate", req.Tools[0].Function.Name)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Calculated",
			TokensUsed:   10,
			FinishReason: "stop",
		},
	})
}

// runAnthropicResponseBuildTests 响应构建测试
func runAnthropicResponseBuildTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	// 测试用例1: usage计算
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "usage计算正确",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content:      "Response",
			TokensUsed:   100,
			FinishReason: "stop",
			Metadata: map[string]interface{}{
				"prompt_tokens": 50,
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			usage, ok := resp["usage"].(map[string]interface{})
			if !ok {
				t.Fatal("usage not found")
			}
			shared.AssertFieldExists(t, usage, "input_tokens")
			shared.AssertFieldExists(t, usage, "output_tokens")
		},
	})

	// 测试用例2: stop_sequence
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "stop_sequence传递",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content:      "Response",
			TokensUsed:   10,
			FinishReason: "stop",
			Metadata: map[string]interface{}{
				"stop_sequence": "\n\nHuman:",
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "stop_sequence", "\n\nHuman:")
		},
	})

	// 测试用例3: cache tokens
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "cache_tokens传递",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content:      "Cached",
			TokensUsed:   100,
			FinishReason: "stop",
			Metadata: map[string]interface{}{
				"prompt_tokens":                500,
				"cache_creation_input_tokens":  200,
				"cache_read_input_tokens":      300,
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			usage := resp["usage"].(map[string]interface{})
			shared.AssertField(t, usage, "input_tokens", float64(500))
			shared.AssertField(t, usage, "cache_creation_input_tokens", float64(200))
			shared.AssertField(t, usage, "cache_read_input_tokens", float64(300))
		},
	})

	// 测试用例4: 错误格式(G2修复)
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "错误格式_G2修复",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}]}`,
		MockResponse: &plugin.ProxyResponse{
			Error: &plugin.ErrorResponse{
				Type:    "invalid_request_error",
				Message: "Missing required field",
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "type", "error")
			errObj, ok := resp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("error should be object")
			}
			shared.AssertField(t, errObj, "type", "invalid_request_error")
			shared.AssertField(t, errObj, "message", "Missing required field")
		},
	})

	// 测试用例5: tool_use响应
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "tool_use响应",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "Use tool"}]}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content: "",
			ToolCalls: []plugin.ToolCall{
				{
					ID:   "toolu_456",
					Type: "function",
					Function: plugin.FunctionCall{
						Name:      "search",
						Arguments: `{"q":"test"}`,
					},
				},
			},
			TokensUsed:   20,
			FinishReason: "tool_calls",
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "stop_reason", "tool_use")
			content, ok := resp["content"].([]interface{})
			if !ok || len(content) == 0 {
				t.Fatal("content should not be empty")
			}
			block := content[0].(map[string]interface{})
			shared.AssertField(t, block, "type", "tool_use")
			shared.AssertField(t, block, "id", "toolu_456")
			shared.AssertField(t, block, "name", "search")
		},
	})

	// 测试用例6: 默认stop_reason
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "默认stop_reason",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content:      "Response",
			TokensUsed:   10,
			FinishReason: "",
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "stop_reason", "end_turn")
		},
	})
}

// runAnthropicStreamTests 流式响应测试
func runAnthropicStreamTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	// 测试用例1: 基础流式
	runner.RunStreamTest(shared.TestCase{
		Name: "基础流式响应",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "Stream"}]}], "stream": true}`,
		MockBackend: &shared.MockBackend{
			StreamChunks: []*plugin.StreamChunk{
				{Content: "Hello", Done: false},
				{Content: " World", Done: false},
				{Content: "", Done: true, FinishReason: "stop", TokensUsed: 10},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			chunks := resp["chunks"].([]string)
			count := resp["count"].(int)
			if count < 3 {
				t.Errorf("expected at least 3 chunks, got %d", count)
			}
			// 应该有 message_start
			foundStart := false
			for _, chunk := range chunks {
				if shared.ContainsAny(chunk, "message_start") {
					foundStart = true
					break
				}
			}
			if !foundStart {
				t.Error("message_start not found")
			}
		},
	})

	// 测试用例2: thinking流式
	runner.RunStreamTest(shared.TestCase{
		Name: "thinking流式",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 4096, "messages": [{"role": "user", "content": [{"type": "text", "text": "Think"}]}], "stream": true}`,
		MockBackend: &shared.MockBackend{
			StreamChunks: []*plugin.StreamChunk{
				{ReasoningContent: "Let me think...", Done: false},
				{Content: "Answer", Done: false},
				{Content: "", Done: true, FinishReason: "stop", TokensUsed: 50},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			chunks := resp["chunks"].([]string)
			foundThinking := false
			for _, chunk := range chunks {
				if shared.ContainsAny(chunk, "thinking", "thinking_delta") {
					foundThinking = true
					break
				}
			}
			if !foundThinking {
				t.Error("thinking event not found")
			}
		},
	})

	// 测试用例3: tool_use流式
	runner.RunStreamTest(shared.TestCase{
		Name: "tool_use流式",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "Tool"}]}], "stream": true}`,
		MockBackend: &shared.MockBackend{
			StreamChunks: []*plugin.StreamChunk{
				{
					ToolCalls: []plugin.ToolCall{
						{ID: "toolu_789", Type: "function", Function: plugin.FunctionCall{Name: "search", Arguments: `{}`}},
					},
					Done: false,
				},
				{Content: "", Done: true, FinishReason: "tool_calls", TokensUsed: 10},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			chunks := resp["chunks"].([]string)
			// 应该有 tool_use 相关事件
			foundToolUse := false
			for _, chunk := range chunks {
				if shared.ContainsAny(chunk, "tool_use") {
					foundToolUse = true
					break
				}
			}
			if !foundToolUse {
				t.Error("tool_use event not found")
			}
		},
	})

	// 测试用例4: stop_reason映射
	runner.RunStreamTest(shared.TestCase{
		Name: "stop_reason映射",
		RequestJSON: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}], "stream": true}`,
		MockBackend: &shared.MockBackend{
			StreamChunks: []*plugin.StreamChunk{
				{Content: "Done", Done: false},
				{Content: "", Done: true, FinishReason: "tool_calls", TokensUsed: 10},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			chunks := resp["chunks"].([]string)
			// 应该有 message_delta 包含 tool_use
			foundToolUse := false
			for _, chunk := range chunks {
				if shared.ContainsAny(chunk, "tool_use") {
					foundToolUse = true
					break
				}
			}
			if !foundToolUse {
				t.Error("tool_use stop_reason not found")
			}
		},
	})
}

// runAnthropicEdgeCaseTests 边界情况测试
func runAnthropicEdgeCaseTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	// 测试用例1: 畸形JSON
	t.Run("畸形JSON处理", func(t *testing.T) {
		protocol := &Protocol{}
		c, _ := shared.CreateTestGinContext(`{invalid json`)
		_, err := protocol.ParseRequest(c)
		if err == nil {
			t.Error("expected error for malformed JSON")
		}
	})

	// 测试用例2: 缺少model
	t.Run("缺少model", func(t *testing.T) {
		protocol := &Protocol{}
		c, _ := shared.CreateTestGinContext(`{"max_tokens":1024,"messages":[{"role":"user","content":[{"type":"text","text":"Hi"}]}]}`)
		req, err := protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Model != "" {
			t.Errorf("model should be empty, got %q", req.Model)
		}
	})

	// 测试用例3: thinking禁用
	t.Run("thinking禁用", func(t *testing.T) {
		protocol := &Protocol{}
		reqJSON := `{"model":"claude-3-5-sonnet-20241022","max_tokens":1024,"messages":[{"role":"user","content":[{"type":"text","text":"Test"}]}],"thinking":{"type":"disabled"}}`
		c, _ := shared.CreateTestGinContext(reqJSON)
		req, err := protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Reasoning.Specified {
			t.Error("reasoning.specified should be false when thinking is disabled")
		}
	})

	// 测试用例4: 空content块
	t.Run("空content块", func(t *testing.T) {
		protocol := &Protocol{}
		reqJSON := `{"model":"claude-3-5-sonnet-20241022","max_tokens":1024,"messages":[{"role":"user","content":[]}]}`
		c, _ := shared.CreateTestGinContext(reqJSON)
		req, err := protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Messages) != 1 {
			t.Fatalf("messages: got %d, want 1", len(req.Messages))
		}
	})

	// 测试用例5: 多种content类型
	t.Run("多种content类型", func(t *testing.T) {
		protocol := &Protocol{}
		reqJSON := `{"model":"claude-3-5-sonnet-20241022","max_tokens":1024,"messages":[{"role":"user","content":[{"type":"text","text":"Hello"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"..."}},{"type":"tool_result","tool_use_id":"toolu_123","content":"Result"}]}]}`
		c, _ := shared.CreateTestGinContext(reqJSON)
		req, err := protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Messages) != 1 {
			t.Fatalf("messages: got %d, want 1", len(req.Messages))
		}
	})

	// 测试用例6: 并发安全性
	t.Run("并发安全性", func(t *testing.T) {
		protocol := &Protocol{}
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				c, _ := shared.CreateTestGinContext(`{"model":"claude-3-5-sonnet-20241022","max_tokens":1024,"messages":[{"role":"user","content":[{"type":"text","text":"Concurrent"}]}]}`)
				_, err := protocol.ParseRequest(c)
				if err != nil {
					t.Errorf("concurrent ParseRequest failed: %v", err)
				}
			}()
		}
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	// 测试用例7: 验证G2错误格式
	t.Run("G2错误格式验证", func(t *testing.T) {
		protocol := &Protocol{}
		c, _ := shared.CreateTestGinContext(`{}`)
		errResp := &plugin.ErrorResponse{
			Type:    "authentication_error",
			Message: "Invalid API key",
		}
		resp := &plugin.ProxyResponse{Error: errResp}
		err := protocol.HandleResponse(c, resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// 测试用例8: 验证tool_use_id映射
	t.Run("tool_use_id映射验证", func(t *testing.T) {
		protocol := &Protocol{}
		reqJSON := `{"model":"claude-3-5-sonnet-20241022","max_tokens":1024,"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"toolu_abc","name":"search","input":{"q":"test"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_abc","content":"Found it"}]}]}`
		c, _ := shared.CreateTestGinContext(reqJSON)
		req, err := protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Messages[0].ToolCalls) != 1 {
			t.Fatalf("tool_calls: got %d, want 1", len(req.Messages[0].ToolCalls))
		}
		if req.Messages[0].ToolCalls[0].ID != "toolu_abc" {
			t.Errorf("tool_call id: got %q, want toolu_abc", req.Messages[0].ToolCalls[0].ID)
		}
		if req.Messages[1].ToolCallID != "toolu_abc" {
			t.Errorf("ToolCallID: got %q, want toolu_abc", req.Messages[1].ToolCallID)
		}
	})
}
