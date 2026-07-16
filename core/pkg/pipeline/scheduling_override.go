package pipeline

// applySchedulingOverrides applies backend/model chosen by an upstream scheduler node.
func applySchedulingOverrides(nodeConfig *NodeConfig, input *NodeInput, execCtx *ExecutionContext) {
	if nodeConfig == nil {
		return
	}

	backendID := ""
	model := ""

	if input != nil {
		if input.Metadata != nil {
			if v, ok := input.Metadata["backend_id"].(string); ok && v != "" {
				backendID = v
			}
			if v, ok := input.Metadata["model"].(string); ok && v != "" {
				model = v
			}
		}
		for _, result := range input.UpstreamResults {
			if result == nil || result.Metadata == nil {
				continue
			}
			scheduled, _ := result.Metadata["scheduler_decision"].(bool)
			geo, _ := result.Metadata["geo_decision"].(bool)
			if !scheduled && !geo {
				continue
			}
			if v, ok := result.Metadata["backend_id"].(string); ok && v != "" {
				backendID = v
			}
			if v, ok := result.Metadata["model"].(string); ok && v != "" {
				model = v
			}
		}
	}

	if execCtx != nil {
		if v, ok := execCtx.GetVariable("backend_id"); ok {
			if s, ok := v.(string); ok && s != "" {
				backendID = s
			}
		}
		if v, ok := execCtx.GetVariable("scheduled_model"); ok {
			if s, ok := v.(string); ok && s != "" {
				model = s
			}
		}
	}

	if backendID != "" {
		nodeConfig.Backend = backendID
	}
	if model != "" {
		nodeConfig.Model = model
	}
}