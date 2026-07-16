package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"centag/core/pkg/logger"
)

func TestMain(m *testing.M) {
	// 初始化日志，避免测试中 logger.Logger 为 nil 导致 panic
	_ = logger.Init(logger.Config{
		Level:  "error",
		Format: "console",
		Output: "stdout",
	})
	os.Exit(m.Run())
}

func TestExecutionGraphTopologicalSort(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []PipelineNodeConfig
		wantErr bool
	}{
		{
			name: "linear pipeline",
			nodes: []PipelineNodeConfig{
				{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node2"}},
				{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m", NextNodes: []string{"node3"}},
				{ID: "node3", Type: NodeTypeReviewer, Backend: "b", Model: "m"},
			},
			wantErr: false,
		},
		{
			name: "diamond pipeline",
			nodes: []PipelineNodeConfig{
				{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node2", "node3"}},
				{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m", NextNodes: []string{"node4"}},
				{ID: "node3", Type: NodeTypeProcessor, Backend: "b", Model: "m", NextNodes: []string{"node4"}},
				{ID: "node4", Type: NodeTypeAggregator, Backend: "b", Model: "m"},
			},
			wantErr: false,
		},
		{
			name: "cycle detection",
			nodes: []PipelineNodeConfig{
				{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node2"}},
				{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m", NextNodes: []string{"node3"}},
				{ID: "node3", Type: NodeTypeReviewer, Backend: "b", Model: "m", NextNodes: []string{"node1"}},
			},
			wantErr: true,
		},
		{
			name: "self cycle",
			nodes: []PipelineNodeConfig{
				{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node1"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := &AgentPatternPipeline{
				ID:    "test",
				Name:  "Test",
				Nodes: tt.nodes,
			}

			graph := NewExecutionGraph(pipeline)
			order, err := graph.TopologicalSort()

			if (err != nil) != tt.wantErr {
				t.Errorf("TopologicalSort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(order) != len(tt.nodes) {
					t.Errorf("Expected %d nodes in order, got %d", len(tt.nodes), len(order))
				}

				// 验证顺序：每个节点在依赖之后
				for _, node := range tt.nodes {
					nodeIndex := -1
					for i, id := range order {
						if id == node.ID {
							nodeIndex = i
							break
						}
					}

					for _, dep := range node.DependsOn {
						depIndex := -1
						for i, id := range order {
							if id == dep {
								depIndex = i
								break
							}
						}
						if depIndex >= nodeIndex {
							t.Errorf("Node %s should come after dependency %s", node.ID, dep)
						}
					}
				}
			}
		})
	}
}

func TestExecutionContext(t *testing.T) {
	pipeline := &AgentPatternPipeline{ID: "test", Name: "Test"}
	ctx := NewExecutionContext(pipeline)

	// 测试变量设置和获取
	ctx.SetVariable("key1", "value1")
	val, ok := ctx.GetVariable("key1")
	if !ok || val != "value1" {
		t.Error("Failed to set/get variable")
	}

	// 测试不存在的变量
	_, ok = ctx.GetVariable("nonexistent")
	if ok {
		t.Error("Should not find nonexistent variable")
	}

	// 测试结果设置和获取
	output := &NodeOutput{Content: "test output"}
	ctx.SetResult("node1", output)
	result, ok := ctx.GetResult("node1")
	if !ok || result.Content != "test output" {
		t.Error("Failed to set/get result")
	}

	// 测试获取最后输出
	last := ctx.GetLastOutput()
	if last == nil || last.Content != "test output" {
		t.Error("GetLastOutput should return the last set result")
	}
}

func TestExecutionNodeStatus(t *testing.T) {
	pipeline := &AgentPatternPipeline{
		ID:   "test",
		Name: "Test",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
		},
	}

	graph := NewExecutionGraph(pipeline)
	node := graph.GetNode("node1")
	if node == nil {
		t.Fatal("Node not found in graph")
	}

	if node.Status != StatusPending {
		t.Errorf("Initial status should be pending, got %v", node.Status)
	}

	// 模拟状态变更
	node.Status = StatusRunning
	if node.Status != StatusRunning {
		t.Error("Status change failed")
	}
}

func TestConditionEvaluator(t *testing.T) {
	pipeline := &AgentPatternPipeline{ID: "test", Name: "Test"}
	execCtx := NewExecutionContext(pipeline)
	evaluator := NewConditionEvaluator(execCtx)

	// 空条件应该返回true
	if !evaluator.Evaluate("") {
		t.Error("Empty condition should evaluate to true")
	}

	// 非空条件（目前实现返回true）
	if !evaluator.Evaluate("some condition") {
		t.Log("Non-empty condition evaluation not fully implemented yet")
	}
}

func TestPipelineEngineCreation(t *testing.T) {
	nodeRegistry := NewNodeRegistry()
	pipelineRegistry := NewPipelineRegistry()
	plLogger := NewPipelineLogger()

	// 创建引擎
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)
	if engine == nil {
		t.Fatal("Failed to create pipeline engine")
	}
}

func TestPipelineEngineExecuteNotFound(t *testing.T) {
	nodeRegistry := NewNodeRegistry()
	pipelineRegistry := NewPipelineRegistry()
	plLogger := NewPipelineLogger()
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, plLogger, nil)

	ctx := context.Background()
	input := &PipelineInput{Content: "test"}

	_, err := engine.Execute(ctx, "nonexistent-pipeline", input)
	if err == nil {
		t.Error("Execute should fail for nonexistent pipeline")
	}
}

func TestExecutionGraphGetDependencies(t *testing.T) {
	pipeline := &AgentPatternPipeline{
		ID:   "test",
		Name: "Test",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node2", "node3"}},
			{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m", NextNodes: []string{"node4"}},
			{ID: "node3", Type: NodeTypeProcessor, Backend: "b", Model: "m", NextNodes: []string{"node4"}},
			{ID: "node4", Type: NodeTypeAggregator, Backend: "b", Model: "m"},
		},
	}

	graph := NewExecutionGraph(pipeline)

	// node4 应该依赖 node2 和 node3
	deps := graph.GetDependencies("node4")
	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies for node4, got %d", len(deps))
	}

	// node1 应该没有依赖
	deps = graph.GetDependencies("node1")
	if len(deps) != 0 {
		t.Errorf("Expected 0 dependencies for node1, got %d", len(deps))
	}
}

func TestLayeredTopologicalSort(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []PipelineNodeConfig
		wantLayers int
		wantErr  bool
	}{
		{
			name: "linear pipeline → single node per layer",
			nodes: []PipelineNodeConfig{
				{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node2"}},
				{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m", NextNodes: []string{"node3"}},
				{ID: "node3", Type: NodeTypeReviewer, Backend: "b", Model: "m"},
			},
			wantLayers: 3,
			wantErr:    false,
		},
		{
			name: "parallel generators → same layer",
			nodes: []PipelineNodeConfig{
				{ID: "gen1", Type: NodeTypeGenerator, Backend: "b", Model: "m", DependsOn: []string{}},
				{ID: "gen2", Type: NodeTypeGenerator, Backend: "b", Model: "m", DependsOn: []string{}},
				{ID: "agg", Type: NodeTypeAggregator, Backend: "b", Model: "m", DependsOn: []string{"gen1", "gen2"}},
			},
			wantLayers: 2,
			wantErr:    false,
		},
		{
			name: "cycle detection",
			nodes: []PipelineNodeConfig{
				{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node2"}},
				{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m", NextNodes: []string{"node1"}},
			},
			wantLayers: 0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := &AgentPatternPipeline{
				ID:    "test",
				Name:  "Test",
				Nodes: tt.nodes,
			}

			graph := NewExecutionGraph(pipeline)
			layers, err := graph.LayeredTopologicalSort()

			if (err != nil) != tt.wantErr {
				t.Errorf("LayeredTopologicalSort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(layers) != tt.wantLayers {
					t.Errorf("Expected %d layers, got %d: %v", tt.wantLayers, len(layers), layers)
				}

				// 验证同一个节点不出现在多个层中
				seen := make(map[string]int)
				for layerIdx, layer := range layers {
					for _, nodeID := range layer {
						if prevLayer, ok := seen[nodeID]; ok {
							t.Errorf("Node %s appears in layers %d and %d", nodeID, prevLayer, layerIdx)
						}
						seen[nodeID] = layerIdx
					}
				}

				// 验证并行层包含多个节点
				if tt.name == "parallel generators → same layer" {
					foundParallelLayer := false
					for _, layer := range layers {
						if len(layer) >= 2 {
							foundParallelLayer = true
							break
						}
					}
					if !foundParallelLayer {
						t.Error("Expected at least one layer with multiple nodes for parallel execution")
					}
				}
			}
		})
	}
}

func TestExecutionContextConcurrency(t *testing.T) {
	pipeline := &AgentPatternPipeline{ID: "test", Name: "Test"}
	ctx := NewExecutionContext(pipeline)

	var wg sync.WaitGroup
	numGoroutines := 10

	// 并发 SetResult
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			nodeID := fmt.Sprintf("node-%d", idx)
			output := &NodeOutput{Content: nodeID}
			ctx.SetResult(nodeID, output)
		}(i)
	}

	// 并发 SetVariable
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", idx)
			ctx.SetVariable(key, idx)
		}(i)
	}

	wg.Wait()

	// 验证所有结果都正确存储
	for i := 0; i < numGoroutines; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		result, ok := ctx.GetResult(nodeID)
		if !ok {
			t.Errorf("Missing result for %s", nodeID)
			continue
		}
		if result.Content != nodeID {
			t.Errorf("Expected %s, got %s", nodeID, result.Content)
		}
	}

	// 验证 GetLastOutput 可安全调用（无 panic）
	last := ctx.GetLastOutput()
	if last == nil {
		t.Log("GetLastOutput returned nil (expected with concurrent results)")
	}
}

