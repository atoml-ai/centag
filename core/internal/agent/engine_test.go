package agent

import (
	"testing"
)

func TestDefaultAgentEngine(t *testing.T) {
	// 创建Agent引擎
	engine := NewDefaultAgentEngine("http://localhost:20060")

	// 测试1: 基本创建
	t.Run("create_engine", func(t *testing.T) {
		if engine == nil {
			t.Fatal("engine is nil")
		}
		if engine.proxyURL != "http://localhost:20060" {
			t.Errorf("proxyURL = %v, want http://localhost:20060", engine.proxyURL)
		}
	})

	// 测试2: 注册工具
	t.Run("register_tool", func(t *testing.T) {
		tool := ToolDefinition{
			Name:        "test_tool",
			Description: "A test tool",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param1": map[string]interface{}{
						"type": "string",
					},
				},
			},
		}

		err := engine.RegisterTool(tool)
		if err != nil {
			t.Fatalf("RegisterTool() error = %v", err)
		}

		// 验证工具已注册
		tools := engine.GetTools()
		if len(tools) != 1 {
			t.Errorf("GetTools() returned %d tools, want 1", len(tools))
		}
		if tools[0].Name != "test_tool" {
			t.Errorf("tool.Name = %v, want test_tool", tools[0].Name)
		}
	})

	// 测试3: 注销工具
	t.Run("unregister_tool", func(t *testing.T) {
		err := engine.UnregisterTool("test_tool")
		if err != nil {
			t.Fatalf("UnregisterTool() error = %v", err)
		}

		// 验证工具已注销
		tools := engine.GetTools()
		if len(tools) != 0 {
			t.Errorf("GetTools() returned %d tools, want 0", len(tools))
		}
	})

	// 测试4: 注销不存在的工具
	t.Run("unregister_nonexistent_tool", func(t *testing.T) {
		err := engine.UnregisterTool("nonexistent_tool")
		if err == nil {
			t.Errorf("UnregisterTool() expected error, got nil")
		}
	})

	// 测试5: 取消不存在的请求
	t.Run("cancel_nonexistent_request", func(t *testing.T) {
		err := engine.Cancel("nonexistent_request")
		if err == nil {
			t.Errorf("Cancel() expected error, got nil")
		}
	})
}

func TestTUIRenderer(t *testing.T) {
	renderer := NewTUIRenderer("default", true)

	// 测试1: 渲染用户消息
	t.Run("render_user_message", func(t *testing.T) {
		msg := Message{
			Role:    "user",
			Content: "Hello, world!",
		}

		result := renderer.RenderMessage(msg)
		if result == "" {
			t.Errorf("RenderMessage() returned empty string")
		}
	})

	// 测试2: 渲染助手消息
	t.Run("render_assistant_message", func(t *testing.T) {
		msg := Message{
			Role:    "assistant",
			Content: "I can help you with that.",
		}

		result := renderer.RenderMessage(msg)
		if result == "" {
			t.Errorf("RenderMessage() returned empty string")
		}
	})

	// 测试3: 渲染工具调用
	t.Run("render_tool_call", func(t *testing.T) {
		call := ToolCallInfo{
			ID:   "call_123",
			Type: "function",
			Function: FunctionCallInfo{
				Name:      "read_file",
				Arguments: `{"path": "test.txt"}`,
			},
		}

		result := renderer.RenderToolCall(call)
		if result == "" {
			t.Errorf("RenderToolCall() returned empty string")
		}
	})

	// 测试4: 渲染工具结果
	t.Run("render_tool_result", func(t *testing.T) {
		result := ToolResult{
			ToolCallID: "call_123",
			Content:    "file content here",
			IsError:    false,
		}

		rendered := renderer.RenderToolResult(result)
		if rendered == "" {
			t.Errorf("RenderToolResult() returned empty string")
		}
	})

	// 测试5: 渲染错误
	t.Run("render_error", func(t *testing.T) {
		err := &AgentError{
			Code:    "TEST_ERROR",
			Message: "This is a test error",
		}

		result := renderer.RenderError(err)
		if result == "" {
			t.Errorf("RenderError() returned empty string")
		}
	})

	// 测试6: 渲染进度
	t.Run("render_progress", func(t *testing.T) {
		progress := AgentProgressInfo{
			CurrentTurn: 1,
			MaxTurns:    10,
			Status:      "calling LLM",
		}

		result := renderer.RenderProgress(progress)
		if result == "" {
			t.Errorf("RenderProgress() returned empty string")
		}
	})
}

func TestWebRenderer(t *testing.T) {
	renderer := NewWebRenderer("json")

	// 测试1: 渲染消息
	t.Run("render_message", func(t *testing.T) {
		msg := Message{
			Role:    "user",
			Content: "Hello, world!",
		}

		result := renderer.RenderMessage(msg)
		if result == "" {
			t.Errorf("RenderMessage() returned empty string")
		}
	})

	// 测试2: 渲染工具调用
	t.Run("render_tool_call", func(t *testing.T) {
		call := ToolCallInfo{
			ID:   "call_123",
			Type: "function",
			Function: FunctionCallInfo{
				Name:      "read_file",
				Arguments: `{"path": "test.txt"}`,
			},
		}

		result := renderer.RenderToolCall(call)
		if result == "" {
			t.Errorf("RenderToolCall() returned empty string")
		}
	})
}

func TestAgentEventTypes(t *testing.T) {
	// 验证事件类型常量
	eventTypes := []EventType{
		EventAgentStart,
		EventAgentEnd,
		EventMessageUpdate,
		EventToolStart,
		EventToolEnd,
		EventToolPermissionRequest,
		EventToolPermissionResponse,
		EventError,
		EventProgress,
	}

	for _, eventType := range eventTypes {
		if eventType == "" {
			t.Errorf("EventType is empty")
		}
	}
}

func TestAgentRequest(t *testing.T) {
	req := &AgentRequest{
		RequestID: "test_request_123",
		Messages: []Message{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		Model:      "gpt-4",
		MaxTurns:   10,
		Timeout:    60,
	}

	if req.RequestID != "test_request_123" {
		t.Errorf("RequestID = %v, want test_request_123", req.RequestID)
	}

	if len(req.Messages) != 1 {
		t.Errorf("Messages length = %v, want 1", len(req.Messages))
	}

	if req.Model != "gpt-4" {
		t.Errorf("Model = %v, want gpt-4", req.Model)
	}
}

func TestToolDefinition(t *testing.T) {
	tool := ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type": "string",
				},
			},
		},
		IsReadOnly: true,
	}

	if tool.Name != "test_tool" {
		t.Errorf("Name = %v, want test_tool", tool.Name)
	}

	if tool.Description != "A test tool" {
		t.Errorf("Description = %v, want A test tool", tool.Description)
	}

	if !tool.IsReadOnly {
		t.Errorf("IsReadOnly = %v, want true", tool.IsReadOnly)
	}
}