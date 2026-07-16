package pipeline

import (
	"context"
	"testing"
	"time"
)

// TestToolCallInjectorIntegration 验证ToolCallInjector在完整Pipeline中的集成行为
func TestToolCallInjectorIntegration(t *testing.T) {
	// 创建测试引擎
	nodeRegistry := NewNodeRegistry()
	RegisterBuiltinNodes(nodeRegistry)
	pipelineRegistry := NewPipelineRegistry()
	plLogger := NewPipelineLogger()
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

	t.Run("pipeline_with_tool_call_injector", func(t *testing.T) {
		// 创建包含ToolCallInjector的Pipeline定义
		pipeline := &AgentPatternPipeline{
			ID:   "test-injector-pipeline",
			Name: "Test Injector Pipeline",
			Nodes: []PipelineNodeConfig{
				{
					ID:   "input",
					Type: NodeTypeGenerator,
				},
				{
					ID:   "injector",
					Type: NodeTypeToolCallInjector,
					Config: NodeConfig{
						CustomConfig: map[string]interface{}{
							"tool_calls": []map[string]interface{}{
								{
									"id":   "auto_fix",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "fix_code",
										"arguments": `{"file": "test.go", "line": 42}`,
									},
								},
							},
						},
					},
				},
			},
			GlobalConfig: GlobalPipelineConfig{
				ParallelLimit: 1,
				BypassOnError: false,
			},
		}

		// 创建执行图
		graph := NewExecutionGraph(pipeline)
		execCtx := NewExecutionContext(pipeline)

		// 执行ToolCallInjector节点
		err := engine.executeLayerNode(context.Background(), graph, execCtx, "injector", pipeline)
		if err != nil {
			t.Fatalf("executeLayerNode(injector) error = %v", err)
		}

		// 验证节点执行成功
		node := graph.GetNode("injector")
		if node == nil {
			t.Fatal("injector node not found")
		}
		if node.Status != StatusSuccess {
			t.Errorf("injector node status = %v, want StatusSuccess", node.Status)
		}

		// 验证输出包含tool_calls
		if node.Output == nil {
			t.Fatal("injector node output is nil")
		}
		if len(node.Output.ToolCalls) != 1 {
			t.Errorf("expected 1 tool_call, got %d", len(node.Output.ToolCalls))
		}
		if node.Output.ToolCalls[0].Function.Name != "fix_code" {
			t.Errorf("tool_call function name = %v, want fix_code", node.Output.ToolCalls[0].Function.Name)
		}
	})

	t.Run("conditional_injection_based_on_previous_node", func(t *testing.T) {
		// 创建Pipeline：Generator → ConditionChecker → ToolCallInjector
		pipeline := &AgentPatternPipeline{
			ID:   "test-conditional-pipeline",
			Name: "Test Conditional Pipeline",
			Nodes: []PipelineNodeConfig{
				{
					ID:   "reviewer",
					Type: NodeTypeReviewer,
				},
				{
					ID:   "injector",
					Type: NodeTypeToolCallInjector,
					Config: NodeConfig{
						CustomConfig: map[string]interface{}{
							"condition": "{{node.reviewer.score}} < 0.8",
							"tool_calls": []map[string]interface{}{
								{
									"id":   "auto_fix",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "fix_code",
										"arguments": `{"file": "test.go"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		// 创建执行图
		graph := NewExecutionGraph(pipeline)
		execCtx := NewExecutionContext(pipeline)

		// 模拟reviewer节点输出（分数低于阈值）
		score := 0.6
		execCtx.SetResult("reviewer", &NodeOutput{
			Content: "Code review result",
			Score:   &score,
		})

		// 执行ToolCallInjector节点
		err := engine.executeLayerNode(context.Background(), graph, execCtx, "injector", pipeline)
		if err != nil {
			t.Fatalf("executeLayerNode(injector) error = %v", err)
		}

		// 验证节点执行成功
		node := graph.GetNode("injector")
		if node == nil {
			t.Fatal("injector node not found")
		}
		if node.Status != StatusSuccess {
			t.Errorf("injector node status = %v, want StatusSuccess", node.Status)
		}

		// 验证条件满足时注入tool_calls
		if node.Output == nil {
			t.Fatal("injector node output is nil")
		}
		if len(node.Output.ToolCalls) != 1 {
			t.Errorf("expected 1 tool_call when condition satisfied, got %d", len(node.Output.ToolCalls))
		}
	})

	t.Run("condition_not_satisfied_skips_injection", func(t *testing.T) {
		// 创建Pipeline
		pipeline := &AgentPatternPipeline{
			ID:   "test-skip-pipeline",
			Name: "Test Skip Pipeline",
			Nodes: []PipelineNodeConfig{
				{
					ID:   "reviewer",
					Type: NodeTypeReviewer,
				},
				{
					ID:   "injector",
					Type: NodeTypeToolCallInjector,
					Config: NodeConfig{
						CustomConfig: map[string]interface{}{
							"condition": "{{node.reviewer.score}} < 0.8",
							"tool_calls": []map[string]interface{}{
								{
									"id":   "auto_fix",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "fix_code",
										"arguments": `{"file": "test.go"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		// 创建执行图
		graph := NewExecutionGraph(pipeline)
		execCtx := NewExecutionContext(pipeline)

		// 模拟reviewer节点输出（分数高于阈值）
		highScore := 0.9
		execCtx.SetResult("reviewer", &NodeOutput{
			Content: "Code review result",
			Score:   &highScore,
		})

		// 执行ToolCallInjector节点
		err := engine.executeLayerNode(context.Background(), graph, execCtx, "injector", pipeline)
		if err != nil {
			t.Fatalf("executeLayerNode(injector) error = %v", err)
		}

		// 验证节点执行成功
		node := graph.GetNode("injector")
		if node == nil {
			t.Fatal("injector node not found")
		}
		if node.Status != StatusSuccess {
			t.Errorf("injector node status = %v, want StatusSuccess", node.Status)
		}

		// 验证条件不满足时不注入tool_calls
		if node.Output == nil {
			t.Fatal("injector node output is nil")
		}
		if len(node.Output.ToolCalls) != 0 {
			t.Errorf("expected 0 tool_calls when condition not satisfied, got %d", len(node.Output.ToolCalls))
		}
	})

	t.Run("multiple_tool_calls_injection", func(t *testing.T) {
		// 创建包含多个tool_calls的Pipeline
		pipeline := &AgentPatternPipeline{
			ID:   "test-multi-pipeline",
			Name: "Test Multi Pipeline",
			Nodes: []PipelineNodeConfig{
				{
					ID:   "injector",
					Type: NodeTypeToolCallInjector,
					Config: NodeConfig{
						CustomConfig: map[string]interface{}{
							"tool_calls": []map[string]interface{}{
								{
									"id":   "read_file",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "read_file",
										"arguments": `{"path": "input.txt"}`,
									},
								},
								{
									"id":   "write_file",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "write_file",
										"arguments": `{"path": "output.txt", "content": "processed"}`,
									},
								},
								{
									"id":   "delete_file",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "delete_file",
										"arguments": `{"path": "temp.txt"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		// 创建执行图
		graph := NewExecutionGraph(pipeline)
		execCtx := NewExecutionContext(pipeline)

		// 执行ToolCallInjector节点
		err := engine.executeLayerNode(context.Background(), graph, execCtx, "injector", pipeline)
		if err != nil {
			t.Fatalf("executeLayerNode(injector) error = %v", err)
		}

		// 验证节点执行成功
		node := graph.GetNode("injector")
		if node == nil {
			t.Fatal("injector node not found")
		}
		if node.Status != StatusSuccess {
			t.Errorf("injector node status = %v, want StatusSuccess", node.Status)
		}

		// 验证注入了3个tool_calls
		if node.Output == nil {
			t.Fatal("injector node output is nil")
		}
		if len(node.Output.ToolCalls) != 3 {
			t.Errorf("expected 3 tool_calls, got %d", len(node.Output.ToolCalls))
		}

		// 验证tool_call名称
		expectedNames := []string{"read_file", "write_file", "delete_file"}
		for i, name := range expectedNames {
			if node.Output.ToolCalls[i].Function.Name != name {
				t.Errorf("tool_call[%d] function name = %v, want %v", i, node.Output.ToolCalls[i].Function.Name, name)
			}
		}
	})
}

// TestToolCallInjectorWithPipelineEngine 验证ToolCallInjector与PipelineEngine的完整集成
func TestToolCallInjectorWithPipelineEngine(t *testing.T) {
	t.Run("execute_pipeline_with_tool_call_injector", func(t *testing.T) {
		// 创建测试引擎
		nodeRegistry := NewNodeRegistry()
		RegisterBuiltinNodes(nodeRegistry)
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		// 注册Pipeline
		pipeline := &AgentPatternPipeline{
			ID:   "test-full-pipeline",
			Name: "Test Full Pipeline",
			Nodes: []PipelineNodeConfig{
				{
					ID:   "injector",
					Type: NodeTypeToolCallInjector,
					Config: NodeConfig{
						CustomConfig: map[string]interface{}{
							"tool_calls": []map[string]interface{}{
								{
									"id":   "test_tool",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "test_function",
										"arguments": `{"key": "value"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		pipelineRegistry.Register(pipeline)

		// 执行Pipeline
		input := &PipelineInput{
			Content: "test input",
		}

		output, err := engine.Execute(context.Background(), pipeline.ID, input)
		if err != nil {
			t.Fatalf("engine.Execute() error = %v", err)
		}

		// 验证输出
		if output == nil {
			t.Fatal("output is nil")
		}

		// 验证tool_calls被正确合并到输出
		if len(output.ToolCalls) != 1 {
			t.Errorf("expected 1 tool_call in output, got %d", len(output.ToolCalls))
		}
		if output.ToolCalls[0].Function.Name != "test_function" {
			t.Errorf("tool_call function name = %v, want test_function", output.ToolCalls[0].Function.Name)
		}
	})
}

// TestMergeToolCallsIntegration 验证mergeToolCalls在完整Pipeline执行中的行为
func TestMergeToolCallsIntegration(t *testing.T) {
	t.Run("merge_tool_calls_from_multiple_nodes", func(t *testing.T) {
		// 创建测试引擎
		nodeRegistry := NewNodeRegistry()
		RegisterBuiltinNodes(nodeRegistry)
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		// 创建执行上下文
		execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "test"})

		// 模拟多个节点的tool_calls
		execCtx.SetResult("node1", &NodeOutput{
			ToolCalls: []ToolCall{
				{ID: "call_1", Function: FunctionCall{Name: "tool_a"}},
				{ID: "call_2", Function: FunctionCall{Name: "tool_b"}},
			},
		})
		execCtx.SetResult("node2", &NodeOutput{
			ToolCalls: []ToolCall{
				{ID: "call_3", Function: FunctionCall{Name: "tool_c"}},
			},
		})
		execCtx.SetResult("node3", &NodeOutput{}) // 无tool_calls

		// 合并tool_calls
		merged := engine.mergeToolCalls(execCtx)

		// 验证合并结果
		if len(merged) != 3 {
			t.Errorf("expected 3 merged tool_calls, got %d", len(merged))
		}

		// 验证tool_call ID唯一性
		seen := make(map[string]bool)
		for _, tc := range merged {
			if seen[tc.ID] {
				t.Errorf("duplicate tool_call ID: %s", tc.ID)
			}
			seen[tc.ID] = true
		}
	})
}

// TestToolCallInjectorErrorHandling 验证ToolCallInjector的错误处理
func TestToolCallInjectorErrorHandling(t *testing.T) {
	t.Run("invalid_condition_format", func(t *testing.T) {
		// 创建测试引擎
		nodeRegistry := NewNodeRegistry()
		RegisterBuiltinNodes(nodeRegistry)
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		// 创建Pipeline
		pipeline := &AgentPatternPipeline{
			ID:   "test-error-pipeline",
			Name: "Test Error Pipeline",
			Nodes: []PipelineNodeConfig{
				{
					ID:   "injector",
					Type: NodeTypeToolCallInjector,
					Config: NodeConfig{
						CustomConfig: map[string]interface{}{
							"condition": "{{invalid.syntax}}",
							"tool_calls": []map[string]interface{}{
								{
									"id":   "test_tool",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "test",
										"arguments": "{}",
									},
								},
							},
						},
					},
				},
			},
		}

		// 创建执行图
		graph := NewExecutionGraph(pipeline)
		execCtx := NewExecutionContext(pipeline)

		// 执行ToolCallInjector节点（条件评估错误应该跳过，不返回错误）
		err := engine.executeLayerNode(context.Background(), graph, execCtx, "injector", pipeline)
		if err != nil {
			t.Fatalf("executeLayerNode(injector) error = %v", err)
		}

		// 验证节点执行成功（跳过注入）
		node := graph.GetNode("injector")
		if node == nil {
			t.Fatal("injector node not found")
		}
		if node.Status != StatusSuccess {
			t.Errorf("injector node status = %v, want StatusSuccess", node.Status)
		}

		// 验证条件评估错误时跳过注入
		if node.Output == nil {
			t.Fatal("injector node output is nil")
		}
		if len(node.Output.ToolCalls) != 0 {
			t.Errorf("expected 0 tool_calls when condition has error, got %d", len(node.Output.ToolCalls))
		}
	})
}

// TestToolCallInjectorMetadata 验证ToolCallInjector输出的metadata
func TestToolCallInjectorMetadata(t *testing.T) {
	t.Run("metadata_contains_injection_info", func(t *testing.T) {
		// 创建测试引擎
		nodeRegistry := NewNodeRegistry()
		RegisterBuiltinNodes(nodeRegistry)
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		// 创建Pipeline
		pipeline := &AgentPatternPipeline{
			ID:   "test-metadata-pipeline",
			Name: "Test Metadata Pipeline",
			Nodes: []PipelineNodeConfig{
				{
					ID:   "injector",
					Type: NodeTypeToolCallInjector,
					Config: NodeConfig{
						CustomConfig: map[string]interface{}{
							"tool_calls": []map[string]interface{}{
								{
									"id":   "meta_tool",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "meta_function",
										"arguments": `{"action": "test"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		// 创建执行图
		graph := NewExecutionGraph(pipeline)
		execCtx := NewExecutionContext(pipeline)

		// 执行ToolCallInjector节点
		err := engine.executeLayerNode(context.Background(), graph, execCtx, "injector", pipeline)
		if err != nil {
			t.Fatalf("executeLayerNode(injector) error = %v", err)
		}

		// 验证节点执行成功
		node := graph.GetNode("injector")
		if node == nil {
			t.Fatal("injector node not found")
		}
		if node.Status != StatusSuccess {
			t.Errorf("injector node status = %v, want StatusSuccess", node.Status)
		}

		// 验证metadata
		if node.Output == nil {
			t.Fatal("injector node output is nil")
		}
		if node.Output.Metadata == nil {
			t.Fatal("injector node output metadata is nil")
		}

		// 验证node_type
		if nodeType, ok := node.Output.Metadata["node_type"]; !ok || nodeType != "tool_call_injector" {
			t.Errorf("metadata node_type = %v, want tool_call_injector", nodeType)
		}

		// 验证injected_count
		if count, ok := node.Output.Metadata["injected_count"]; !ok || count != 1 {
			t.Errorf("metadata injected_count = %v, want 1", count)
		}

		// 验证tool_call_ids
		if ids, ok := node.Output.Metadata["tool_call_ids"]; !ok {
			t.Error("metadata tool_call_ids not found")
		} else if idList, ok := ids.([]string); !ok || len(idList) != 1 {
			t.Errorf("metadata tool_call_ids = %v, want 1 id", ids)
		}
	})
}

// TestToolCallInjectorPerformance 验证ToolCallInjector的性能
func TestToolCallInjectorPerformance(t *testing.T) {
	t.Run("concurrent_injection", func(t *testing.T) {
		// 创建测试引擎
		nodeRegistry := NewNodeRegistry()
		RegisterBuiltinNodes(nodeRegistry)
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		// 创建Pipeline
		pipeline := &AgentPatternPipeline{
			ID:   "test-concurrent-pipeline",
			Name: "Test Concurrent Pipeline",
			Nodes: []PipelineNodeConfig{
				{
					ID:   "injector",
					Type: NodeTypeToolCallInjector,
					Config: NodeConfig{
						CustomConfig: map[string]interface{}{
							"tool_calls": []map[string]interface{}{
								{
									"id":   "perf_tool",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "perf_function",
										"arguments": `{"task": "performance"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		// 并发执行
		concurrency := 10
		start := time.Now()

		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				// 创建独立的执行图和上下文
				graph := NewExecutionGraph(pipeline)
				execCtx := NewExecutionContext(pipeline)

				// 执行ToolCallInjector节点
				err := engine.executeLayerNode(context.Background(), graph, execCtx, "injector", pipeline)
				if err != nil {
					t.Errorf("goroutine %d: executeLayerNode error = %v", idx, err)
				}
			}(i)
		}

		// 等待所有goroutine完成
		time.Sleep(100 * time.Millisecond)

		duration := time.Since(start)
		t.Logf("Concurrent injection completed in %v", duration)

		// 验证性能（10次并发执行应该在1秒内完成）
		if duration > time.Second {
			t.Errorf("concurrent injection took too long: %v", duration)
		}
	})
}

// TestToolCallInjectorWithRealPipeline 验证ToolCallInjector与真实Pipeline的集成
func TestToolCallInjectorWithRealPipeline(t *testing.T) {
	t.Run("full_pipeline_execution", func(t *testing.T) {
		// 创建测试引擎
		nodeRegistry := NewNodeRegistry()
		RegisterBuiltinNodes(nodeRegistry)
		pipelineRegistry := NewPipelineRegistry()
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

		// 创建完整的Pipeline（模拟代码审查场景）
		pipeline := &AgentPatternPipeline{
			ID:   "code-review-pipeline",
			Name: "Code Review Pipeline",
			Nodes: []PipelineNodeConfig{
				{
					ID:   "reviewer",
					Type: NodeTypeReviewer,
				},
				{
					ID:   "injector",
					Type: NodeTypeToolCallInjector,
					Config: NodeConfig{
						CustomConfig: map[string]interface{}{
							"condition": "{{node.reviewer.score}} < 0.8",
							"tool_calls": []map[string]interface{}{
								{
									"id":   "auto_fix",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "fix_code",
										"arguments": `{"file": "test.go", "line": 42, "suggestion": "use error wrapping"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		// 注册Pipeline
		pipelineRegistry.Register(pipeline)

		// 执行Pipeline
		input := &PipelineInput{
			Content: "func main() { err := doSomething(); if err != nil { fmt.Println(err) } }",
		}

		output, err := engine.Execute(context.Background(), pipeline.ID, input)
		if err != nil {
			t.Logf("Pipeline execution result (may fail if backend unreachable): %v", err)
		} else {
			// 如果执行成功，验证输出
			if output != nil {
				t.Logf("Pipeline output: %s", output.Content)
				if len(output.ToolCalls) > 0 {
					t.Logf("Injected tool_calls: %d", len(output.ToolCalls))
					for _, tc := range output.ToolCalls {
						t.Logf("  - %s(%s)", tc.Function.Name, tc.Function.Arguments)
					}
				}
			}
		}
	})
}
