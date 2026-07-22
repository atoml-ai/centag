package openai

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

// TestHTTPOpenAI_BasicRequest 验证 HTTP 层基本请求/响应
func TestHTTPOpenAI_BasicRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 Content-Type
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		// 验证请求方法
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// 验证路径
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected path /v1/chat/completions, got %s", r.URL.Path)
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
		if _, ok := reqMap["messages"]; !ok {
			t.Error("missing required field: messages")
		}

		// 返回模拟响应
		resp := map[string]interface{}{
			"id":      "chatcmpl-test123",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   reqMap["model"],
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello from test server!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// 构造请求
	reqBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 发送 HTTP 请求
	resp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 验证 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// 验证响应 Content-Type
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	// 解析响应
	var respMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respMap); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 验证响应字段
	if respMap["object"] != "chat.completion" {
		t.Errorf("expected object chat.completion, got %v", respMap["object"])
	}
	if respMap["id"] != "chatcmpl-test123" {
		t.Errorf("expected id chatcmpl-test123, got %v", respMap["id"])
	}

	choices, ok := respMap["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatal("expected non-empty choices array")
	}

	usage, ok := respMap["usage"].(map[string]interface{})
	if !ok {
		t.Fatal("expected usage object")
	}
	if usage["total_tokens"] != float64(15) {
		t.Errorf("expected total_tokens 15, got %v", usage["total_tokens"])
	}
}

// TestHTTPOpenAI_StreamRequest 验证 SSE 流式响应格式
func TestHTTPOpenAI_StreamRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 stream 参数
		body, _ := io.ReadAll(r.Body)
		var reqMap map[string]interface{}
		json.Unmarshal(body, &reqMap)

		if stream, ok := reqMap["stream"].(bool); !ok || !stream {
			t.Error("expected stream=true in request")
		}

		// 设置 SSE 响应头
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flushing")
		}

		// 发送第一个 chunk（role）
		chunk1 := map[string]interface{}{
			"id":      "chatcmpl-stream1",
			"object":  "chat.completion.chunk",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"role": "assistant",
					},
					"finish_reason": nil,
				},
			},
		}
		data, _ := json.Marshal(chunk1)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// 发送第二个 chunk（content）
		chunk2 := map[string]interface{}{
			"id":      "chatcmpl-stream1",
			"object":  "chat.completion.chunk",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": "Hello",
					},
					"finish_reason": nil,
				},
			},
		}
		data, _ = json.Marshal(chunk2)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// 发送第三个 chunk（content）
		chunk3 := map[string]interface{}{
			"id":      "chatcmpl-stream1",
			"object":  "chat.completion.chunk",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": " world!",
					},
					"finish_reason": nil,
				},
			},
		}
		data, _ = json.Marshal(chunk3)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// 发送结束 chunk
		chunkDone := map[string]interface{}{
			"id":      "chatcmpl-stream1",
			"object":  "chat.completion.chunk",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"delta":         map[string]interface{}{},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		data, _ = json.Marshal(chunkDone)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// 发送 [DONE]
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// 构造请求
	reqBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
		"stream": true,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 发送请求
	resp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", bytes.NewReader(bodyBytes))
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
	var events []string
	var fullContent string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				events = append(events, "[DONE]")
				break
			}
			events = append(events, data)

			// 解析 chunk 提取 content
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							if content, ok := delta["content"].(string); ok {
								fullContent += content
							}
						}
					}
				}
			}
		}
	}

	// 验证事件数量（4 个 chunk + 1 个 [DONE]）
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}

	// 验证最终事件是 [DONE]
	if events[len(events)-1] != "[DONE]" {
		t.Errorf("expected last event to be [DONE], got %s", events[len(events)-1])
	}

	// 验证内容拼接
	if fullContent != "Hello world!" {
		t.Errorf("expected full content 'Hello world!', got '%s'", fullContent)
	}
}

