package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
)

type autoBuildPipelineRequest struct {
	Strategy       string                 `json:"strategy"`
	DryRun         *bool                  `json:"dry_run,omitempty"`
	Apply          bool                   `json:"apply,omitempty"`
	ProbeBackends  bool                   `json:"probe_backends,omitempty"`
	Canary         bool                   `json:"canary,omitempty"`
	MaxUpdates     int                    `json:"max_updates,omitempty"`
	Categories     []string               `json:"categories,omitempty"`
	PreviewUpdates []routeAutoBuildUpdate `json:"preview_updates,omitempty"`
}

type routeAutoBuildUpdate struct {
	Category        string                 `json:"category"`
	TargetNode      string                 `json:"target_node"`
	OldBackend      string                 `json:"old_backend,omitempty"`
	OldModel        string                 `json:"old_model,omitempty"`
	NewBackend      string                 `json:"new_backend"`
	NewModel        string                 `json:"new_model,omitempty"`
	Reason          string                 `json:"reason,omitempty"`
	Sample          string                 `json:"sample,omitempty"`
	StrategyFactors map[string]interface{} `json:"strategy_factors,omitempty"`
}

type autoBuildRollbackRequest struct {
	DryRun bool `json:"dry_run,omitempty"`
}

type autoBuildRevision struct {
	CreatedAt   time.Time
	Strategy    string
	UpdateCount int
	Pipeline    *pipeline.AgentPatternPipeline
}

// AutoBuildPipeline 自动构建路由配置（MVP）。
// POST /api/v1/pipelines/:id/auto-build
func (h *PipelineHandler) AutoBuildPipeline(c *gin.Context) {
	if auth.GetScopedAccess(c) != auth.AccessGlobal {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "auto-build requires admin scope"})
		return
	}
	if h.autoBuildScheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "auto-build scheduler is not initialized"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	p, err := h.requirePipelineAccess(c, id)
	if err != nil {
		return
	}

	req, err := parseAutoBuildPipelineRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	strategy := normalizeAutoBuildStrategy(req.Strategy)
	if strategy == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "strategy must be one of: balance, cost, quality, latency, fast"})
		return
	}

	// 更新快速匹配器的后端缓存（确保使用最新的后端状态）
	if h.autoBuildBackendMgr != nil {
		allBackends := h.autoBuildBackendMgr.List()
		logger.Infof("[AutoBuild] Updating fast matcher backend cache: %d backends", len(allBackends))
		for _, b := range allBackends {
			logger.Infof("[AutoBuild]   Backend: %s, enabled=%v, type=%s", b.ID, b.Enabled, b.Type)
		}
		h.autoBuildScheduler.UpdateBackendCache(allBackends)
	}

	if req.ProbeBackends {
		if h.autoBuildBackendMgr == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backend manager is not initialized for probing"})
			return
		}
		// Auto-build 是交互式操作：探测仅做连通性检查，不拉取模型列表，且严格限制总时长。
		ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
		defer cancel()
		if _, probeErr := h.autoBuildBackendMgr.ProbeAllBackends(ctx, false); probeErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("probe backends failed: %v", probeErr)})
			return
		}
		if saveErr := h.autoBuildBackendMgr.Save(); saveErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fmt.Sprintf("save backend probe result failed: %v", saveErr)})
			return
		}
		// 更新快速匹配器的后端缓存
		if h.autoBuildScheduler != nil {
			h.autoBuildScheduler.UpdateBackendCache(h.autoBuildBackendMgr.List())
		}
	}

	cloned, err := clonePipeline(p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fmt.Sprintf("clone pipeline failed: %v", err)})
		return
	}

	apply := req.Apply
	if req.DryRun != nil {
		apply = !*req.DryRun
	}
	maxUpdates := normalizeAutoBuildMaxUpdates(req.Canary, req.MaxUpdates)

	var updates []routeAutoBuildUpdate
	var warnings []string
	if apply && len(req.PreviewUpdates) > 0 {
		if applyErr := applyPreviewUpdates(cloned, req.PreviewUpdates); applyErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": applyErr.Error()})
			return
		}
		updates = req.PreviewUpdates
	} else {
		var buildErr error
		updates, warnings, buildErr = h.buildAutoRoutePlan(cloned, strategy, req.Categories, maxUpdates)
		if buildErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": buildErr.Error(), "warnings": warnings})
			return
		}
	}

	if apply {
		h.pushAutoBuildHistory(id, p, strategy, len(updates))
		for i := range cloned.Nodes {
			cloned.Nodes[i].Normalize()
		}
		if regErr := h.pipelineRegistry.Register(cloned); regErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("apply auto-build failed: %v", regErr),
				"data": gin.H{
					"pipeline_id": id,
					"strategy":    strategy,
					"updates":     updates,
					"warnings":    warnings,
				},
			})
			return
		}
		h.syncModesFromRegistry()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"pipeline_id": id,
			"strategy":    strategy,
			"dry_run":     !apply,
			"applied":     apply,
			"canary":      req.Canary,
			"max_updates": maxUpdates,
			"updates":     updates,
			"warnings":    warnings,
			"pipeline":    cloned,
		},
	})
}