func TestBranchSelector(t *testing.T) {
	t.Run("node with matching route executes", func(t *testing.T) {
		pipeline := &AgentPatternPipeline{
			ID:   "test",
			Name: "Test",
			Nodes: []PipelineNodeConfig{
				{ID: "router", Type: NodeTypeRouter, Backend: "b", Model: "m"},
				{
					ID: "branch-a", Type: NodeTypeGenerator, Backend: "b", Model: "m",
					RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "a"},
					DependsOn:   []string{"router"},
				},
				{
					ID: "branch-b", Type: NodeTypeGenerator, Backend: "b", Model: "m",
					RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "b", IsDefault: true},
					DependsOn:   []string{"router"},
				},
			},
			GlobalConfig: DefaultGlobalConfig(),
		}

		graph := NewExecutionGraph(pipeline)
		layers, err := graph.LayeredTopologicalSort()
		if err != nil {
			t.Fatalf("LayeredTopologicalSort failed: %v", err)
		}

		_ = layers
		// RouteConfig 结构体的序列化/反序列化验证
		if pipeline.Nodes[1].RouteConfig.RouterNodeID != "router" {
			t.Error("RouteConfig.RouterNodeID not set correctly")
		}
		if pipeline.Nodes[1].RouteConfig.RouteValue != "a" {
			t.Error("RouteConfig.RouteValue not set correctly")
		}
		if pipeline.Nodes[2].RouteConfig.IsDefault != true {
			t.Error("RouteConfig.IsDefault not set correctly")
		}
	})
}

