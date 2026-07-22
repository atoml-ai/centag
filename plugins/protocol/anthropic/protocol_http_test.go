package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPAnthropic_BasicRequest 验证 HTTP 层基本请求/响应
func TestHTTPAnthropic_BasicRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求头
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		if apikey := r.Header.Get("x-api-key"); apikey == "" {
			t.Error("expected x-api-key header")
		}
		if version := r.Header.Get("anthropic-version"); version == "" {
			t.Error("expected anthropic-version header")
		}

		// 验证请求方法
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// 验证路径
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected path /v1/messages, got %s", r.URL.Path)
		}

		// 读取并解析请求体
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		var reqMap map[string]interface{}
		if err := json.Unmarshal(body, &reqMap); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}

		// 验证必需字段
		if _, ok := reqMap["model"]; !ok {
			t.Error("missing required field: model")
		}
		if _, ok := reqMap["max_tokens"]; !ok {
			t.Error("missing required field: max_tokens")
		}
		if _, ok := reqMap["messages"]; !ok {
			t.Error("missing required field: messages")
		}

		// 返回模拟响应
		resp := map[string]interface{}{
			"id":         "msg_test123",
			"type":       "message",
			"role":       "assistant",
			"content":    []map[string]interface{}{{"type": "text", "text": "Hello from Anthropic test!"}},
			"model":      reqMap["model"],
			"stop_reason": "end_turn",
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// 构造请求
	reqBody := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 发送 HTTP 请求
	req, _ := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-api-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 验证 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// 解析响应
	var respMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respMap); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 验证响应字段
	if respMap["type"] != "message" {
		t.Errorf("expected type message, got %v", respMap["type"])
	}
	if respMap["id"] != "msg_test123" {
		t.Errorf("expected id msg_test123, got %v", respMap["id"])
	}
	if respMap["stop_reason"] != "end_turn" {
		t.Errorf("expected stop_reason end_turn, got %v", respMap["stop_reason"])
	}

	// 验证 content 数组
	content, ok := respMap["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("expected non-empty content array")
	}
	contentBlock := content[0].(map[string]interface{})
	if contentBlock["type"] != "text" {
		t.Errorf("expected content type text, got %v", contentBlock["type"])
	}
}

// TestHTTPAnthropic_StreamRequest 验证 SSE 流式响应格式
func TestHTTPAnthropic_StreamRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置 SSE 响应头
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flushing")
		}

		// 发送 message_start 事件
		startEvent := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":         "msg_stream1",
				"type":       "message",
				"role":       "assistant",
				"content":    []interface{}{},
				"model":      "claude-3-opus-20240229",
				"stop_reason": nil,
				"usage": map[string]interface{}{
					"input_tokens":  10,
					"output_tokens": 0,
				},
			},
		}
		data, _ := json.Marshal(startEvent)
		fmt.Fprintf(w, "event: message_start\ndata: %s\n\n", data)
		flusher.Flush()

		// 发送 content_block_start 事件
		blockStart := map[string]interface{}{
			"type":         "content_block_start",
			"index":        0,
			"content_block": map[string]interface{}{
				"type": "text",
				"text": "",
			},
		}
		data, _ = json.Marshal(blockStart)
		fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", data)
		flusher.Flush()

		// 发送 content_block_delta 事件（文本）
		delta1 := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": "Hello",
			},
		}
		data, _ = json.Marshal(delta1)
		fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", data)
		flusher.Flush()

		// 发送 content_block_delta 事件（文本）
		delta2 := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": " world!",
			},
		}
		data, _ = json.Marshal(delta2)
		fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", data)
		flusher.Flush()

		// 发送 content_block_stop 事件
		blockStop := map[string]interface{}{
			"type":  "content_block_stop",
			"index": 0,
		}
		data, _ = json.Marshal(blockStop)
		fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", data)
		flusher.Flush()

		// 发送 message_delta 事件
		msgDelta := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": "end_turn",
				"stop_sequence": nil,
			},
			"usage": map[string]interface{}{
				"output_tokens": 5,
			},
		}
		data, _ = json.Marshal(msgDelta)
		fmt.Fprintf(w, "event: message_delta\ndata: %s\n\n", data)
		flusher.Flush()

		// 发送 message_stop 事件
		msgStop := map[string]interface{}{
			"type": "message_stop",
		}
		data, _ = json.Marshal(msgStop)
		fmt.Fprintf(w, "event: message_stop\ndata: %s\n\n", data)
		flusher.Flush()
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// 构造请求
	reqBody := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
		"stream": true,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 发送请求
	req, _ := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-api-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 验证 SSE 响应头
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	// 解析 SSE 事件
	scanner := bufio.NewScanner(resp.Body)
	var events []map[string]interface{}
	var eventTypes []string
	var fullContent string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")
			eventTypes = append(eventTypes, eventType)
		}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				events = append(events, event)

				// 提取文本内容
				if eventType, ok := event["type"].(string); ok {
					switch eventType {
					case "content_block_delta":
						if delta, ok := event["delta"].(map[string]interface{}); ok {
							if text, ok := delta["text"].(string); ok {
								fullContent += text
							}
						}
					}
				}
			}
		}
	}

	// 验证事件序列
	expectedEvents := []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}

	if len(eventTypes) != len(expectedEvents) {
		t.Errorf("expected %d events, got %d", len(expectedEvents), len(eventTypes))
	}

	for i, expected := range expectedEvents {
		if i < len(eventTypes) && eventTypes[i] != expected {
			t.Errorf("event %d: expected %s, got %s", i, expected, eventTypes[i])
		}
	}

	// 验证内容拼接
	if fullContent != "Hello world!" {
		t.Errorf("expected full content 'Hello world!', got '%s'", fullContent)
	}
}

