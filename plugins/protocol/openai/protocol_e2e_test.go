package openai

import (
	"testing"

	"centag/core/pkg/plugin"
	"centag/plugins/protocol/shared"
)

// TestOpenAIProtocolE2E 全量协议端到端测试套件
func TestOpenAIProtocolE2E(t *testing.T) {
	protocol := &Protocol{}
	runner := shared.NewProtocolTestRunner(t, protocol)

	t.Run("请求解析", func(t *testing.T) {
		runOpenAIRequestParseTests(t, runner)
	})

	t.Run("响应构建", func(t *testing.T) {
		runOpenAIResponseBuildTests(t, runner)
	})

	t.Run("流式响应", func(t *testing.T) {
		runOpenAIStreamTests(t, runner)
	})

	t.Run("边界情况", func(t *testing.T) {
		runOpenAIEdgeCaseTests(t, runner)
	})
}

// runOpenAIRequestParseTests 请求解析测试
func runOpenAIRequestParseTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	// 测试用例1: 基础字段
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "基础字段解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Hello"}],
			"temperature": 0.7,
			"max_tokens": 1024,
			"top_p": 0.9,
			"frequency_penalty": 0.1,
			"presence_penalty": 0.2,
			"stop": ["END"]
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			shared.AssertField(t, map[string]interface{}{
				"model":             req.Model,
				"temperature":       req.Temperature,
				"max_tokens":        req.MaxTokens,
				"top_p":             req.TopP,
				"frequency_penalty": req.FrequencyPenalty,
				"presence_penalty":  req.PresencePenalty,
			}, "model", "gpt-4")
			if req.Temperature != 0.7 {
				t.Errorf("temperature: got %f, want 0.7", req.Temperature)
			}
			if req.MaxTokens != 1024 {
				t.Errorf("max_tokens: got %d, want 1024", req.MaxTokens)
			}
			if len(req.Stop) != 1 || req.Stop[0] != "END" {
				t.Errorf("stop: got %v, want [END]", req.Stop)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Hello!",
			Model:        "gpt-4",
			TokensUsed:   10,
			FinishReason: "stop",
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "object", "chat.completion")
			shared.AssertFieldExists(t, resp, "usage")
		},
	})

	// 测试用例2: 工具调用
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "工具调用解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "What is the weather?"}],
			"tools": [
				{
					"type": "function",
					"function": {
						"name": "get_weather",
						"description": "Get weather info",
						"parameters": {"type": "object", "properties": {"location": {"type": "string"}}}
					}
				}
			],
			"tool_choice": "auto"
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if len(req.Tools) != 1 {
				t.Fatalf("tools: got %d, want 1", len(req.Tools))
			}
			if req.Tools[0].Function.Name != "get_weather" {
				t.Errorf("tool name: got %q, want get_weather", req.Tools[0].Function.Name)
			}
			if req.ToolChoice != "auto" {
				t.Errorf("tool_choice: got %v, want auto", req.ToolChoice)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content: "",
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
			TokensUsed:   20,
			FinishReason: "tool_calls",
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "object", "chat.completion")
		},
	})

	// 测试用例3: tool_choice 对象形式
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "tool_choice对象形式",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Use the tool"}],
			"tools": [{"type": "function", "function": {"name": "search"}}],
			"tool_choice": {"type": "function", "function": {"name": "search"}}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			tc, ok := req.ToolChoice.(map[string]interface{})
			if !ok {
				t.Fatalf("tool_choice should be object, got %T", req.ToolChoice)
			}
			if tc["type"] != "function" {
				t.Errorf("tool_choice.type: got %v, want function", tc["type"])
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Done",
			TokensUsed:   5,
			FinishReason: "stop",
		},
	})

	// 测试用例4: response_format
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "response_format解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Return JSON"}],
			"response_format": {"type": "json_object"}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.ResponseFormat == nil {
				t.Fatal("response_format should not be nil")
			}
			if req.ResponseFormat.Type != "json_object" {
				t.Errorf("response_format.type: got %q, want json_object", req.ResponseFormat.Type)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      `{"key": "value"}`,
			TokensUsed:   15,
			FinishReason: "stop",
		},
	})

	// 测试用例5: seed
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "seed解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Random"}],
			"seed": 42
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.Seed == nil {
				t.Fatal("seed should not be nil")
			}
			if *req.Seed != 42 {
				t.Errorf("seed: got %d, want 42", *req.Seed)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Result",
			TokensUsed:   5,
			FinishReason: "stop",
		},
	})

	// 测试用例6: n
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "n解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Generate"}],
			"n": 3
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.N == nil {
				t.Fatal("n should not be nil")
			}
			if *req.N != 3 {
				t.Errorf("n: got %d, want 3", *req.N)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Generated",
			TokensUsed:   10,
			FinishReason: "stop",
		},
	})

	// 测试用例7: user
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "user解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Track me"}],
			"user": "user-123"
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.User != "user-123" {
				t.Errorf("user: got %q, want user-123", req.User)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Tracked",
			TokensUsed:   5,
			FinishReason: "stop",
		},
	})

	// 测试用例8: parallel_tool_calls
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "parallel_tool_calls解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Use tools"}],
			"parallel_tool_calls": true
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.ParallelToolCalls == nil {
				t.Fatal("parallel_tool_calls should not be nil")
			}
			if !*req.ParallelToolCalls {
				t.Error("parallel_tool_calls: got false, want true")
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Done",
			TokensUsed:   5,
			FinishReason: "stop",
		},
	})

	// 测试用例9: reasoning_effort
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "reasoning_effort解析",
		RequestJSON: `{
			"model": "o1",
			"messages": [{"role": "user", "content": "Think"}],
			"reasoning_effort": "high"
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if !req.Reasoning.Specified {
				t.Error("reasoning.specified should be true")
			}
			if req.Reasoning.Effort != "high" {
				t.Errorf("reasoning.effort: got %q, want high", req.Reasoning.Effort)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Thinking...",
			TokensUsed:   50,
			FinishReason: "stop",
		},
	})

	// 测试用例10: RawBody保留
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "RawBody保留未知字段",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Test"}],
			"custom_field": "should_be_preserved",
			"another_field": 123
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			rawBody, ok := req.RawBody.(map[string]interface{})
			if !ok {
				t.Fatal("RawBody should be map")
			}
			if rawBody["custom_field"] != "should_be_preserved" {
				t.Error("RawBody should preserve custom_field")
			}
			if rawBody["another_field"] != float64(123) {
				t.Error("RawBody should preserve another_field")
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "Done",
			TokensUsed:   5,
			FinishReason: "stop",
		},
	})

	// 测试用例11: tool_choice none
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "tool_choice_none",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "No tools"}],
			"tool_choice": "none"
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.ToolChoice != "none" {
				t.Errorf("tool_choice: got %v, want none", req.ToolChoice)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:      "OK",
			TokensUsed:   5,
			FinishReason: "stop",
		},
	})

	// 测试用例12: tool_choice required
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "tool_choice_required",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Must use tool"}],
			"tools": [{"type": "function", "function": {"name": "search"}}],
			"tool_choice": "required"
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.ToolChoice != "required" {
				t.Errorf("tool_choice: got %v, want required", req.ToolChoice)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content: "",
			ToolCalls: []plugin.ToolCall{
				{ID: "call_456", Type: "function", Function: plugin.FunctionCall{Name: "search", Arguments: `{}`}},
			},
			TokensUsed:   10,
			FinishReason: "tool_calls",
		},
	})
}

// runOpenAIResponseBuildTests 响应构建测试
func runOpenAIResponseBuildTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	// 测试用例1: usage计算
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "usage计算正确",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Test"}]}`,
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
			// TotalTokens = PromptTokens + CompletionTokens
			promptTokens := int(usage["prompt_tokens"].(float64))
			completionTokens := int(usage["completion_tokens"].(float64))
			totalTokens := int(usage["total_tokens"].(float64))
			if totalTokens != promptTokens+completionTokens {
				t.Errorf("total_tokens: got %d, want %d+%d=%d", totalTokens, promptTokens, completionTokens, promptTokens+completionTokens)
			}
		},
	})

	// 测试用例2: system_fingerprint
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "system_fingerprint传递",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Test"}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content:      "Response",
			TokensUsed:   10,
			FinishReason: "stop",
			Metadata: map[string]interface{}{
				"system_fingerprint": "fp_123456",
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "system_fingerprint", "fp_123456")
		},
	})

	// 测试用例3: refusal
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "refusal填充",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Bad request"}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content:      "",
			TokensUsed:   5,
			FinishReason: "stop",
			Metadata: map[string]interface{}{
				"refusal": "I cannot help with that",
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			choices, ok := resp["choices"].([]interface{})
			if !ok || len(choices) == 0 {
				t.Fatal("choices not found")
			}
			choice := choices[0].(map[string]interface{})
			msg, ok := choice["message"].(map[string]interface{})
			if !ok {
				t.Fatal("message not found")
			}
			if msg["refusal"] != "I cannot help with that" {
				t.Errorf("refusal: got %v, want 'I cannot help with that'", msg["refusal"])
			}
		},
	})

	// 测试用例4: tool_calls响应
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "tool_calls响应",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Use tool"}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content: "",
			ToolCalls: []plugin.ToolCall{
				{
					ID:   "call_789",
					Type: "function",
					Function: plugin.FunctionCall{
						Name:      "get_data",
						Arguments: `{"id": 1}`,
					},
				},
			},
			TokensUsed:   20,
			FinishReason: "tool_calls",
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "object", "chat.completion")
			choices := resp["choices"].([]interface{})
			choice := choices[0].(map[string]interface{})
			shared.AssertField(t, choice, "finish_reason", "tool_calls")
		},
	})

	// 测试用例5: service_tier
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "service_tier传递",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Test"}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content:      "Response",
			TokensUsed:   10,
			FinishReason: "stop",
			Metadata: map[string]interface{}{
				"service_tier": "priority",
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			shared.AssertField(t, resp, "service_tier", "priority")
		},
	})

	// 测试用例6: usage_details
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "usage_details传递",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Test"}]}`,
		MockResponse: &plugin.ProxyResponse{
			Content:      "Response",
			TokensUsed:   100,
			FinishReason: "stop",
			Metadata: map[string]interface{}{
				"prompt_tokens": 60,
				"prompt_tokens_details": &TokenDetails{
					CachedTokens: 10,
				},
				"completion_tokens_details": &CompletionTokenDetails{
					ReasoningTokens: 20,
				},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			usage := resp["usage"].(map[string]interface{})
			shared.AssertFieldExists(t, usage, "prompt_tokens_details")
			shared.AssertFieldExists(t, usage, "completion_tokens_details")
		},
	})
}

