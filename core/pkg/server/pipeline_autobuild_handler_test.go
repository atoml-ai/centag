package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/internal/auth"
	"centag/core/pkg/backend"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/scheduler"

	"github.com/gin-gonic/gin"
)

type strategyEchoScheduler struct{}

func (s *strategyEchoScheduler) ScheduleWithStrategy(question string, requestedModel string, strategy string) (*scheduler.ScheduleDecision, error) {
	return &scheduler.ScheduleDecision{
		RecommendedBackendID: "backend-" + strategy,
		RecommendedModel:     "model-" + strategy,
		Reason:               "strategy=" + strategy,
		Intent: &scheduler.ClassificationResult{
			TaskType:   scheduler.TaskCodeGeneration,
			Confidence: 0.9,
		},
	}, nil
}

func (s *strategyEchoScheduler) ScheduleWithCategory(category string, strategy string) (*scheduler.ScheduleDecision, error) {
	if strategy != "fast" {
		return &scheduler.ScheduleDecision{
			RecommendedBackendID: "backend-" + strategy,
			RecommendedModel:     "model-" + strategy,
			Reason:               "category-strategy=" + strategy,
		}, nil
	}
	return &scheduler.ScheduleDecision{
		RecommendedBackendID: "backend-" + category,
		RecommendedModel:     "model-" + category,
		Reason:               "category=" + category,
	}, nil
}

func (s *strategyEchoScheduler) UpdateBackendCache(backends []*backend.BackendConfig) {}

type countingScheduler struct {
	count int
}

func (s *countingScheduler) ScheduleWithStrategy(question string, requestedModel string, strategy string) (*scheduler.ScheduleDecision, error) {
	s.count++
	return &scheduler.ScheduleDecision{
		RecommendedBackendID: "backend-" + strategy,
		RecommendedModel:     "model-" + strategy,
		Reason:               "strategy=" + strategy,
	}, nil
}

func (s *countingScheduler) ScheduleWithCategory(category string, strategy string) (*scheduler.ScheduleDecision, error) {
	s.count++
	return &scheduler.ScheduleDecision{
		RecommendedBackendID: "backend-" + strategy,
		RecommendedModel:     "model-" + strategy,
		Reason:               "category=" + category,
	}, nil
}

func (s *countingScheduler) UpdateBackendCache(backends []*backend.BackendConfig) {}

