package pipeline

// RecordSchedulerMetrics optionally feeds production request outcomes back into the
// scheduler scorer (latency / success rate loop). Wired from server startup.
var RecordSchedulerMetrics func(backendID, model string, latencyMs int64, success bool)

// RecordSchedulerMetricsFromOutput feeds pipeline execution stats to the optional scheduler hook.
func RecordSchedulerMetricsFromOutput(output *PipelineOutput) {
	if RecordSchedulerMetrics == nil || output == nil {
		return
	}
	backendID := ""
	model := ""
	if output.Metadata != nil {
		if v, ok := output.Metadata["backend_id"].(string); ok {
			backendID = v
		}
		if v, ok := output.Metadata["model"].(string); ok {
			model = v
		}
	}
	if model == "" && output.ExecutionLog != nil {
		for _, nl := range output.ExecutionLog.NodeLogs {
			if !nl.Success {
				continue
			}
			if nl.NodeType == NodeTypeGenerator || nl.NodeType == NodeTypeTransparentForward {
				if nl.Model != "" {
					model = nl.Model
				}
				break
			}
		}
	}
	if backendID == "" {
		return
	}
	var latencyMs int64
	success := true
	if output.ExecutionLog != nil {
		latencyMs = int64(output.ExecutionLog.Duration)
		success = output.ExecutionLog.Success
	}
	RecordSchedulerMetrics(backendID, model, latencyMs, success)
}