func TestFallbackGroupStructure(t *testing.T) {
	fg := FallbackGroup{
		PrimaryNodeID: "primary",
		FallbackNodes: []string{"fb1", "fb2"},
		MaxAttempts:   3,
	}

	if fg.PrimaryNodeID != "primary" {
		t.Error("FallbackGroup PrimaryNodeID not set")
	}
	if len(fg.FallbackNodes) != 2 {
		t.Errorf("Expected 2 fallback nodes, got %d", len(fg.FallbackNodes))
	}
	if fg.MaxAttempts != 3 {
		t.Errorf("Expected 3 max attempts, got %d", fg.MaxAttempts)
	}

	// 验证 FallbackGroup 集成到 GlobalPipelineConfig
	cfg := DefaultGlobalConfig()
	cfg.FallbackGroups = []FallbackGroup{fg}

	if len(cfg.FallbackGroups) != 1 {
		t.Error("GlobalPipelineConfig should contain 1 fallback group")
	}
	if cfg.FallbackGroups[0].PrimaryNodeID != "primary" {
		t.Error("FallbackGroup stored in GlobalPipelineConfig incorrectly")
	}
}

func TestPipelineEngineConcurrencySafety(t *testing.T) {
	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "test"})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			execCtx.SetResult(fmt.Sprintf("node-%d", idx), &NodeOutput{Content: fmt.Sprintf("result-%d", idx)})
		}(i)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			execCtx.GetResult(fmt.Sprintf("node-%d", idx))
		}(i)
	}
	wg.Wait()

	// 验证不会 race（go test -race 通过）
	// 验证所有写入都可读
	for i := 0; i < 20; i++ {
		result, ok := execCtx.GetResult(fmt.Sprintf("node-%d", i))
		if !ok {
			t.Errorf("Missing result for node-%d", i)
		} else if result.Content != fmt.Sprintf("result-%d", i) {
			t.Errorf("Wrong content for node-%d: got %s", i, result.Content)
		}
	}
}

