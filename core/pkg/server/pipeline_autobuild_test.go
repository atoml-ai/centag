package server

import (
	"fmt"
	"strings"
	"testing"

	"centag/core/pkg/backend"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/scheduler"
)

type fakeAutoBuildScheduler struct {
	decisionByKeyword map[string]*scheduler.ScheduleDecision
}

func (f *fakeAutoBuildScheduler) ScheduleWithStrategy(question string, requestedModel string, strategy string) (*scheduler.ScheduleDecision, error) {
	for keyword, decision := range f.decisionByKeyword {
		if keyword != "" && containsFold(question, keyword) {
			return decision, nil
		}
	}
	if d, ok := f.decisionByKeyword["*"]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("no decision for question")
}

func (f *fakeAutoBuildScheduler) ScheduleWithCategory(category string, strategy string) (*scheduler.ScheduleDecision, error) {
	if d, ok := f.decisionByKeyword[category]; ok {
		return d, nil
	}
	if d, ok := f.decisionByKeyword["*"]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("no decision for category: %s", category)
}

func (f *fakeAutoBuildScheduler) UpdateBackendCache(backends []*backend.BackendConfig) {}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (sub == "" || stringContainsFold(s, sub))
}

func stringContainsFold(s, sub string) bool {
	// small helper to keep tests dependency-free
	for i := 0; i+len(sub) <= len(s); i++ {
		if equalFoldASCII(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		aa := a[i]
		bb := b[i]
		if aa >= 'A' && aa <= 'Z' {
			aa += 'a' - 'A'
		}
		if bb >= 'A' && bb <= 'Z' {
			bb += 'a' - 'A'
		}
		if aa != bb {
			return false
		}
	}
	return true
}

func TestBuildAutoRoutePlan_NoRouterNode(t *testing.T) {
	handler := &PipelineHandler{
		autoBuildScheduler: &fakeAutoBuildScheduler{
			decisionByKeyword: map[string]*scheduler.ScheduleDecision{
				"*": {RecommendedBackendID: "some-backend", RecommendedModel: "some-model", Reason: "default"},
			},
		},
	}

	p := &pipeline.AgentPatternPipeline{
		ID:   "no-router",
		Name: "No Router",
		Nodes: []pipeline.PipelineNodeConfig{
			{ID: "gen1", Type: pipeline.NodeTypeGenerator, Config: pipeline.NodeConfig{Backend: "old", Model: "old"}},
		},
	}

	updates, warnings, err := handler.buildAutoRoutePlan(p, "balance", nil, 0)
	if err == nil {
		t.Fatal("expected error for pipeline without router node")
	}
	if !strings.Contains(err.Error(), "no router node") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates, got %d", len(updates))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %v", warnings)
	}
}

func TestBuildAutoRoutePlan_EmptyRoutes(t *testing.T) {
	handler := &PipelineHandler{
		autoBuildScheduler: &fakeAutoBuildScheduler{
			decisionByKeyword: map[string]*scheduler.ScheduleDecision{
				"*": {RecommendedBackendID: "some-backend", RecommendedModel: "some-model", Reason: "default"},
			},
		},
	}

	p := &pipeline.AgentPatternPipeline{
		ID:   "empty-routes",
		Name: "Empty Routes",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "classifier",
				Type: pipeline.NodeTypeRouter,
				Config: pipeline.NodeConfig{
					CustomConfig: map[string]interface{}{
						"routes": map[string]interface{}{},
					},
				},
			},
		},
	}

	updates, warnings, err := handler.buildAutoRoutePlan(p, "balance", nil, 0)
	if err == nil {
		t.Fatal("expected error for empty routes")
	}
	// findRouterNodeAndRoutes returns nil for empty routes, triggering "no router node" error
	if !strings.Contains(err.Error(), "no router node") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates, got %d", len(updates))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %v", warnings)
	}
}

func TestBuildAutoRoutePlan_NoBackendDecision(t *testing.T) {
	handler := &PipelineHandler{
		autoBuildScheduler: &fakeAutoBuildScheduler{
			decisionByKeyword: map[string]*scheduler.ScheduleDecision{
				"code":      {RecommendedBackendID: "", RecommendedModel: "", Reason: "no decision"},
				"translate": {RecommendedBackendID: "ppinfra", RecommendedModel: "qwen3.5", Reason: "translation optimized"},
			},
		},
	}

	p := &pipeline.AgentPatternPipeline{
		ID:   "router-mode",
		Name: "router-mode",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "classifier",
				Type: pipeline.NodeTypeRouter,
				Config: pipeline.NodeConfig{
					CustomConfig: map[string]interface{}{
						"routes": map[string]interface{}{
							"code":      "code-generator",
							"translate": "translate-generator",
						},
					},
				},
			},
			{
				ID:   "code-generator",
				Type: pipeline.NodeTypeGenerator,
				Config: pipeline.NodeConfig{
					Backend: "old-backend",
					Model:   "old-model",
				},
			},
			{
				ID:   "translate-generator",
				Type: pipeline.NodeTypeGenerator,
				Config: pipeline.NodeConfig{
					Backend: "old-backend",
					Model:   "old-model",
				},
			},
		},
	}

	// 使用 fast 策略直接按 category 匹配，避免 sampleQuestionForCategory 转换干扰
	updates, warnings, err := handler.buildAutoRoutePlan(p, "fast", nil, 0)
	if err != nil {
		t.Fatalf("buildAutoRoutePlan failed: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for no-backend-decision nodes")
	}
	foundWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "no backend decision") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("expected warning about 'no backend decision', got %v", warnings)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update (translate only), got %d", len(updates))
	}
	if updates[0].TargetNode != "translate-generator" {
		t.Fatalf("expected update for translate-generator, got %+v", updates[0])
	}
}