// runOpenAIStreamTests 流式响应测试
func runOpenAIStreamTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	// 测试用例1: 基础流式
	runner.RunStreamTest(shared.TestCase{
		Name: "基础流式响应",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Stream"}], "stream": true}`,
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
			// 最后一个应该是 [DONE]
			lastChunk := chunks[len(chunks)-1]
			if lastChunk != "[DONE]" {
				t.Errorf("last chunk: got %q, want [DONE]", lastChunk)
			}
		},
	})

	// 测试用例2: 流式usage事件
	runner.RunStreamTest(shared.TestCase{
		Name: "流式usage事件",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Usage"}], "stream": true}`,
		MockBackend: &shared.MockBackend{
			StreamChunks: []*plugin.StreamChunk{
				{Content: "Response", Done: false},
				{Content: "", Done: true, FinishReason: "stop", UsagePromptTokens: 50, UsageCompletionTokens: 30},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			chunks := resp["chunks"].([]string)
			// 应该有 usage 事件
			foundUsage := false
			for _, chunk := range chunks {
				if shared.ContainsAny(chunk, "prompt_tokens", "completion_tokens") {
					foundUsage = true
					break
				}
			}
			if !foundUsage {
				t.Error("usage event not found in stream")
			}
		},
	})

	// 测试用例3: 流式tool_calls
	runner.RunStreamTest(shared.TestCase{
		Name: "流式tool_calls",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Tool"}], "stream": true}`,
		MockBackend: &shared.MockBackend{
			StreamChunks: []*plugin.StreamChunk{
				{
					ToolCalls: []plugin.ToolCall{
						{ID: "call_1", Type: "function", Function: plugin.FunctionCall{Name: "search", Arguments: `{}`}},
					},
					Done: false,
				},
				{Content: "", Done: true, FinishReason: "tool_calls", TokensUsed: 10},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			chunks := resp["chunks"].([]string)
			foundToolCall := false
			for _, chunk := range chunks {
				if shared.ContainsAny(chunk, "tool_calls", "function") {
					foundToolCall = true
					break
				}
			}
			if !foundToolCall {
				t.Error("tool_calls not found in stream")
			}
		},
	})

	// 测试用例4: 流式reasoning_content
	runner.RunStreamTest(shared.TestCase{
		Name: "流式reasoning_content",
		RequestJSON: `{"model": "o1", "messages": [{"role": "user", "content": "Think"}], "stream": true}`,
		MockBackend: &shared.MockBackend{
			StreamChunks: []*plugin.StreamChunk{
				{ReasoningContent: "Let me think...", Done: false},
				{Content: "Answer", Done: false},
				{Content: "", Done: true, FinishReason: "stop", TokensUsed: 50},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			chunks := resp["chunks"].([]string)
			foundReasoning := false
			for _, chunk := range chunks {
				if shared.ContainsAny(chunk, "reasoning_content") {
					foundReasoning = true
					break
				}
			}
			if !foundReasoning {
				t.Error("reasoning_content not found in stream")
			}
		},
	})

	// 测试用例5: 流式prompt=0 completion>0
	runner.RunStreamTest(shared.TestCase{
		Name: "流式prompt_tokens为0",
		RequestJSON: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Test"}], "stream": true}`,
		MockBackend: &shared.MockBackend{
			StreamChunks: []*plugin.StreamChunk{
				{Content: "Response", Done: false},
				{Content: "", Done: true, FinishReason: "stop", UsagePromptTokens: 0, UsageCompletionTokens: 50},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			chunks := resp["chunks"].([]string)
			foundUsage := false
			for _, chunk := range chunks {
				if shared.ContainsAny(chunk, "completion_tokens") {
					foundUsage = true
					break
				}
			}
			if !foundUsage {
				t.Error("usage event should be sent even when prompt_tokens=0")
			}
		},
	})
}

// runOpenAIEdgeCaseTests 边界情况测试
func runOpenAIEdgeCaseTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	// 测试用例1: 空messages
	t.Run("空messages验证", func(t *testing.T) {
		protocol := &Protocol{}
		c, _ := shared.CreateTestGinContext(`{"model": "gpt-4", "messages": []}`)
		req, err := protocol.ParseRequest(c)
		if err == nil && len(req.Messages) == 0 {
			t.Log("empty messages allowed")
		}
	})

	// 测试用例2: 畸形JSON
	t.Run("畸形JSON处理", func(t *testing.T) {
		protocol := &Protocol{}
		c, _ := shared.CreateTestGinContext(`{invalid json`)
		_, err := protocol.ParseRequest(c)
		if err == nil {
			t.Error("expected error for malformed JSON")
		}
	})

	// 测试用例3: 缺少model
	t.Run("缺少model", func(t *testing.T) {
		protocol := &Protocol{}
		c, _ := shared.CreateTestGinContext(`{"messages": [{"role": "user", "content": "Hi"}]}`)
		req, err := protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Model != "" {
			t.Errorf("model should be empty, got %q", req.Model)
		}
	})

	// 测试用例4: MessageContent vision格式
	t.Run("MessageContent_vision格式", func(t *testing.T) {
		protocol := &Protocol{}
		reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":[{"type":"text","text":"What is in this image?"},{"type":"image_url","image_url":{"url":"https://example.com/image.png"}}]}]}`
		c, _ := shared.CreateTestGinContext(reqJSON)
		req, err := protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(req.Messages))
		}
		if req.Messages[0].Content == "" {
			t.Error("content should not be empty")
		}
	})

	// 测试用例5: tool_choice 对象形式各种变体
	t.Run("tool_choice_各种变体", func(t *testing.T) {
		testCases := []struct {
			name     string
			json     string
			expected string
		}{
			{"auto", `"tool_choice":"auto"`, "auto"},
			{"none", `"tool_choice":"none"`, "none"},
			{"required", `"tool_choice":"required"`, "required"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				protocol := &Protocol{}
				reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"Test"}],` + tc.json + `}`
				c, _ := shared.CreateTestGinContext(reqJSON)
				req, err := protocol.ParseRequest(c)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if req.ToolChoice != tc.expected {
					t.Errorf("tool_choice: got %v, want %v", req.ToolChoice, tc.expected)
				}
			})
		}
	})

	// 测试用例6: 验证错误响应格式
	t.Run("错误响应格式", func(t *testing.T) {
		protocol := &Protocol{}
		c, _ := shared.CreateTestGinContext(`{}`)
		errResp := &plugin.ErrorResponse{
			Type:    "invalid_request_error",
			Message: "Missing required field",
			Param:   "model",
			Code:    "invalid_request",
		}
		resp := &plugin.ProxyResponse{Error: errResp}
		err := protocol.HandleResponse(c, resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// 测试用例7: 并发安全性
	t.Run("并发安全性", func(t *testing.T) {
		protocol := &Protocol{}
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				c, _ := shared.CreateTestGinContext(`{"model":"gpt-4","messages":[{"role":"user","content":"Concurrent"}]}`)
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

	// 测试用例8: 超长内容
	t.Run("超长内容", func(t *testing.T) {
		protocol := &Protocol{}
		longContent := make([]byte, 100000)
		for i := range longContent {
			longContent[i] = 'a'
		}
		reqJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"` + string(longContent) + `"}]}`
		c, _ := shared.CreateTestGinContext(reqJSON)
		req, err := protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Messages[0].Content) != 100000 {
			t.Errorf("content length: got %d, want 100000", len(req.Messages[0].Content))
		}
	})
}
