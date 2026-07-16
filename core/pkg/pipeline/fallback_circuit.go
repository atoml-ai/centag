package pipeline

// filterFallbackNodesByCircuit returns fallback node IDs with circuit-open backends removed.
func (e *PipelineEngine) filterFallbackNodesByCircuit(graph *ExecutionGraph, fallbackNodes []string) []string {
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
		backendID := nodeBackendID(node.Config, node.Config.Config)
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