func TestBuildAutoRoutePlan_CanaryLimitStopsEarly(t *testing.T) {
	handler := &PipelineHandler{
		autoBuildScheduler: &fakeAutoBuildScheduler{
			decisionByKeyword: map[string]*scheduler.ScheduleDecision{
				"*": {RecommendedBackendID: "some-backend", RecommendedModel: "some-model", Reason: "default"},
			},
		},
	}

	p := &pipeline.AgentPatternPipeline{
		ID:   "router-mode",
		Name: "router-mode",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "classifier",
				Type: pipeline.NodeTypeRouter,
				Config: pipeline.NodeConfig{
					CustomConfig: map[string]interface{}{
						"routes": map[string]interface{}{
							"code":      "code-generator",
							"translate": "translate-generator",
							"summary":   "summary-generator",
						},
					},
				},
			},
			{
				ID:   "code-generator",
				Type: pipeline.NodeTypeGenerator,
				Config: pipeline.NodeConfig{Backend: "old", Model: "old"},
			},
			{
				ID:   "translate-generator",
				Type: pipeline.NodeTypeGenerator,
				Config: pipeline.NodeConfig{Backend: "old", Model: "old"},
			},
			{
				ID:   "summary-generator",
				Type: pipeline.NodeTypeGenerator,
				Config: pipeline.NodeConfig{Backend: "old", Model: "old"},
			},
		},
	}

	updates, warnings, err := handler.buildAutoRoutePlan(p, "balance", nil, 1)
	if err != nil {
		t.Fatalf("buildAutoRoutePlan failed: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected exactly 1 update (canary limit), got %d", len(updates))
	}
	_ = warnings
	// 验证 pipeline 只更新了第一个节点
	changedCount := 0
	for _, node := range p.Nodes {
		if node.Type == pipeline.NodeTypeGenerator && node.Config.Backend != "old" {
			changedCount++
		}
	}
	if changedCount != 1 {
		t.Fatalf("expected only 1 generator changed (canary limit), got %d modified", changedCount)
	}
}

func TestBuildAutoRoutePlan(t *testing.T) {
	handler := &PipelineHandler{
		autoBuildScheduler: &fakeAutoBuildScheduler{
			decisionByKeyword: map[string]*scheduler.ScheduleDecision{
				"Go": {
					RecommendedBackendID: "bigmodel",
					RecommendedModel:     "glm-4-flash",
					Reason:               "code optimized",
				},
				"翻译": {
					RecommendedBackendID: "ppinfra",
					RecommendedModel:     "qwen3.5",
					Reason:               "translation optimized",
				},
				"*": {
					RecommendedBackendID: "bigmodel",
					RecommendedModel:     "glm-4-flash",
					Reason:               "default",
				},
			},
		},
	}

	p := &pipeline.AgentPatternPipeline{
		ID:   "router-mode",
		Name: "router-mode",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "classifier",
				Type: pipeline.NodeTypeRouter,
				Config: pipeline.NodeConfig{
					CustomConfig: map[string]interface{}{
						"routes": map[string]interface{}{
							"code":      "code-generator",
							"translate": "translate-generator",
						},
					},
				},
			},
			{
				ID:   "code-generator",
				Type: pipeline.NodeTypeGenerator,
				Config: pipeline.NodeConfig{
					Backend: "old-backend",
					Model:   "old-model",
				},
			},
			{
				ID:   "translate-generator",
				Type: pipeline.NodeTypeGenerator,
				Config: pipeline.NodeConfig{
					Backend: "old-backend",
					Model:   "old-model",
				},
			},
		},
	}

	updates, warnings, err := handler.buildAutoRoutePlan(p, "balance", nil, 0)
	if err != nil {
		t.Fatalf("buildAutoRoutePlan failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	if p.Nodes[1].Config.Backend == "old-backend" {
		t.Fatalf("expected code generator backend updated")
	}
	if p.Nodes[2].Config.Backend == "old-backend" {
		t.Fatalf("expected translate generator backend updated")
	}
}

