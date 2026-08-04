package pipeline

import "context"

// filterFallbackNodesByCircuit returns fallback node IDs with circuit-open backends removed.
func (e *PipelineEngine) filterFallbackNodesByCircuit(ctx context.Context, graph *ExecutionGraph, fallbackNodes []string) []string {
	if graph == nil || len(fallbackNodes) == 0 || IsCircuitOpen == nil {
		return fallbackNodes
	}

	filtered := make([]string, 0, len(fallbackNodes))
	for _, nodeID := range fallbackNodes {
		node := graph.GetNode(nodeID)
		if node == nil {
			filtered = append(filtered, nodeID)
			continue
		}
		backendID := nodeBackendIDContext(ctx, node.Config, node.Config.Config)
		if isCircuitOpenForBackend(backendID) {
			node.Status = StatusSkipped
			if e.logger != nil {
				e.logger.Warn("fallback node skipped: circuit breaker open",
					"fallback_node_id", nodeID,
					"backend_id", backendID,
				)
			}
			continue
		}
		filtered = append(filtered, nodeID)
	}
	return filtered
}

// executeFallbackGroup 执行单个降级组：主节点失败时按序尝试备用节点（跳过熔断打开的后端）。
// 返回 (groupOK, lastFallbackErr)：
//   - groupOK=true：主节点本已成功，或经降级节点恢复；
//   - groupOK=false：主节点失败且所有降级尝试均未成功。
// 流式/非流式路径共用，避免逻辑分叉。
//
// max_attempts 语义：表示「主节点之外的降级尝试次数上限」，默认按降级节点数自动计算；
// 引擎以「≤ max_attempts-1 的下标」换算为实际尝试次数（保留历史行为）。
func (e *PipelineEngine) executeFallbackGroup(
	ctx context.Context,
	graph *ExecutionGraph,
	execCtx *ExecutionContext,
	pipeline *AgentPatternPipeline,
	fg FallbackGroup,
	primaryNode *ExecutionNode,
) (bool, error) {
	if primaryNode == nil {
		return false, nil
	}

	primaryFailed := primaryNode.Status == StatusFailed || primaryNode.Error != nil
	if !primaryFailed {
		// 主节点成功，跳过降级（组已成功，不能返回 false，否则调用方会误报 all fallback failed）
		for _, fbID := range fg.FallbackNodes {
			if fbNode := graph.GetNode(fbID); fbNode != nil {
				fbNode.Status = StatusSkipped
				e.logger.Info("fallback node skipped (primary succeeded)",
					"primary_node_id", fg.PrimaryNodeID,
					"fallback_node_id", fbID,
				)
			}
		}
		return true, nil
	}

	maxAttempts := fg.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = len(fg.FallbackNodes) + 1
	}
	fallbackSuccess := false
	var lastFallbackErr error
	for attemptIdx, fbID := range e.filterFallbackNodesByCircuit(ctx, graph, fg.FallbackNodes) {
		if attemptIdx >= maxAttempts-1 {
			break
		}
		e.logger.Warn("executing fallback node (primary failed)",
			"primary_node_id", fg.PrimaryNodeID,
			"fallback_node_id", fbID,
			"attempt", attemptIdx+1,
		)
		execErr := e.executeLayerNode(ctx, graph, execCtx, fbID, pipeline)
		if execErr != nil {
			lastFallbackErr = execErr
			e.logger.Warn("fallback node also failed",
				"fallback_node_id", fbID,
				"error", execErr,
			)
			continue
		}
		if fbNode := graph.GetNode(fbID); fbNode != nil && fbNode.Status == StatusSuccess {
			markFallbackGroupOutput(fbNode, fg.PrimaryNodeID)
			// 主节点经降级组恢复：追加成功日志，避免整体 Success 被主节点失败拉成 false
			execCtx.AddNodeLog(NodeExecutionLog{
				NodeID:       fg.PrimaryNodeID,
				NodeType:     primaryNode.Config.Type,
				Success:      true,
				ErrorMessage: "",
			})
			fallbackSuccess = true
			e.logger.Info("fallback node succeeded",
				"fallback_node_id", fbID,
			)
			break
		}
	}
	return fallbackSuccess, lastFallbackErr
}
