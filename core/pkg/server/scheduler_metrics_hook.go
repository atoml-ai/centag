package server

import (
	"centag/core/pkg/pipeline"
	"centag/core/pkg/scheduler"
)

func wireSchedulerMetricsFeedback(sched *scheduler.Scheduler) {
	if sched == nil {
		return
	}
	pipeline.RecordSchedulerMetrics = func(backendID, model string, latencyMs int64, success bool) {
		sched.RecordRequestResult(backendID, model, latencyMs, success, 0)
	}
}