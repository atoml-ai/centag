package pipeline

import (
	"context"
	"testing"
)

func TestSchedulerNode_Execute(t *testing.T) {
	ScheduleBackend = func(req ScheduleRequest) (*ScheduleResult, error) {
		if req.Question != "写一段 Python 代码" {
			t.Errorf("unexpected question: %q", req.Question)
		}
		return &ScheduleResult{
			BackendID: "bigmodel",
			Model:     "glm-4-flash",
			Reason:    "代码生成任务",
			TaskType:  "code_generation",
		}, nil
	}
	defer func() { ScheduleBackend = nil }()

	node, err := NewSchedulerNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"strategy": "balance",
		},
	})
	if err != nil {
		t.Fatalf("NewSchedulerNode: %v", err)
	}

	sched := node.(*SchedulerNode)
	sched.BaseNode.id = "scheduler"

	output, err := sched.Execute(context.Background(), &NodeInput{
		Content: "写一段 Python 代码",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if output.Metadata["backend_id"] != "bigmodel" {
		t.Errorf("backend_id = %v, want bigmodel", output.Metadata["backend_id"])
	}
	if output.Metadata["model"] != "glm-4-flash" {
		t.Errorf("model = %v, want glm-4-flash", output.Metadata["model"])
	}
	if output.Metadata["scheduler_decision"] != true {
		t.Errorf("scheduler_decision = %v, want true", output.Metadata["scheduler_decision"])
	}
}

func TestSchedulerNode_UnwiredHook(t *testing.T) {
	ScheduleBackend = nil
	node, _ := NewSchedulerNode(NodeConfig{})
	sched := node.(*SchedulerNode)
	sched.BaseNode.id = "scheduler"

	_, err := sched.Execute(context.Background(), &NodeInput{Content: "hello"})
	if err == nil {
		t.Fatal("expected error when ScheduleBackend is nil")
	}
}

func TestApplySchedulingOverrides_GeoRouter(t *testing.T) {
	nodeConfig := NodeConfig{Backend: "fallback", Model: "fallback-model"}
	input := &NodeInput{
		UpstreamResults: map[string]*NodeOutput{
			"geo_router": {
				Metadata: map[string]interface{}{
					"geo_decision": true,
					"backend_id":   "openai-us",
					"model":        "gpt-4o",
				},
			},
		},
	}
	applySchedulingOverrides(&nodeConfig, input, nil)
	if nodeConfig.Backend != "openai-us" {
		t.Errorf("backend = %q, want openai-us", nodeConfig.Backend)
	}
}

func TestApplySchedulingOverrides(t *testing.T) {
	nodeConfig := NodeConfig{Backend: "fallback", Model: "fallback-model"}
	input := &NodeInput{
		UpstreamResults: map[string]*NodeOutput{
			"scheduler": {
				Metadata: map[string]interface{}{
					"scheduler_decision": true,
					"backend_id":         "ppinfra",
					"model":              "deepseek-v3",
				},
			},
		},
	}
	applySchedulingOverrides(&nodeConfig, input, nil)
	if nodeConfig.Backend != "ppinfra" {
		t.Errorf("backend = %q, want ppinfra", nodeConfig.Backend)
	}
	if nodeConfig.Model != "deepseek-v3" {
		t.Errorf("model = %q, want deepseek-v3", nodeConfig.Model)
	}
}

func TestApplySchedulingOverrides_SkipsFallbackNode(t *testing.T) {
	// 降级节点使用 {{system.fallback_backend}} / {{system.fallback_model}}，不能被调度决策覆盖
	nodeConfig := NodeConfig{
		Backend: "{{system.fallback_backend}}",
		Model:   "{{system.fallback_model}}",
	}
	input := &NodeInput{
		UpstreamResults: map[string]*NodeOutput{
			"scheduler": {
				Metadata: map[string]interface{}{
					"scheduler_decision": true,
					"backend_id":         "ppinfra",
					"model":              "deepseek-v3",
				},
			},
		},
	}
	applySchedulingOverrides(&nodeConfig, input, nil)
	if nodeConfig.Backend != "{{system.fallback_backend}}" {
		t.Errorf("fallback node backend was overridden to %q", nodeConfig.Backend)
	}
	if nodeConfig.Model != "{{system.fallback_model}}" {
		t.Errorf("fallback node model was overridden to %q", nodeConfig.Model)
	}
}

func TestIsFallbackNodeConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  NodeConfig
		want bool
	}{
		{"nil", NodeConfig{}, false},
		{"is_fallback true", NodeConfig{CustomConfig: map[string]interface{}{"is_fallback": true}}, true},
		{"is_fallback string", NodeConfig{CustomConfig: map[string]interface{}{"is_fallback": "true"}}, true},
		{"is_fallback false", NodeConfig{CustomConfig: map[string]interface{}{"is_fallback": false}}, false},
		{"fallback backend template", NodeConfig{Backend: "{{system.fallback_backend}}"}, true},
		{"fallback model template", NodeConfig{Model: "{{system.fallback_model}}"}, true},
		{"default backend template", NodeConfig{Backend: "{{system.default_backend}}"}, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFallbackNodeConfig(&tt.cfg); got != tt.want {
				t.Errorf("isFallbackNodeConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}