func TestAutoBuildPipeline_StrategiesIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := pipeline.NewPipelineRegistry()
	p := &pipeline.AgentPatternPipeline{
		ID:           "router-mode",
		Name:         "router-mode",
		Version:      "1.0",
		ShortcutCode: "#r",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "classifier",
				Type: pipeline.NodeTypeRouter,
				Config: pipeline.NodeConfig{
					CustomConfig: map[string]interface{}{
						"default_route": "chat-generator",
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
	if err := registry.Register(p); err != nil {
		t.Fatalf("register pipeline: %v", err)
	}

	handler := NewPipelineHandler(nil, pipeline.NewNodeRegistry(), registry, nil, nil)
	handler.SetAutoBuildScheduler(&strategyEchoScheduler{})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, auth.RoleAdmin)
		c.Next()
	})
	router.POST("/api/v1/pipelines/:id/auto-build", handler.AutoBuildPipeline)

	testCases := []string{"balance", "cost", "quality"}
	for _, strategy := range testCases {
		t.Run(strategy, func(t *testing.T) {
			body := map[string]interface{}{
				"strategy": strategy,
				"dry_run":  true,
			}
			payload, _ := json.Marshal(body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/router-mode/auto-build", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
			}

			var resp struct {
				Success bool `json:"success"`
				Data    struct {
					Strategy string `json:"strategy"`
					DryRun   bool   `json:"dry_run"`
					Updates  []struct {
						NewBackend string `json:"new_backend"`
						NewModel   string `json:"new_model"`
					} `json:"updates"`
				} `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if !resp.Success {
				t.Fatalf("expected success=true")
			}
			if resp.Data.Strategy != strategy {
				t.Fatalf("expected strategy %s, got %s", strategy, resp.Data.Strategy)
			}
			if !resp.Data.DryRun {
				t.Fatalf("expected dry_run=true")
			}
			if len(resp.Data.Updates) == 0 {
				t.Fatalf("expected updates for strategy=%s", strategy)
			}
			for _, u := range resp.Data.Updates {
				if u.NewBackend != "backend-"+strategy {
					t.Fatalf("unexpected new_backend: %s", u.NewBackend)
				}
				if u.NewModel != "model-"+strategy {
					t.Fatalf("unexpected new_model: %s", u.NewModel)
				}
			}
		})
	}
}

func TestAutoBuildPipeline_FullApply(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := pipeline.NewPipelineRegistry()
	p := &pipeline.AgentPatternPipeline{
		ID:           "router-mode",
		Name:         "router-mode",
		Version:      "1.0",
		ShortcutCode: "#r",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "classifier",
				Type: pipeline.NodeTypeRouter,
				Config: pipeline.NodeConfig{
					CustomConfig: map[string]interface{}{
						"default_route": "chat-generator",
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
	if err := registry.Register(p); err != nil {
		t.Fatalf("register pipeline: %v", err)
	}

	handler := NewPipelineHandler(nil, pipeline.NewNodeRegistry(), registry, nil, nil)
	handler.SetAutoBuildScheduler(&strategyEchoScheduler{})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, auth.RoleAdmin)
		c.Next()
	})
	router.POST("/api/v1/pipelines/:id/auto-build", handler.AutoBuildPipeline)

	applyPayload, _ := json.Marshal(map[string]interface{}{
		"strategy": "balance",
		"apply":    true,
	})
	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/router-mode/auto-build", bytes.NewReader(applyPayload))
	applyReq.Header.Set("Content-Type", "application/json")
	applyW := httptest.NewRecorder()
	router.ServeHTTP(applyW, applyReq)
	if applyW.Code != http.StatusOK {
		t.Fatalf("auto-build full apply expected 200, got %d body=%s", applyW.Code, applyW.Body.String())
	}

	var applyResp struct {
		Success bool `json:"success"`
		Data    struct {
			Applied bool                   `json:"applied"`
			Canary  bool                   `json:"canary"`
			Updates []routeAutoBuildUpdate `json:"updates"`
		} `json:"data"`
	}
	if err := json.Unmarshal(applyW.Body.Bytes(), &applyResp); err != nil {
		t.Fatalf("unmarshal apply response: %v", err)
	}
	if !applyResp.Success || !applyResp.Data.Applied {
		t.Fatalf("expected applied success response")
	}
	if applyResp.Data.Canary {
		t.Fatalf("expected non-canary (full) apply, got canary=true")
	}
	if len(applyResp.Data.Updates) != 2 {
		t.Fatalf("expected 2 updates (all generators), got %d", len(applyResp.Data.Updates))
	}

	// 验证所有 generator 节点都被更新
	updated := registry.Get("router-mode")
	if updated == nil {
		t.Fatal("updated pipeline not found")
	}
	updatedCount := 0
	for _, node := range updated.Nodes {
		if node.Type != pipeline.NodeTypeGenerator {
			continue
		}
		if node.Config.Backend == "backend-balance" && node.Config.Model == "model-balance" {
			updatedCount++
		}
	}
	if updatedCount != 2 {
		t.Fatalf("expected 2 generators updated with new values, got %d", updatedCount)
	}
}

func TestAutoBuildPipeline_InvalidStrategy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := pipeline.NewPipelineRegistry()
	p := &pipeline.AgentPatternPipeline{
		ID:   "router-mode",
		Name: "router-mode",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "classifier",
				Type: pipeline.NodeTypeRouter,
				Config: pipeline.NodeConfig{
					CustomConfig: map[string]interface{}{
						"routes": map[string]interface{}{"code": "code-generator"},
					},
				},
			},
		},
	}
	if err := registry.Register(p); err != nil {
		t.Fatalf("register pipeline: %v", err)
	}

	handler := NewPipelineHandler(nil, pipeline.NewNodeRegistry(), registry, nil, nil)
	handler.SetAutoBuildScheduler(&strategyEchoScheduler{})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, auth.RoleAdmin)
		c.Next()
	})
	router.POST("/api/v1/pipelines/:id/auto-build", handler.AutoBuildPipeline)

	body, _ := json.Marshal(map[string]interface{}{
		"strategy": "invalid-strategy",
		"dry_run":  true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/router-mode/auto-build", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid strategy, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAutoBuildPipeline_NonRouterPipelineRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := pipeline.NewPipelineRegistry()
	p := &pipeline.AgentPatternPipeline{
		ID:   "translator",
		Name: "translator",
		Nodes: []pipeline.PipelineNodeConfig{
			{ID: "gen1", Type: pipeline.NodeTypeGenerator, Config: pipeline.NodeConfig{Backend: "b", Model: "m"}},
		},
	}
	if err := registry.Register(p); err != nil {
		t.Fatalf("register pipeline: %v", err)
	}

	handler := NewPipelineHandler(nil, pipeline.NewNodeRegistry(), registry, nil, nil)
	handler.SetAutoBuildScheduler(&strategyEchoScheduler{})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, auth.RoleAdmin)
		c.Next()
	})
	router.POST("/api/v1/pipelines/:id/auto-build", handler.AutoBuildPipeline)

	body, _ := json.Marshal(map[string]interface{}{
		"strategy": "balance",
		"dry_run":  true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/translator/auto-build", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-router pipeline, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Success  bool     `json:"success"`
		Error    string   `json:"error"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !strings.Contains(resp.Error, "no router node") {
		t.Fatalf("expected error about missing router node, got: %s", resp.Error)
	}
}

func TestAutoBuildPipeline_CanaryApplyAndRollback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := pipeline.NewPipelineRegistry()
	p := &pipeline.AgentPatternPipeline{
		ID:           "router-mode",
		Name:         "router-mode",
		Version:      "1.0",
		ShortcutCode: "#r",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "classifier",
				Type: pipeline.NodeTypeRouter,
				Config: pipeline.NodeConfig{
					CustomConfig: map[string]interface{}{
						"default_route": "chat-generator",
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
	if err := registry.Register(p); err != nil {
		t.Fatalf("register pipeline: %v", err)
	}

	handler := NewPipelineHandler(nil, pipeline.NewNodeRegistry(), registry, nil, nil)
	handler.SetAutoBuildScheduler(&strategyEchoScheduler{})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, auth.RoleAdmin)
		c.Next()
	})
	router.POST("/api/v1/pipelines/:id/auto-build", handler.AutoBuildPipeline)
	router.POST("/api/v1/pipelines/:id/auto-build/rollback", handler.AutoBuildRollback)

	applyPayload, _ := json.Marshal(map[string]interface{}{
		"strategy": "balance",
		"apply":    true,
		"canary":   true,
	})
	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/router-mode/auto-build", bytes.NewReader(applyPayload))
	applyReq.Header.Set("Content-Type", "application/json")
	applyW := httptest.NewRecorder()
	router.ServeHTTP(applyW, applyReq)
	if applyW.Code != http.StatusOK {
		t.Fatalf("auto-build apply expected 200, got %d body=%s", applyW.Code, applyW.Body.String())
	}
	var applyResp struct {
		Success bool `json:"success"`
		Data    struct {
			Applied    bool                   `json:"applied"`
			Canary     bool                   `json:"canary"`
			MaxUpdates int                    `json:"max_updates"`
			Updates    []routeAutoBuildUpdate `json:"updates"`
		} `json:"data"`
	}
	if err := json.Unmarshal(applyW.Body.Bytes(), &applyResp); err != nil {
		t.Fatalf("unmarshal apply response: %v", err)
	}
	if !applyResp.Success || !applyResp.Data.Applied {
		t.Fatalf("expected applied success response")
	}
	if !applyResp.Data.Canary || applyResp.Data.MaxUpdates != 1 {
		t.Fatalf("expected canary max_updates=1, got canary=%v max_updates=%d", applyResp.Data.Canary, applyResp.Data.MaxUpdates)
	}
	if len(applyResp.Data.Updates) != 1 {
		t.Fatalf("expected exactly 1 canary update, got %d", len(applyResp.Data.Updates))
	}

	rollbackReq := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/router-mode/auto-build/rollback", bytes.NewReader([]byte(`{}`)))
	rollbackReq.Header.Set("Content-Type", "application/json")
	rollbackW := httptest.NewRecorder()
	router.ServeHTTP(rollbackW, rollbackReq)
	if rollbackW.Code != http.StatusOK {
		t.Fatalf("rollback expected 200, got %d body=%s", rollbackW.Code, rollbackW.Body.String())
	}

	restored := registry.Get("router-mode")
	if restored == nil {
		t.Fatalf("restored pipeline not found")
	}
	for _, node := range restored.Nodes {
		if node.Type != pipeline.NodeTypeGenerator {
			continue
		}
		if node.Config.Backend != "old-backend" || node.Config.Model != "old-model" {
			t.Fatalf("expected rollback to old backend/model, got node=%s backend=%s model=%s", node.ID, node.Config.Backend, node.Config.Model)
		}
	}
}

func TestAutoBuildPipeline_ApplyWithPreviewUpdates_SkipRecompute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := pipeline.NewPipelineRegistry()
	p := &pipeline.AgentPatternPipeline{
		ID:   "router-mode",
		Name: "router-mode",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "classifier",
				Type: pipeline.NodeTypeRouter,
				Config: pipeline.NodeConfig{
					CustomConfig: map[string]interface{}{
						"routes": map[string]interface{}{"code": "code-generator"},
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
		},
	}
	if err := registry.Register(p); err != nil {
		t.Fatalf("register pipeline: %v", err)
	}

	s := &countingScheduler{}
	handler := NewPipelineHandler(nil, pipeline.NewNodeRegistry(), registry, nil, nil)
	handler.SetAutoBuildScheduler(s)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, auth.RoleAdmin)
		c.Next()
	})
	router.POST("/api/v1/pipelines/:id/auto-build", handler.AutoBuildPipeline)
	router.POST("/api/v1/pipelines/:id/auto-build/rollback", handler.AutoBuildRollback)

	body, _ := json.Marshal(map[string]interface{}{
		"strategy": "balance",
		"apply":    true,
		"preview_updates": []map[string]interface{}{
			{
				"category":    "code",
				"target_node": "code-generator",
				"old_backend": "old-backend",
				"old_model":   "old-model",
				"new_backend": "backend-preview",
				"new_model":   "model-preview",
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/router-mode/auto-build", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("apply with preview expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if s.count != 0 {
		t.Fatalf("expected scheduler not called when applying preview updates, got %d", s.count)
	}

	updated := registry.Get("router-mode")
	if updated == nil {
		t.Fatal("updated pipeline not found")
	}
	var target *pipeline.PipelineNodeConfig
	for i := range updated.Nodes {
		if updated.Nodes[i].ID == "code-generator" {
			target = &updated.Nodes[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("code-generator not found")
	}
	if target.Config.Backend != "backend-preview" || target.Config.Model != "model-preview" {
		t.Fatalf("expected preview values applied, got backend=%s model=%s", target.Config.Backend, target.Config.Model)
	}

	rollbackReq := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/router-mode/auto-build/rollback", bytes.NewReader([]byte(`{}`)))
	rollbackReq.Header.Set("Content-Type", "application/json")
	rollbackW := httptest.NewRecorder()
	router.ServeHTTP(rollbackW, rollbackReq)
	if rollbackW.Code != http.StatusOK {
		t.Fatalf("rollback expected 200, got %d body=%s", rollbackW.Code, rollbackW.Body.String())
	}
}