func applyPreviewUpdates(p *pipeline.AgentPatternPipeline, updates []routeAutoBuildUpdate) error {
	if p == nil {
		return fmt.Errorf("pipeline is nil")
	}
	if len(updates) == 0 {
		return fmt.Errorf("preview_updates is empty")
	}
	nodesByID := make(map[string]*pipeline.PipelineNodeConfig, len(p.Nodes))
	for i := range p.Nodes {
		nodesByID[p.Nodes[i].ID] = &p.Nodes[i]
	}
	for _, update := range updates {
		targetID := strings.TrimSpace(update.TargetNode)
		if targetID == "" {
			return fmt.Errorf("preview update target_node is required")
		}
		node := nodesByID[targetID]
		if node == nil {
			return fmt.Errorf("preview update target node %q not found", targetID)
		}
		if node.Type != pipeline.NodeTypeGenerator {
			return fmt.Errorf("preview update target node %q is not generator", targetID)
		}
		oldBackend := strings.TrimSpace(update.OldBackend)
		oldModel := strings.TrimSpace(update.OldModel)
		currentBackend := strings.TrimSpace(node.Config.Backend)
		currentModel := strings.TrimSpace(node.Config.Model)
		if oldBackend != "" && currentBackend != oldBackend {
			return fmt.Errorf("preview stale for node %q backend mismatch", targetID)
		}
		if oldModel != "" && currentModel != oldModel {
			return fmt.Errorf("preview stale for node %q model mismatch", targetID)
		}
		newBackend := strings.TrimSpace(update.NewBackend)
		if newBackend == "" {
			return fmt.Errorf("preview update node %q has empty new_backend", targetID)
		}
		node.Config.Backend = newBackend
		node.Backend = newBackend
		if strings.TrimSpace(update.NewModel) != "" {
			newModel := strings.TrimSpace(update.NewModel)
			node.Config.Model = newModel
			node.Model = newModel
		}
	}
	return nil
}

// AutoBuildRollback 一键回滚最近一次 auto-build apply。
// POST /api/v1/pipelines/:id/auto-build/rollback
func (h *PipelineHandler) AutoBuildRollback(c *gin.Context) {
	if auth.GetScopedAccess(c) != auth.AccessGlobal {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "auto-build rollback requires admin scope"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}
	req := autoBuildRollbackRequest{}
	if c.Request != nil && c.Request.Body != nil {
		body, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
		if strings.TrimSpace(string(body)) != "" {
			if err := json.Unmarshal(body, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("invalid request body: %v", err)})
				return
			}
		}
	}

	rev, ok := h.latestAutoBuildRevision(id)
	if !ok || rev.Pipeline == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "no auto-build revision available for rollback"})
		return
	}

	if req.DryRun {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"pipeline_id":   id,
				"dry_run":       true,
				"rollback_from": rev.CreatedAt.Format(time.RFC3339),
				"strategy":      rev.Strategy,
				"update_count":  rev.UpdateCount,
				"pipeline":      rev.Pipeline,
			},
		})
		return
	}

	applied, err := clonePipeline(rev.Pipeline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fmt.Sprintf("clone rollback pipeline failed: %v", err)})
		return
	}
	for i := range applied.Nodes {
		applied.Nodes[i].Normalize()
	}
	if err := h.pipelineRegistry.Register(applied); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fmt.Sprintf("rollback apply failed: %v", err)})
		return
	}
	h.popAutoBuildRevision(id)
	h.syncModesFromRegistry()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"pipeline_id":   id,
			"rolled_back":   true,
			"rollback_from": rev.CreatedAt.Format(time.RFC3339),
			"strategy":      rev.Strategy,
			"update_count":  rev.UpdateCount,
			"pipeline":      applied,
		},
	})
}