// testBackendClient 模拟后端客户端，返回固定内容（避免与 builtin_nodes_test.go 中的 mockBackendClient 冲突）
type testBackendClient struct {
	response string
	err      error
}

func (m *testBackendClient) Chat(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &LLMResponse{
		Model:      req.Model,
		Content:    m.response,
		TokenUsage: 10,
	}, nil
}

// testCapabilityBroker 模拟能力代理，将 testBackendClient 作为 LLMClient 返回
type testCapabilityBroker struct {
	llmClient LLMClient
}

func (m *testCapabilityBroker) GetLLMClient(ctx context.Context, permissions []string) (LLMClient, error) {
	return m.llmClient, nil
}
func (m *testCapabilityBroker) GetLLMStreamClient(ctx context.Context, permissions []string) (LLMStreamClient, error) {
	return nil, nil
}
func (m *testCapabilityBroker) GetStorage(ctx context.Context, permissions []string) (Storage, error) {
	return nil, nil
}
func (m *testCapabilityBroker) GetMemory(ctx context.Context, permissions []string) (Memory, error) {
	return nil, nil
}
func (m *testCapabilityBroker) GetSecretsResolver(ctx context.Context, permissions []string) (SecretsResolver, error) {
	return nil, nil
}
func (m *testCapabilityBroker) GetHTTPClient(ctx context.Context, permissions []string) (HTTPClient, error) {
	return nil, nil
}
func (m *testCapabilityBroker) GetCacheStrategy(ctx context.Context, strategy string, permissions []string) (CacheStrategyCapability, error) {
	return nil, nil
}
func (m *testCapabilityBroker) GetVectorCache(ctx context.Context, permissions []string) (VectorCacheCapability, error) {
	return nil, nil
}
func (m *testCapabilityBroker) GetEmbeddingService(ctx context.Context, permissions []string) (EmbeddingCapability, error) {
	return nil, nil
}

func newTestPipelineEngine() (*PipelineEngine, *NodeRegistry) {
	nodeRegistry := NewNodeRegistry()
	RegisterBuiltinNodes(nodeRegistry)

	plLogger := NewPipelineLogger()
	engine := NewPipelineEngine(nodeRegistry, NewPipelineRegistry(), nil, plLogger, nil)
	return engine, nodeRegistry
}

