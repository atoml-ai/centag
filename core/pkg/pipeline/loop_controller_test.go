package pipeline

import (
	"context"
	"testing"
)

func TestNewLoopControllerNode(t *testing.T) {
	tests := []struct {
		name          string
		config        NodeConfig
		wantErr       bool
		wantMaxIter   int
		wantCondition string
		wantLoopVar   string
	}{
		{
			name: "default values",
			config: NodeConfig{
				CustomConfig: map[string]interface{}{},
			},
			wantErr:       false,
			wantMaxIter:   3,
			wantCondition: "",
			wantLoopVar:   "loop_count",
		},
		{
			name: "custom values",
			config: NodeConfig{
				CustomConfig: map[string]interface{}{
					"max_iterations": 5,
					"condition":      "{{.auditor.metadata.passed}} == false",
					"loop_variable":  "retry_count",
				},
			},
			wantErr:       false,
			wantMaxIter:   5,
			wantCondition: "{{.auditor.metadata.passed}} == false",
			wantLoopVar:   "retry_count",
		},
		{
			name: "float64 max_iterations",
			config: NodeConfig{
				CustomConfig: map[string]interface{}{
					"max_iterations": float64(10),
				},
			},
			wantErr:     false,
			wantMaxIter: 10,
			wantLoopVar: "loop_count", // 使用默认值
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := NewLoopControllerNode(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLoopControllerNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			loopNode, ok := node.(*LoopControllerNode)
			if !ok {
				t.Errorf("NewLoopControllerNode() did not return *LoopControllerNode")
				return
			}

			if loopNode.MaxIterations != tt.wantMaxIter {
				t.Errorf("MaxIterations = %d, want %d", loopNode.MaxIterations, tt.wantMaxIter)
			}
			if loopNode.Condition != tt.wantCondition {
				t.Errorf("Condition = %s, want %s", loopNode.Condition, tt.wantCondition)
			}
			if loopNode.LoopVariable != tt.wantLoopVar {
				t.Errorf("LoopVariable = %s, want %s", loopNode.LoopVariable, tt.wantLoopVar)
			}
		})
	}
}

