package pipeline

import (
	"testing"
)

func TestPipelineValidation(t *testing.T) {
	tests := []struct {
		name    string
		pipeline *AgentPatternPipeline
		wantErr bool
	}{
		{
			name: "valid pipeline",
			pipeline: &AgentPatternPipeline{
				ID:      "test-pipeline",
				Name:    "Test Pipeline",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{
						ID:      "node1",
						Type:    NodeTypeGenerator,
						Name:    "Generator",
						Backend: "test-backend",
						Model:   "gpt-4",
					},
				},
				GlobalConfig: DefaultGlobalConfig(),
			},
			wantErr: false,
		},
		{
			name: "missing id",
			pipeline: &AgentPatternPipeline{
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty nodes draft",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes:   []PipelineNodeConfig{},
			},
			wantErr: false,
		},
		{
			name: "duplicate node id",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
					{ID: "node1", Type: NodeTypeProcessor, Backend: "b", Model: "m"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid node type",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeType("invalid"), Backend: "b", Model: "m"},
				},
			},
			wantErr: true,
		},
		{
			name: "draft node missing backend",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Model: "m"},
				},
			},
			wantErr: false,
		},
		{
			name: "draft node missing model",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Backend: "b"},
				},
			},
			wantErr: false,
		},
		{
			name: "cycle detection",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node2"}},
					{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m", NextNodes: []string{"node1"}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pipeline.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPipelineGetNode(t *testing.T) {
	pipeline := &AgentPatternPipeline{
		ID:   "test",
		Name: "Test",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
			{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m"},
		},
	}

	node := pipeline.GetNode("node1")
	if node == nil {
		t.Fatal("GetNode returned nil for existing node")
	}
	if node.ID != "node1" {
		t.Errorf("GetNode returned wrong node: %v", node.ID)
	}

	node = pipeline.GetNode("nonexistent")
	if node != nil {
		t.Error("GetNode should return nil for nonexistent node")
	}
}

func TestPipelineGetStartNodes(t *testing.T) {
	pipeline := &AgentPatternPipeline{
		ID:   "test",
		Name: "Test",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
			{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m", DependsOn: []string{"node1"}},
		},
	}

	starts := pipeline.GetStartNodes()
	if len(starts) != 1 {
		t.Errorf("Expected 1 start node, got %d", len(starts))
	}
	if starts[0].ID != "node1" {
		t.Errorf("Expected start node to be node1, got %v", starts[0].ID)
	}
}

func TestPipelineGetEndNodes(t *testing.T) {
	pipeline := &AgentPatternPipeline{
		ID:   "test",
		Name: "Test",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node2"}},
			{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m"},
		},
	}

	ends := pipeline.GetEndNodes()
	if len(ends) != 1 {
		t.Errorf("Expected 1 end node, got %d", len(ends))
	}
	if ends[0].ID != "node2" {
		t.Errorf("Expected end node to be node2, got %v", ends[0].ID)
	}
}

func TestPipelineRegistry(t *testing.T) {
	registry := NewPipelineRegistry()

	pipeline := &AgentPatternPipeline{
		ID:      "test-pipeline",
		Name:    "Test Pipeline",
		Version: "1.0",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
		},
		GlobalConfig: DefaultGlobalConfig(),
	}

	// 测试注册
	err := registry.Register(pipeline)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 测试获取
	got := registry.Get("test-pipeline")
	if got == nil {
		t.Fatal("Get returned nil for registered pipeline")
	}
	if got.ID != "test-pipeline" {
		t.Errorf("Get returned wrong pipeline: %v", got.ID)
	}

	// 测试存在性
	if !registry.Exists("test-pipeline") {
		t.Error("Exists should return true for registered pipeline")
	}
	if registry.Exists("nonexistent") {
		t.Error("Exists should return false for nonexistent pipeline")
	}

	// 测试重复注册
	pipeline2 := &AgentPatternPipeline{
		ID:      "test-pipeline",
		Name:    "Test Pipeline 2",
		Version: "2.0",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
		},
		GlobalConfig: DefaultGlobalConfig(),
	}
	err = registry.Register(pipeline2)
	if err != nil {
		t.Logf("Re-register should succeed (updates): %v", err)
	}

	// 测试nil注册
	err = registry.Register(nil)
	if err == nil {
		t.Error("Register nil should fail")
	}

	// 测试列表
	list := registry.List()
	if len(list) != 1 {
		t.Errorf("Expected 1 pipeline in list, got %d", len(list))
	}

	// 测试删除
	registry.Remove("test-pipeline")
	if registry.Exists("test-pipeline") {
		t.Error("Pipeline should not exist after removal")
	}
}

func TestDefaultGlobalConfig(t *testing.T) {
	config := DefaultGlobalConfig()
	if config.Timeout != 120 {
		t.Errorf("Timeout = %v, want 120", config.Timeout)
	}
	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %v, want 3", config.MaxRetries)
	}
	if !config.BypassOnError {
		t.Error("BypassOnError should be true")
	}
	if config.ParallelLimit != 4 {
		t.Errorf("ParallelLimit = %v, want 4", config.ParallelLimit)
	}
}

func TestPipelineInputOutput(t *testing.T) {
	input := &PipelineInput{
		Content:   "test content",
		UserID:    "user123",
		SessionID: "session456",
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	if input.Content != "test content" {
		t.Errorf("Content = %v", input.Content)
	}

	output := &PipelineOutput{
		Content: "response",
		Metadata: map[string]interface{}{
			"tokens": 100,
		},
	}

	if output.Content != "response" {
		t.Errorf("Content = %v", output.Content)
	}
}