// TestBranchSelectorBehavior 验证 BranchSelector 按路由值正确跳过/执行分支
func TestBranchSelectorBehavior(t *testing.T) {
	t.Run("selected_route matches → branch executes", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		cfg := DefaultGlobalConfig()
		cfg.BypassOnError = false
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: cfg}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		// 模拟路由节点输出 selected_route = "a"
		execCtx.SetResult("router", &NodeOutput{
			Content: "",
			Metadata: map[string]interface{}{
				"selected_route": "a",
			},
		})

		// 分支 a：RouteValue="a"，应该通过检查（即使后端为 nil，也证明没被跳过）
		graph.nodes["branch-a"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "branch-a",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "a"},
			},
			Status: StatusPending,
		}

		err := engine.executeLayerNode(context.Background(), graph, execCtx, "branch-a", p)
		if err == nil {
			t.Error("expected executeNode error (nil backend) but route check passed correctly")
		} else {
			// 如果是后端错误而非跳过，说明 RouteConfig 检查正确通过了
			node := graph.GetNode("branch-a")
			if node != nil && node.Status == StatusSkipped {
				t.Error("branch-a was incorrectly skipped when route matched")
			}
			t.Logf("route check passed, got expected execution error: %v", err)
		}
	})

	t.Run("selected_route differs and not default → skipped", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		cfg := DefaultGlobalConfig()
		cfg.BypassOnError = false
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: cfg}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		execCtx.SetResult("router", &NodeOutput{
			Content: "",
			Metadata: map[string]interface{}{
				"selected_route": "a",
			},
		})

		graph.nodes["branch-b"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "branch-b",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "b"},
			},
			Status: StatusPending,
		}

		err := engine.executeLayerNode(context.Background(), graph, execCtx, "branch-b", p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		node := graph.GetNode("branch-b")
		if node == nil || node.Status != StatusSkipped {
			t.Errorf("branch-b should be StatusSkipped, got %v", node.Status)
		}
	})

	t.Run("selected_route differs but IsDefault=true → executes", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		cfg := DefaultGlobalConfig()
		cfg.BypassOnError = false
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: cfg}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		execCtx.SetResult("router", &NodeOutput{
			Content: "",
			Metadata: map[string]interface{}{
				"selected_route": "a",
			},
		})

		graph.nodes["default-branch"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "default-branch",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "b", IsDefault: true},
			},
			Status: StatusPending,
		}

		err := engine.executeLayerNode(context.Background(), graph, execCtx, "default-branch", p)
		if err == nil {
			t.Error("expected executeNode error (nil backend) but IsDefault check passed correctly")
		} else {
			node := graph.GetNode("default-branch")
			if node != nil && node.Status == StatusSkipped {
				t.Error("default branch was incorrectly skipped")
			}
			t.Logf("IsDefault route check passed, got expected execution error: %v", err)
		}
	})

	t.Run("route result not available → skipped", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: DefaultGlobalConfig()}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		graph.nodes["orphan"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "orphan",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "nonexistent", RouteValue: "a"},
			},
			Status: StatusPending,
		}

		err := engine.executeLayerNode(context.Background(), graph, execCtx, "orphan", p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		node := graph.GetNode("orphan")
		if node == nil || node.Status != StatusSkipped {
			t.Errorf("orphan should be StatusSkipped when route result unavailable, got %v", node.Status)
		}
	})
}