func parseAutoBuildPipelineRequest(c *gin.Context) (*autoBuildPipelineRequest, error) {
	req := &autoBuildPipelineRequest{}
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return req, nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body failed: %w", err)
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
	if len(strings.TrimSpace(string(body))) == 0 {
		return req, nil
	}
	if err := json.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}
	return req, nil
}

func normalizeAutoBuildStrategy(strategy string) string {
	s := strings.ToLower(strings.TrimSpace(strategy))
	if s == "" {
		return "balance"
	}
	switch s {
	case "balance", "cost", "quality", "latency", "fast":
		return s
	default:
		return ""
	}
}

func clonePipeline(src *pipeline.AgentPatternPipeline) (*pipeline.AgentPatternPipeline, error) {
	if src == nil {
		return nil, fmt.Errorf("pipeline is nil")
	}
	data, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}
	var dst pipeline.AgentPatternPipeline
	if err := json.Unmarshal(data, &dst); err != nil {
		return nil, err
	}
	return &dst, nil
}

func (h *PipelineHandler) buildAutoRoutePlan(
	p *pipeline.AgentPatternPipeline,
	strategy string,
	categories []string,
	maxUpdates int,
) ([]routeAutoBuildUpdate, []string, error) {
	routerNode, routeMap := findRouterNodeAndRoutes(p)
	if routerNode == nil {
		return nil, nil, fmt.Errorf("pipeline has no router node with routes")
	}
	if len(routeMap) == 0 {
		return nil, nil, fmt.Errorf("router node has empty routes")
	}

	logger.Infof("[AutoBuild] Found router node: %s, routeMap count: %d", routerNode.ID, len(routeMap))
	for k, v := range routeMap {
		logger.Infof("[AutoBuild] Route: %s -> %s", k, v)
	}

	allowedCategories := make(map[string]bool)
	if len(categories) > 0 {
		for _, c := range categories {
			trimmed := strings.TrimSpace(c)
			if trimmed != "" {
				allowedCategories[trimmed] = true
			}
		}
	}

	nodesByID := make(map[string]*pipeline.PipelineNodeConfig, len(p.Nodes))
	for i := range p.Nodes {
		nodesByID[p.Nodes[i].ID] = &p.Nodes[i]
	}

	// 按目标节点去重：多个 category 可能映射到同一个节点，只需要为每个节点做一次决策
	nodeCategories := make(map[string][]string) // targetNodeID -> []category
	for k, v := range routeMap {
		if len(allowedCategories) > 0 && !allowedCategories[k] {
			continue
		}
		targetID := strings.TrimSpace(v)
		nodeCategories[targetID] = append(nodeCategories[targetID], k)
	}

	logger.Infof("[AutoBuild] nodeCategories count: %d", len(nodeCategories))
	for nodeID, cats := range nodeCategories {
		logger.Infof("[AutoBuild] Node %s has %d categories: %v", nodeID, len(cats), cats)
	}

	// 按节点 ID 排序，确保处理顺序稳定
	nodeIDs := make([]string, 0, len(nodeCategories))
	for nodeID := range nodeCategories {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Strings(nodeIDs)

	var updates []routeAutoBuildUpdate
	var warnings []string

	_ = routerNode
	for _, targetID := range nodeIDs {
		if maxUpdates > 0 && len(updates) >= maxUpdates {
			warnings = append(warnings, fmt.Sprintf("canary limit reached, skipped remaining nodes from %q", targetID))
			break
		}
		categories := nodeCategories[targetID]
		targetNode, ok := nodesByID[targetID]
		if !ok || targetNode == nil {
			warnings = append(warnings, fmt.Sprintf("node %q not found (categories: %v)", targetID, categories))
			continue
		}
		if targetNode.Type != pipeline.NodeTypeGenerator {
			warnings = append(warnings, fmt.Sprintf("node %q is not generator (type=%s)", targetID, targetNode.Type))
			continue
		}

		// 使用第一个 category 进行调度决策
		sample := categories[0]
		// auto-build 使用已知 category 做稳定映射，避免每个节点都触发一次分类器调用。
		// 这样 balance/cost/quality/latency 仍按策略生效（在 FastMatcher 中选不同优先级），
		// 但不会引入额外网络延迟和分类误判。
		decision, scheduleErr := h.autoBuildScheduler.ScheduleWithCategory(sample, strategy)
		if scheduleErr == nil && decision == nil {
			// category 未命中时再回退到分类器，保障兼容性。
			sampleQuestion := sampleQuestionForCategory(categories[0])
			sample = sampleQuestion
			decision, scheduleErr = h.autoBuildScheduler.ScheduleWithStrategy(sampleQuestion, "", strategy)
		}
		if scheduleErr != nil {
			warnings = append(warnings, fmt.Sprintf("node %q schedule failed: %v", targetID, scheduleErr))
			continue
		}
		if decision == nil || strings.TrimSpace(decision.RecommendedBackendID) == "" {
			warnings = append(warnings, fmt.Sprintf("node %q has no backend decision", targetID))
			continue
		}

		oldBackend := strings.TrimSpace(targetNode.Config.Backend)
		oldModel := strings.TrimSpace(targetNode.Config.Model)

		backendID := strings.TrimSpace(decision.RecommendedBackendID)
		targetNode.Config.Backend = backendID
		targetNode.Backend = backendID
		if strings.TrimSpace(decision.RecommendedModel) != "" {
			modelID := strings.TrimSpace(decision.RecommendedModel)
			targetNode.Config.Model = modelID
			targetNode.Model = modelID
		} else {
			targetNode.Model = ""
		}

		logger.Infof("[AutoBuild] Node %s: old=%s/%s, new=%s/%s, reason=%s",
			targetID, oldBackend, oldModel, targetNode.Config.Backend, targetNode.Config.Model, decision.Reason)

		// 始终添加到 updates 列表，即使没有变化（用于预览展示）
		updates = append(updates, routeAutoBuildUpdate{
			Category:        strings.Join(categories, ","),
			TargetNode:      targetID,
			OldBackend:      oldBackend,
			OldModel:        oldModel,
			NewBackend:      targetNode.Config.Backend,
			NewModel:        targetNode.Config.Model,
			Reason:          decision.Reason,
			Sample:          sample,
			StrategyFactors: decision.StrategyFactors,
		})
	}

	if len(updates) == 0 && len(warnings) == 0 {
		warnings = append(warnings, "no route updates generated")
	}
	return updates, warnings, nil
}

func normalizeAutoBuildMaxUpdates(canary bool, maxUpdates int) int {
	if maxUpdates < 0 {
		return 0
	}
	if maxUpdates > 0 {
		return maxUpdates
	}
	if canary {
		return 1
	}
	return 0
}

func (h *PipelineHandler) pushAutoBuildHistory(pipelineID string, current *pipeline.AgentPatternPipeline, strategy string, updateCount int) {
	if h == nil || current == nil || strings.TrimSpace(pipelineID) == "" {
		return
	}
	snapshot, err := clonePipeline(current)
	if err != nil {
		return
	}
	h.autoBuildMu.Lock()
	defer h.autoBuildMu.Unlock()
	if h.autoBuildHistory == nil {
		h.autoBuildHistory = make(map[string][]autoBuildRevision)
	}
	rev := autoBuildRevision{
		CreatedAt:   time.Now(),
		Strategy:    strategy,
		UpdateCount: updateCount,
		Pipeline:    snapshot,
	}
	history := append(h.autoBuildHistory[pipelineID], rev)
	const keep = 20
	if len(history) > keep {
		history = history[len(history)-keep:]
	}
	h.autoBuildHistory[pipelineID] = history
}

func (h *PipelineHandler) latestAutoBuildRevision(pipelineID string) (autoBuildRevision, bool) {
	if h == nil {
		return autoBuildRevision{}, false
	}
	h.autoBuildMu.Lock()
	defer h.autoBuildMu.Unlock()
	history := h.autoBuildHistory[pipelineID]
	if len(history) == 0 {
		return autoBuildRevision{}, false
	}
	return history[len(history)-1], true
}

func (h *PipelineHandler) popAutoBuildRevision(pipelineID string) {
	if h == nil {
		return
	}
	h.autoBuildMu.Lock()
	defer h.autoBuildMu.Unlock()
	history := h.autoBuildHistory[pipelineID]
	if len(history) == 0 {
		return
	}
	h.autoBuildHistory[pipelineID] = history[:len(history)-1]
}

func findRouterNodeAndRoutes(p *pipeline.AgentPatternPipeline) (*pipeline.PipelineNodeConfig, map[string]string) {
	if p == nil {
		return nil, nil
	}
	for i := range p.Nodes {
		node := &p.Nodes[i]
		if node.Type != pipeline.NodeTypeRouter {
			continue
		}
		if node.Config.CustomConfig == nil {
			continue
		}
		routes := parseRouteMap(node.Config.CustomConfig["routes"])
		if len(routes) == 0 {
			continue
		}
		return node, routes
	}
	return nil, nil
}

func parseRouteMap(raw interface{}) map[string]string {
	result := map[string]string{}
	switch m := raw.(type) {
	case map[string]interface{}:
		for k, v := range m {
			key := strings.TrimSpace(k)
			val, _ := v.(string)
			val = strings.TrimSpace(val)
			if key != "" && val != "" {
				result[key] = val
			}
		}
	case map[interface{}]interface{}:
		for k, v := range m {
			key := strings.TrimSpace(fmt.Sprintf("%v", k))
			val, _ := v.(string)
			val = strings.TrimSpace(val)
			if key != "" && val != "" {
				result[key] = val
			}
		}
	}
	return result
}

func sampleQuestionForCategory(category string) string {
	c := strings.ToLower(strings.TrimSpace(category))
	switch {
	// 代码生成类
	case strings.Contains(c, "code"), strings.Contains(c, "程序"), strings.Contains(c, "代码"),
		strings.Contains(c, "python"), strings.Contains(c, "java"), strings.Contains(c, "javascript"),
		strings.Contains(c, "go"), strings.Contains(c, "golang"), strings.Contains(c, "rust"),
		strings.Contains(c, "cpp"), strings.Contains(c, "c++"), strings.Contains(c, "typescript"),
		strings.Contains(c, "php"), strings.Contains(c, "ruby"), strings.Contains(c, "swift"),
		strings.Contains(c, "kotlin"), strings.Contains(c, "sql"), strings.Contains(c, "shell"),
		strings.Contains(c, "bash"), strings.Contains(c, "函数"), strings.Contains(c, "方法"),
		strings.Contains(c, "脚本"), strings.Contains(c, "算法"), strings.Contains(c, "类"),
		strings.Contains(c, "接口"), strings.Contains(c, "模块"), strings.Contains(c, "库"),
		strings.Contains(c, "leetcode"), strings.Contains(c, "实现"), strings.Contains(c, "编写"):
		return "请用 Go 实现一个 LRU 缓存并给出测试样例。"
	// 翻译类
	case strings.Contains(c, "translate"), strings.Contains(c, "translation"), strings.Contains(c, "翻译"):
		return "把下面这段中文准确翻译成英文，并保留专业术语。"
	// 摘要类
	case strings.Contains(c, "summary"), strings.Contains(c, "summar"), strings.Contains(c, "摘要"), strings.Contains(c, "总结"):
		return "请总结这篇长文的核心观点，并给出三条关键结论。"
	// 创意写作类
	case strings.Contains(c, "creative"), strings.Contains(c, "story"), strings.Contains(c, "poem"),
		strings.Contains(c, "创意"), strings.Contains(c, "写作"), strings.Contains(c, "小说"),
		strings.Contains(c, "诗歌"), strings.Contains(c, "故事"):
		return "写一个科幻短篇开头，风格克制、细节真实。"
	// 分析推理类
	case strings.Contains(c, "analysis"), strings.Contains(c, "reason"), strings.Contains(c, "math"),
		strings.Contains(c, "推理"), strings.Contains(c, "分析"), strings.Contains(c, "数学"),
		strings.Contains(c, "逻辑"), strings.Contains(c, "数据"), strings.Contains(c, "图表"):
		return "分析这个方案的复杂度与边界条件，并给出改进建议。"
	// 对话类（默认）
	default:
		return "请回答这个通用问题，并保持准确、简洁、可执行。"
	}
}