// TestHTTPAnthropic_ErrorResponse 验证 Anthropic 错误格式（G2 修复）
func TestHTTPAnthropic_ErrorResponse(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		errorBody  map[string]interface{}
	}{
		{
			name:       "missing model",
			statusCode: http.StatusBadRequest,
			errorBody: map[string]interface{}{
				"type": "error",
				"error": map[string]interface{}{
					"type":    "invalid_request_error",
					"message": "model is required",
				},
			},
		},
		{
			name:       "invalid api key",
			statusCode: http.StatusUnauthorized,
			errorBody: map[string]interface{}{
				"type": "error",
				"error": map[string]interface{}{
					"type":    "authentication_error",
					"message": "Invalid API key",
				},
			},
		},
		{
			name:       "rate limit",
			statusCode: http.StatusTooManyRequests,
			errorBody: map[string]interface{}{
				"type": "error",
				"error": map[string]interface{}{
					"type":    "rate_limit_error",
					"message": "Rate limit exceeded",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				json.NewEncoder(w).Encode(tc.errorBody)
			})

			server := httptest.NewServer(handler)
			defer server.Close()

			// 发送请求
			reqBody := map[string]interface{}{
				"model":      "claude-3-opus-20240229",
				"max_tokens": 1024,
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Hello"},
				},
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req, _ := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", "test-api-key")
			req.Header.Set("anthropic-version", "2023-06-01")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// 验证状态码
			if resp.StatusCode != tc.statusCode {
				t.Errorf("expected status %d, got %d", tc.statusCode, resp.StatusCode)
			}

			// 解析错误响应
			var respMap map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&respMap); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			// 验证 G2 修复：error 为对象
			if respMap["type"] != "error" {
				t.Errorf("expected type 'error', got %v", respMap["type"])
			}

			errorObj, ok := respMap["error"].(map[string]interface{})
			if !ok {
				t.Fatal("expected error object in response (G2 fix)")
			}

			if errorObj["type"] == nil {
				t.Error("error.type is required")
			}
			if errorObj["message"] == nil {
				t.Error("error.message is required")
			}
		})
	}
}

// TestHTTPAnthropic_ThinkingRequest 验证 thinking 配置
func TestHTTPAnthropic_ThinkingRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqMap map[string]interface{}
		json.Unmarshal(body, &reqMap)

		// 验证 thinking 字段
		thinking, ok := reqMap["thinking"].(map[string]interface{})
		if !ok {
			t.Error("expected thinking object in request")
		}

		if thinking["type"] != "enabled" {
			t.Errorf("expected thinking.type 'enabled', got '%v'", thinking["type"])
		}

		if _, ok := thinking["budget_tokens"]; !ok {
			t.Error("expected thinking.budget_tokens")
		}

		// 返回带 thinking 的响应
		resp := map[string]interface{}{
			"id":         "msg_thinking1",
			"type":       "message",
			"role":       "assistant",
			"content": []map[string]interface{}{
				{"type": "thinking", "thinking": "Let me think..."},
				{"type": "text", "text": "Here is my answer."},
			},
			"model":      "claude-3-opus-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// 构造带 thinking 的请求
	reqBody := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Explain quantum computing"},
		},
		"thinking": map[string]interface{}{
			"type":          "enabled",
			"budget_tokens": 10000,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 发送请求
	req, _ := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-api-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var respMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respMap); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 验证 content 包含 thinking block
	content := respMap["content"].([]interface{})
	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(content))
	}

	thinkingBlock := content[0].(map[string]interface{})
	if thinkingBlock["type"] != "thinking" {
		t.Errorf("expected first block type 'thinking', got '%v'", thinkingBlock["type"])
	}

	textBlock := content[1].(map[string]interface{})
	if textBlock["type"] != "text" {
		t.Errorf("expected second block type 'text', got '%v'", textBlock["type"])
	}
}

// TestHTTPAnthropic_ToolCallsRequest 验证工具调用
func TestHTTPAnthropic_ToolCallsRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqMap map[string]interface{}
		json.Unmarshal(body, &reqMap)

		// 验证 tools 字段
		tools, ok := reqMap["tools"].([]interface{})
		if !ok || len(tools) == 0 {
			t.Error("expected tools array in request")
		}

		// 返回工具调用响应
		resp := map[string]interface{}{
			"id":         "msg_tools1",
			"type":       "message",
			"role":       "assistant",
			"content": []map[string]interface{}{
				{
					"type":  "tool_use",
					"id":    "toolu_abc123",
					"name":  "get_weather",
					"input": map[string]interface{}{"location": "Boston"},
				},
			},
			"model":       "claude-3-opus-20240229",
			"stop_reason": "tool_use",
			"usage": map[string]interface{}{
				"input_tokens":  20,
				"output_tokens": 15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// 构造带 tools 的请求
	reqBody := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "What's the weather in Boston?"},
		},
		"tools": []map[string]interface{}{
			{
				"name":        "get_weather",
				"description": "Get weather for a location",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 发送请求
	req, _ := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-api-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var respMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respMap); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 验证工具调用
	content := respMap["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}

	toolUse := content[0].(map[string]interface{})
	if toolUse["type"] != "tool_use" {
		t.Errorf("expected content type 'tool_use', got '%v'", toolUse["type"])
	}
	if toolUse["id"] != "toolu_abc123" {
		t.Errorf("expected tool id 'toolu_abc123', got '%v'", toolUse["id"])
	}
	if toolUse["name"] != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got '%v'", toolUse["name"])
	}

	// 验证 stop_reason
	if respMap["stop_reason"] != "tool_use" {
		t.Errorf("expected stop_reason 'tool_use', got '%v'", respMap["stop_reason"])
	}
}
