package pipeline

import (
	"context"
	"testing"
)

func TestMergeToolCalls(t *testing.T) {
	t.Run("no_tool_calls", func(t *testing.T) {
		nodeRegistry := NewNodeRegistry()
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "test"})
		result := engine.mergeToolCalls(execCtx)

		if len(result) != 0 {
			t.Errorf("expected empty tool_calls, got %d", len(result))
		}
	})

	t.Run("single_node_with_tool_calls", func(t *testing.T) {
		nodeRegistry := NewNodeRegistry()
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "test"})
		execCtx.SetResult("node1", &NodeOutput{
			ToolCalls: []ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"test.txt"}`,
					},
				},
			},
		})

		result := engine.mergeToolCalls(execCtx)

		if len(result) != 1 {
			t.Errorf("expected 1 tool_call, got %d", len(result))
		}
		if result[0].ID != "call_1" {
			t.Errorf("expected tool_call ID call_1, got %s", result[0].ID)
		}
	})

	t.Run("multiple_nodes_with_tool_calls", func(t *testing.T) {
		nodeRegistry := NewNodeRegistry()
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "test"})
		execCtx.SetResult("node1", &NodeOutput{
			ToolCalls: []ToolCall{
				{ID: "call_1", Type: "function", Function: FunctionCall{Name: "tool_a"}},
				{ID: "call_2", Type: "function", Function: FunctionCall{Name: "tool_b"}},
			},
		})
		execCtx.SetResult("node2", &NodeOutput{
			ToolCalls: []ToolCall{
				{ID: "call_3", Type: "function", Function: FunctionCall{Name: "tool_c"}},
			},
		})
		execCtx.SetResult("node3", &NodeOutput{}) // no tool_calls

		result := engine.mergeToolCalls(execCtx)

		if len(result) != 3 {
			t.Errorf("expected 3 tool_calls, got %d", len(result))
		}
	})

	t.Run("node_with_nil_output", func(t *testing.T) {
		nodeRegistry := NewNodeRegistry()
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "test"})
		execCtx.SetResult("node1", nil)

		result := engine.mergeToolCalls(execCtx)

		if len(result) != 0 {
			t.Errorf("expected empty tool_calls, got %d", len(result))
		}
	})
}

func TestToolCallInjectorNode(t *testing.T) {
	// 测试1: 基本工具调用注入
	t.Run("basic_tool_call_injection", func(t *testing.T) {
		config := NodeConfig{
			CustomConfig: map[string]interface{}{
				"tool_calls": []map[string]interface{}{
					{
						"id":   "test_tool",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "read_file",
							"arguments": `{"path": "test.txt"}`,
						},
					},
				},
			},
		}

		node, err := NewToolCallInjectorNode(config)
		if err != nil {
			t.Fatalf("NewToolCallInjectorNode() error = %v", err)
		}

		// 验证节点类型
		if node.Type() != NodeTypeToolCallInjector {
			t.Errorf("node.Type() = %v, want %v", node.Type(), NodeTypeToolCallInjector)
		}

		// 验证节点配置
		if err := node.Validate(); err != nil {
			t.Errorf("node.Validate() error = %v", err)
		}
	})

	// 测试2: 条件判断
	t.Run("condition_evaluation", func(t *testing.T) {
		config := NodeConfig{
			CustomConfig: map[string]interface{}{
				"condition": "true",
				"tool_calls": []map[string]interface{}{
					{
						"id":   "conditional_tool",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "write_file",
							"arguments": `{"path": "output.txt", "content": "test"}`,
						},
					},
				},
			},
		}

		node, err := NewToolCallInjectorNode(config)
		if err != nil {
			t.Fatalf("NewToolCallInjectorNode() error = %v", err)
		}

		// 执行节点
		input := &NodeInput{
			Content: "test input",
		}

		output, err := node.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("node.Execute() error = %v", err)
		}

		// 验证输出包含tool_calls
		if len(output.ToolCalls) == 0 {
			t.Errorf("expected tool_calls, got none")
		}

		// 验证tool_call内容
		if len(output.ToolCalls) > 0 {
			tc := output.ToolCalls[0]
			if tc.Function.Name != "write_file" {
				t.Errorf("tool_call.Function.Name = %v, want write_file", tc.Function.Name)
			}
		}
	})

	// 测试3: 条件不满足时跳过
	t.Run("condition_not_satisfied", func(t *testing.T) {
		config := NodeConfig{
			CustomConfig: map[string]interface{}{
				"condition": "false",
				"tool_calls": []map[string]interface{}{
					{
						"id":   "skipped_tool",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "delete_file",
							"arguments": `{"path": "temp.txt"}`,
						},
					},
				},
			},
		}

		node, err := NewToolCallInjectorNode(config)
		if err != nil {
			t.Fatalf("NewToolCallInjectorNode() error = %v", err)
		}

		// 执行节点
		input := &NodeInput{
			Content: "test input",
		}

		output, err := node.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("node.Execute() error = %v", err)
		}

		// 验证输出不包含tool_calls
		if len(output.ToolCalls) != 0 {
			t.Errorf("expected no tool_calls, got %d", len(output.ToolCalls))
		}
	})

	// 测试4: 验证失败
	t.Run("validation_error", func(t *testing.T) {
		config := NodeConfig{
			CustomConfig: map[string]interface{}{
				"tool_calls": []map[string]interface{}{},
			},
		}

		_, err := NewToolCallInjectorNode(config)
		if err == nil {
			t.Errorf("expected error for empty tool_calls, got nil")
		}
	})

	// 测试5: 多个tool_calls
	t.Run("multiple_tool_calls", func(t *testing.T) {
		config := NodeConfig{
			CustomConfig: map[string]interface{}{
				"tool_calls": []map[string]interface{}{
					{
						"id":   "tool_1",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "read_file",
							"arguments": `{"path": "file1.txt"}`,
						},
					},
					{
						"id":   "tool_2",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "write_file",
							"arguments": `{"path": "file2.txt", "content": "data"}`,
						},
					},
				},
			},
		}

		node, err := NewToolCallInjectorNode(config)
		if err != nil {
			t.Fatalf("NewToolCallInjectorNode() error = %v", err)
		}

		input := &NodeInput{Content: "test"}
		output, err := node.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("node.Execute() error = %v", err)
		}

		if len(output.ToolCalls) != 2 {
			t.Errorf("expected 2 tool_calls, got %d", len(output.ToolCalls))
		}
	})

	// 测试6: 空condition（默认注入）
	t.Run("empty_condition_always_injects", func(t *testing.T) {
		config := NodeConfig{
			CustomConfig: map[string]interface{}{
				"tool_calls": []map[string]interface{}{
					{
						"id":   "default_tool",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "get_time",
							"arguments": "{}",
						},
					},
				},
			},
		}

		node, err := NewToolCallInjectorNode(config)
		if err != nil {
			t.Fatalf("NewToolCallInjectorNode() error = %v", err)
		}

		input := &NodeInput{Content: "test"}
		output, err := node.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("node.Execute() error = %v", err)
		}

		if len(output.ToolCalls) != 1 {
			t.Errorf("expected 1 tool_call, got %d", len(output.ToolCalls))
		}
	})

	// 测试7: 工具调用ID格式正确
	t.Run("tool_call_id_format", func(t *testing.T) {
		config := NodeConfig{
			CustomConfig: map[string]interface{}{
				"tool_calls": []map[string]interface{}{
					{
						"id":   "format_test",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "test_tool",
							"arguments": "{}",
						},
					},
				},
			},
		}

		node, _ := NewToolCallInjectorNode(config)
		input := &NodeInput{Content: "test"}

		output, _ := node.Execute(context.Background(), input)

		if len(output.ToolCalls) > 0 {
			id := output.ToolCalls[0].ID
			// 格式: pipeline_{id}_{timestamp}_{random}
			if len(id) < 15 {
				t.Errorf("tool_call ID too short: %s", id)
			}
			// 检查前缀
			prefix := "pipeline_format_test_"
			if len(id) < len(prefix) || id[:len(prefix)] != prefix {
				t.Errorf("tool_call ID should start with '%s', got %s", prefix, id)
			}
		}
	})

	// 测试8: 输出内容透传
	t.Run("content_passthrough", func(t *testing.T) {
		config := NodeConfig{
			CustomConfig: map[string]interface{}{
				"tool_calls": []map[string]interface{}{
					{
						"id":   "passthrough_test",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "test",
							"arguments": "{}",
						},
					},
				},
			},
		}

		node, _ := NewToolCallInjectorNode(config)
		input := &NodeInput{Content: "original content"}

		output, err := node.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("node.Execute() error = %v", err)
		}

		if output.Content != "original content" {
			t.Errorf("expected content 'original content', got %v", output.Content)
		}
	})
}

func TestBuiltinNodesRegistration(t *testing.T) {
	// 测试ToolCallInjector节点是否正确注册
	registry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(registry); err != nil {
		t.Fatalf("RegisterBuiltinNodes() error = %v", err)
	}

	// 验证ToolCallInjector已注册
	if !registry.IsRegistered(NodeTypeToolCallInjector) {
		t.Errorf("ToolCallInjector not registered")
	}

	// 验证可以创建节点
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"tool_calls": []map[string]interface{}{
				{
					"id":   "test",
					"type": "function",
					"function": map[string]interface{}{
						"name":      "test",
						"arguments": "{}",
					},
				},
			},
		},
	}

	node, err := registry.Create(NodeTypeToolCallInjector, config)
	if err != nil {
		t.Fatalf("registry.Create() error = %v", err)
	}

	if node.Type() != NodeTypeToolCallInjector {
		t.Errorf("node.Type() = %v, want %v", node.Type(), NodeTypeToolCallInjector)
	}
}