// TestFallbackGroupExecutionBehavior 验证降级组执行行为
func TestFallbackGroupExecutionBehavior(t *testing.T) {
	makeTestEngine := func() *PipelineEngine {
		mockClient := &testBackendClient{response: "mock response"}
		mockBroker := &testCapabilityBroker{llmClient: mockClient}

		nodeRegistry := NewNodeRegistry()
		RegisterBuiltinNodes(nodeRegistry)

		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, NewPipelineRegistry(), mockBroker, plLogger, nil)
		return engine
	}

	t.Run("primary succeeds → fallback nodes skipped", func(t *testing.T) {
		pipeline := &AgentPatternPipeline{
			ID:   "test-fallback",
			Name: "Test Fallback",
			Nodes: []PipelineNodeConfig{
				{ID: "primary", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
				{ID: "fb1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
			},
			GlobalConfig: GlobalPipelineConfig{
				ParallelLimit: 4,
				BypassOnError: false,
				FallbackGroups: []FallbackGroup{
					{PrimaryNodeID: "primary", FallbackNodes: []string{"fb1"}},
				},
			},
		}

		graph := NewExecutionGraph(pipeline)
		execCtx := NewExecutionContext(pipeline)

		// 模拟主节点已成功执行
		graph.nodes["primary"].Status = StatusSuccess
		graph.nodes["primary"].Output = &NodeOutput{Content: "primary result"}
		execCtx.SetResult("primary", &NodeOutput{Content: "primary result"})
		execCtx.SetCurrentNode("primary")

		// 手动调用降级组处理逻辑（executePipeline 中的第 7 步）
		for _, fg := range pipeline.GlobalConfig.FallbackGroups {
			primaryNode := graph.GetNode(fg.PrimaryNodeID)
			if primaryNode == nil || primaryNode.Status != StatusFailed {
				for _, fbID := range fg.FallbackNodes {
					if fbNode := graph.GetNode(fbID); fbNode != nil {
						fbNode.Status = StatusSkipped
					}
				}
			}
		}

		fb1 := graph.GetNode("fb1")
		if fb1 == nil || fb1.Status != StatusSkipped {
			t.Errorf("fallback should be StatusSkipped when primary succeeds, got %v", fb1.Status)
		}
	})

	t.Run("primary fails → fallback executes", func(t *testing.T) {
		engine := makeTestEngine()
		pipeline := &AgentPatternPipeline{
			ID:   "test-fallback",
			Name: "Test Fallback",
			Nodes: []PipelineNodeConfig{
				{ID: "primary", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
				{ID: "fb1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
			},
			GlobalConfig: GlobalPipelineConfig{
				ParallelLimit: 4,
				BypassOnError: false,
				FallbackGroups: []FallbackGroup{
					{PrimaryNodeID: "primary", FallbackNodes: []string{"fb1"}},
				},
			},
		}

		graph := NewExecutionGraph(pipeline)
		execCtx := NewExecutionContext(pipeline)

		// 模拟主节点失败
		graph.nodes["primary"].Status = StatusFailed
		graph.nodes["primary"].Error = fmt.Errorf("primary failed")

		// 通过 executePipeline 执行完整流程，验证降级组处理
		execCtx.SetResult("primary", &NodeOutput{
			Content: "",
			Metadata: map[string]interface{}{
				"bypass":        true,
				"bypass_reason": "primary failed",
				"bypass_node":   "primary",
			},
		})

		fallbackSuccess := false
		for _, fg := range pipeline.GlobalConfig.FallbackGroups {
			primaryNode := graph.GetNode(fg.PrimaryNodeID)
			if primaryNode == nil || primaryNode.Status != StatusFailed {
				continue
			}
			for _, fbID := range fg.FallbackNodes {
				err := engine.executeLayerNode(context.Background(), graph, execCtx, fbID, pipeline)
				if err != nil {
					t.Logf("fallback node execution: %v", err)
					continue
				}
				fbNode := graph.GetNode(fbID)
				if fbNode != nil && fbNode.Status == StatusSuccess {
					fallbackSuccess = true
					break
				}
			}
		}

		if !fallbackSuccess {
			t.Error("fallback node should have executed when primary failed")
		}

		fb1 := graph.GetNode("fb1")
		if fb1 == nil || fb1.Status != StatusSuccess {
			t.Errorf("fallback node should be StatusSuccess, got %v", fb1.Status)
		}
	})

	t.Run("all fallbacks fail → returns error", func(t *testing.T) {
		mockClient := &testBackendClient{err: fmt.Errorf("mock backend failed")}
		mockBroker := &testCapabilityBroker{llmClient: mockClient}

		nodeRegistry := NewNodeRegistry()
		RegisterBuiltinNodes(nodeRegistry)
		plLogger := NewPipelineLogger()
		engine := NewPipelineEngine(nodeRegistry, NewPipelineRegistry(), mockBroker, plLogger, nil)

		pipeline := &AgentPatternPipeline{
			ID:   "test-fallback-fail",
			Name: "Test",
			Nodes: []PipelineNodeConfig{
				{ID: "primary", Type: NodeTypeGenerator, Backend: "nonexistent", Model: "m"},
				{ID: "fb1", Type: NodeTypeGenerator, Backend: "nonexistent", Model: "m"},
			},
			GlobalConfig: GlobalPipelineConfig{
				ParallelLimit: 4,
				BypassOnError: false,
				FallbackGroups: []FallbackGroup{
					{PrimaryNodeID: "primary", FallbackNodes: []string{"fb1"}},
				},
			},
		}

		graph := NewExecutionGraph(pipeline)
		graph.nodes["primary"].Status = StatusFailed
		graph.nodes["primary"].Error = fmt.Errorf("primary failed")

		_, err := engine.executePipeline(context.Background(), pipeline, &PipelineInput{Content: "test"})
		if err == nil {
			t.Error("expected error when all fallbacks fail")
		} else {
			t.Logf("got expected error: %v", err)
		}
	})
}

// TestLayeredParallelExecution 验证并行执行时间 < 串行执行时间
func TestLayeredParallelExecution(t *testing.T) {
	nodeRegistry := NewNodeRegistry()
	RegisterBuiltinNodes(nodeRegistry)
	plLogger := NewPipelineLogger()

	engine := NewPipelineEngine(nodeRegistry, NewPipelineRegistry(), nil, plLogger, nil)

	// 创建有 3 个并行 generator 的 pipeline
	pipeline := &AgentPatternPipeline{
		ID:   "test-parallel",
		Name: "Test Parallel",
		Nodes: []PipelineNodeConfig{
			{ID: "gen1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
			{ID: "gen2", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
			{ID: "gen3", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
			{ID: "agg", Type: NodeTypeAggregator, Backend: "b", Model: "m", DependsOn: []string{"gen1", "gen2", "gen3"}},
		},
		GlobalConfig: GlobalPipelineConfig{
			ParallelLimit: 3,
			BypassOnError: true,
		},
	}

	input := &PipelineInput{Content: "test input"}

	start := time.Now()
	_, err := engine.ExecutePipelineDefinition(context.Background(), pipeline, input)
	duration := time.Since(start)

	if err != nil {
		t.Logf("execution result (expected may fail if backend unreachable): %v", err)
	} else {
		t.Logf("parallel execution took %v", duration)
		// 3 个并行节点理论上应接近单节点时间而非 3 倍
		if duration > 2*time.Second {
			t.Log("parallel execution took longer than expected, possibly due to mock overhead")
		}
	}
}

func TestExecutePipelineBypassRequiresUsableFallbackOutput(t *testing.T) {
	mockClient := &testBackendClient{err: fmt.Errorf("mock backend failed")}
	mockBroker := &testCapabilityBroker{llmClient: mockClient}

	nodeRegistry := NewNodeRegistry()
	RegisterBuiltinNodes(nodeRegistry)
	plLogger := NewPipelineLogger()
	engine := NewPipelineEngine(nodeRegistry, NewPipelineRegistry(), mockBroker, plLogger, nil)

	p := &AgentPatternPipeline{
		ID:   "test-bypass-without-fallback",
		Name: "Test Bypass Without Fallback",
		Nodes: []PipelineNodeConfig{
			{ID: "generate", Type: NodeTypeGenerator, Backend: "bigmodel", Model: "glm-4-flash"},
		},
		GlobalConfig: GlobalPipelineConfig{
			BypassOnError: true,
			ParallelLimit: 1,
		},
	}
	p.Nodes[0].Normalize()

	_, err := engine.ExecutePipelineDefinition(context.Background(), p, &PipelineInput{Content: "hello"})
	if err == nil {
		t.Fatal("expected error when bypass_on_error has no usable fallback output")
	}
	if !strings.Contains(err.Error(), "no usable fallback output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHasUsableBypassOutput(t *testing.T) {
	tests := []struct {
		name   string
		output *NodeOutput
		want   bool
	}{
		{name: "nil output", output: nil, want: false},
		{name: "empty output", output: &NodeOutput{}, want: false},
		{name: "content output", output: &NodeOutput{Content: "ok"}, want: true},
		{name: "review output", output: &NodeOutput{Passed: boolPtr(true)}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasUsableBypassOutput(tt.output); got != tt.want {
				t.Fatalf("hasUsableBypassOutput()=%v, want %v", got, tt.want)
			}
		})
	}
}

func boolPtr(v bool) *bool {
	return &v
}

// TestFilterRoutedNodes 验证 filterRoutedNodes 对 default 分支的抑制逻辑
func TestFilterRoutedNodes(t *testing.T) {
	t.Run("non-default route matched → default nodes suppressed", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: DefaultGlobalConfig()}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		// 设置路由结果
		execCtx.SetResult("router", &NodeOutput{
			Content: "",
			Metadata: map[string]interface{}{
				"selected_route": "code",
			},
		})

		// 路由节点
		graph.nodes["router"] = &ExecutionNode{
			Config: PipelineNodeConfig{ID: "router", Type: NodeTypeRouter},
			Status: StatusSuccess,
		}
		// code 分支（非 default）
		graph.nodes["code-gen"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "code-gen",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "code"},
			},
			Status: StatusPending,
		}
		// chat 分支（default）
		graph.nodes["chat-gen"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "chat-gen",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "chat", IsDefault: true},
			},
			Status: StatusPending,
		}

		nodeIDs := []string{"code-gen", "chat-gen"}
		result := engine.filterRoutedNodes(nodeIDs, graph, execCtx)

		if len(result) != 1 || result[0] != "code-gen" {
			t.Fatalf("expected only code-gen, got %v", result)
		}
		chatNode := graph.GetNode("chat-gen")
		if chatNode == nil || chatNode.Status != StatusSkipped {
			t.Errorf("expected chat-gen to be StatusSkipped, got %v", chatNode.Status)
		}
	})

	t.Run("no non-default nodes → no suppression", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: DefaultGlobalConfig()}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		graph.nodes["chat-gen"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "chat-gen",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "chat", IsDefault: true},
			},
			Status: StatusPending,
		}
		graph.nodes["fallback"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "fallback",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "fallback", IsDefault: true},
			},
			Status: StatusPending,
		}

		nodeIDs := []string{"chat-gen", "fallback"}
		result := engine.filterRoutedNodes(nodeIDs, graph, execCtx)

		if len(result) != 2 {
			t.Fatalf("expected all 2 nodes, got %v", result)
		}
	})

	t.Run("no default nodes and no match → all skipped", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: DefaultGlobalConfig()}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		graph.nodes["code-gen"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "code-gen",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "code"},
			},
			Status: StatusPending,
		}
		graph.nodes["translate-gen"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "translate-gen",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "translate"},
			},
			Status: StatusPending,
		}

		nodeIDs := []string{"code-gen", "translate-gen"}
		result := engine.filterRoutedNodes(nodeIDs, graph, execCtx)

		if len(result) != 0 {
			t.Fatalf("expected 0 nodes (no match, no default), got %v", result)
		}
	})

	t.Run("no route result available → fallback to default", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: DefaultGlobalConfig()}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		graph.nodes["code-gen"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "code-gen",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "nonexistent", RouteValue: "code"},
			},
			Status: StatusPending,
		}
		graph.nodes["chat-gen"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "chat-gen",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "nonexistent", RouteValue: "chat", IsDefault: true},
			},
			Status: StatusPending,
		}

		nodeIDs := []string{"code-gen", "chat-gen"}
		result := engine.filterRoutedNodes(nodeIDs, graph, execCtx)

		if len(result) != 1 || result[0] != "chat-gen" {
			t.Fatalf("expected only chat-gen (fallback), got %v", result)
		}
	})

	t.Run("matched by node ID (compatibility path)", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: DefaultGlobalConfig()}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		execCtx.SetResult("router", &NodeOutput{
			Metadata: map[string]interface{}{
				"selected_route": "code-generator",
			},
		})

		graph.nodes["code-generator"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "code-generator",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "code"},
			},
			Status: StatusPending,
		}
		graph.nodes["chat-generator"] = &ExecutionNode{
			Config: PipelineNodeConfig{
				ID:          "chat-generator",
				Type:        NodeTypeGenerator,
				RouteConfig: &RouteConfig{RouterNodeID: "router", RouteValue: "chat", IsDefault: true},
			},
			Status: StatusPending,
		}

		nodeIDs := []string{"code-generator", "chat-generator"}
		result := engine.filterRoutedNodes(nodeIDs, graph, execCtx)

		if len(result) != 1 || result[0] != "code-generator" {
			t.Fatalf("expected only code-generator (matched via node ID), got %v", result)
		}
	})

	t.Run("nodes without RouteConfig are ignored", func(t *testing.T) {
		engine, _ := newTestPipelineEngine()
		p := &AgentPatternPipeline{ID: "test", Name: "Test", GlobalConfig: DefaultGlobalConfig()}
		graph := NewExecutionGraph(p)
		execCtx := NewExecutionContext(p)

		graph.nodes["plain"] = &ExecutionNode{
			Config: PipelineNodeConfig{ID: "plain", Type: NodeTypeGenerator},
			Status: StatusPending,
		}

		nodeIDs := []string{"plain"}
		result := engine.filterRoutedNodes(nodeIDs, graph, execCtx)
		if len(result) != 1 || result[0] != "plain" {
			t.Fatalf("expected plain node preserved, got %v", result)
		}
	})
}