// TestHTTPOpenAI_ErrorResponse 验证错误响应格式
func TestHTTPOpenAI_ErrorResponse(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		errorBody  map[string]interface{}
	}{
		{
			name:       "missing model",
			statusCode: http.StatusBadRequest,
			errorBody: map[string]interface{}{
				"error": map[string]interface{}{
					"message": "model is required",
					"type":    "invalid_request_error",
					"code":    "missing_required_parameter",
					"param":   "model",
				},
			},
		},
		{
			name:       "invalid api key",
			statusCode: http.StatusUnauthorized,
			errorBody: map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid API key",
					"type":    "invalid_request_error",
					"code":    "invalid_api_key",
				},
			},
		},
		{
			name:       "rate limit",
			statusCode: http.StatusTooManyRequests,
			errorBody: map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Rate limit exceeded",
					"type":    "rate_limit_error",
					"code":    "rate_limit_exceeded",
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
				"model": "gpt-4",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Hello"},
				},
			}
			bodyBytes, _ := json.Marshal(reqBody)

			resp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", bytes.NewReader(bodyBytes))
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

			// 验证错误结构
			errorObj, ok := respMap["error"].(map[string]interface{})
			if !ok {
				t.Fatal("expected error object in response")
			}

			if errorObj["message"] == nil {
				t.Error("error.message is required")
			}
			if errorObj["type"] == nil {
				t.Error("error.type is required")
			}
		})
	}
}

// TestHTTPOpenAI_ToolCallsRequest 验证工具调用请求/响应
func TestHTTPOpenAI_ToolCallsRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqMap map[string]interface{}
		json.Unmarshal(body, &reqMap)

		// 验证 tools 字段
		tools, ok := reqMap["tools"].([]interface{})
		if !ok || len(tools) == 0 {
			t.Error("expected tools array in request")
		}

		// 验证 tool_choice 字段
		toolChoice, ok := reqMap["tool_choice"].(string)
		if !ok {
			t.Error("expected tool_choice string in request")
		}
		if toolChoice != "auto" {
			t.Errorf("expected tool_choice 'auto', got '%s'", toolChoice)
		}

		// 返回工具调用响应
		resp := map[string]interface{}{
			"id":      "chatcmpl-tools1",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role": "assistant",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_abc123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_weather",
									"arguments": `{"location":"Boston"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     20,
				"completion_tokens": 15,
				"total_tokens":      35,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// 构造带 tools 的请求
	reqBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "What's the weather in Boston?"},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "get_weather",
					"description": "Get weather for a location",
					"parameters": map[string]interface{}{
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
		},
		"tool_choice": "auto",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 发送请求
	resp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", bytes.NewReader(bodyBytes))
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
	choices := respMap["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	message := choice["message"].(map[string]interface{})
	toolCalls := message["tool_calls"].([]interface{})

	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	toolCall := toolCalls[0].(map[string]interface{})
	if toolCall["id"] != "call_abc123" {
		t.Errorf("expected tool call id 'call_abc123', got '%v'", toolCall["id"])
	}

	function := toolCall["function"].(map[string]interface{})
	if function["name"] != "get_weather" {
		t.Errorf("expected function name 'get_weather', got '%v'", function["name"])
	}
}

// TestHTTPOpenAI_ConcurrentRequests 验证并发请求安全性
func TestHTTPOpenAI_ConcurrentRequests(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqMap map[string]interface{}
		json.Unmarshal(body, &reqMap)

		// 模拟处理延迟
		resp := map[string]interface{}{
			"id":      "chatcmpl-concurrent",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   reqMap["model"],
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     5,
				"completion_tokens": 3,
				"total_tokens":      8,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// 并发发送 10 个请求
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			reqBody := map[string]interface{}{
				"model": "gpt-4",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Hello"},
				},
			}
			bodyBytes, _ := json.Marshal(reqBody)

			resp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				t.Errorf("request %d failed: %v", index, err)
				done <- false
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("request %d: expected status 200, got %d", index, resp.StatusCode)
				done <- false
				return
			}

			var respMap map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&respMap); err != nil {
				t.Errorf("request %d: failed to decode response: %v", index, err)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// 等待所有请求完成
	successCount := 0
	for i := 0; i < numRequests; i++ {
		if <-done {
			successCount++
		}
	}

	if successCount != numRequests {
		t.Errorf("expected %d successful requests, got %d", numRequests, successCount)
	}
}