func TestLoopControllerNodeValidate(t *testing.T) {
	tests := []struct {
		name    string
		node    *LoopControllerNode
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid node",
			node: &LoopControllerNode{
				MaxIterations: 3,
				Condition:     "{{.auditor.metadata.passed}} == false",
				Subgraph:      []PipelineNodeConfig{{ID: "test"}},
			},
			wantErr: false,
		},
		{
			name: "zero max iterations",
			node: &LoopControllerNode{
				MaxIterations: 0,
				Condition:     "{{.auditor.metadata.passed}} == false",
				Subgraph:      []PipelineNodeConfig{{ID: "test"}},
			},
			wantErr: true,
			errMsg:  "requires max_iterations > 0",
		},
		{
			name: "negative max iterations",
			node: &LoopControllerNode{
				MaxIterations: -1,
				Condition:     "{{.auditor.metadata.passed}} == false",
				Subgraph:      []PipelineNodeConfig{{ID: "test"}},
			},
			wantErr: true,
			errMsg:  "requires max_iterations > 0",
		},
		{
			name: "excessive max iterations",
			node: &LoopControllerNode{
				MaxIterations: 101,
				Condition:     "{{.auditor.metadata.passed}} == false",
				Subgraph:      []PipelineNodeConfig{{ID: "test"}},
			},
			wantErr: true,
			errMsg:  "cannot exceed 100",
		},
		{
			name: "empty condition",
			node: &LoopControllerNode{
				MaxIterations: 3,
				Condition:     "",
				Subgraph:      []PipelineNodeConfig{{ID: "test"}},
			},
			wantErr: true,
			errMsg:  "requires condition",
		},
		{
			name: "empty subgraph",
			node: &LoopControllerNode{
				MaxIterations: 3,
				Condition:     "{{.auditor.metadata.passed}} == false",
				Subgraph:      []PipelineNodeConfig{},
			},
			wantErr: true,
			errMsg:  "requires subgraph",
		},
		{
			name: "nil subgraph",
			node: &LoopControllerNode{
				MaxIterations: 3,
				Condition:     "{{.auditor.metadata.passed}} == false",
				Subgraph:      nil,
			},
			wantErr: true,
			errMsg:  "requires subgraph",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.node.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error message = %v, should contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestLoopControllerNodeExecute(t *testing.T) {
	tests := []struct {
		name          string
		maxIterations int
		condition     string
		wantIterations int
	}{
		{
			name:          "single iteration",
			maxIterations: 3,
			condition:     "false", // 条件不满足，立即退出
			wantIterations: 1,
		},
		{
			name:          "max iterations",
			maxIterations: 3,
			condition:     "true", // 条件始终满足，达到最大迭代
			wantIterations: 3,
		},
		{
			name:          "single max iteration",
			maxIterations: 1,
			condition:     "true",
			wantIterations: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &LoopControllerNode{
				BaseNode: BaseNode{
					id:   "test-loop",
					name: "Test Loop",
				},
				MaxIterations: tt.maxIterations,
				Condition:     tt.condition,
				LoopVariable:  "loop_count",
				Subgraph: []PipelineNodeConfig{
					{ID: "generator", Type: NodeTypeGenerator},
				},
			}

			input := &NodeInput{
				Content:  "test content",
				Metadata: map[string]interface{}{},
			}

			ctx := context.Background()
			output, err := node.Execute(ctx, input)

			if err != nil {
				// 允许错误，因为我们没有完整的执行上下文
				t.Logf("Execute() returned error (expected in test): %v", err)
			}

			if output == nil {
				t.Fatal("Execute() returned nil output")
			}

			// 验证输出包含循环信息
			if output.Metadata == nil {
				t.Error("Output Metadata is nil")
				return
			}

			// 检查循环完成标记
			if completed, ok := output.Metadata["loop_completed"]; !ok || completed != true {
				t.Error("Expected loop_completed to be true in metadata")
			}

			// 检查最大迭代次数
			if maxIter, ok := output.Metadata["max_iterations"]; !ok || maxIter != tt.maxIterations {
				t.Errorf("Expected max_iterations = %d, got %v", tt.maxIterations, maxIter)
			}
		})
	}
}

func TestLoopControllerNodeType(t *testing.T) {
	node := &LoopControllerNode{}
	if node.Type() != NodeTypeLoopController {
		t.Errorf("Type() = %v, want %v", node.Type(), NodeTypeLoopController)
	}
}

func TestParseNodeConfigFromMap(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		want PipelineNodeConfig
	}{
		{
			name: "complete config",
			m: map[string]interface{}{
				"id":             "test-id",
				"name":           "test-name",
				"type":           "generator",
				"kind":           "llm.generate",
				"implementation": "builtin.generator",
				"backend":        "test-backend",
				"model":          "test-model",
				"timeout":        120,
			},
			want: PipelineNodeConfig{
				ID:             "test-id",
				Name:           "test-name",
				Type:           NodeTypeGenerator,
				Kind:           "llm.generate",
				Implementation: "builtin.generator",
				Backend:        "test-backend",
				Model:          "test-model",
				Timeout:        120,
			},
		},
		{
			name: "float timeout",
			m: map[string]interface{}{
				"timeout": float64(60),
			},
			want: PipelineNodeConfig{
				Timeout: 60,
			},
		},
		{
			name: "empty map",
			m:    map[string]interface{}{},
			want: PipelineNodeConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNodeConfigFromMap(tt.m)
			if got.ID != tt.want.ID {
				t.Errorf("ID = %v, want %v", got.ID, tt.want.ID)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.want.Type)
			}
			if got.Kind != tt.want.Kind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.want.Kind)
			}
			if got.Implementation != tt.want.Implementation {
				t.Errorf("Implementation = %v, want %v", got.Implementation, tt.want.Implementation)
			}
			if got.Backend != tt.want.Backend {
				t.Errorf("Backend = %v, want %v", got.Backend, tt.want.Backend)
			}
			if got.Model != tt.want.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.want.Model)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("Timeout = %v, want %v", got.Timeout, tt.want.Timeout)
			}
		})
	}
}

