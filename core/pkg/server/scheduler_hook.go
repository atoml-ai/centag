package server

import (
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/scheduler"
)

func buildScheduler(cfg *config.Config, backendMgr *backend.Manager) *scheduler.Scheduler {
	if backendMgr == nil {
		return nil
	}

	schedCfg := scheduler.DefaultSchedulerConfig()
	if cfg != nil {
		schedCfg.IntentClassifier.Enabled = cfg.Scheduler.EnableIntentRecognition
		if cfg.Scheduler.AnalyzerModel != "" {
			schedCfg.IntentClassifier.LocalModel = cfg.Scheduler.AnalyzerModel
		}
	}

	sched := scheduler.NewScheduler(schedCfg, backendMgr)

	selector := scheduler.NewTaskTypeSelector()
	if cfg != nil {
		for _, ts := range cfg.Scheduler.TaskStrategies {
			if ts.RecommendedBackend == "" {
				continue
			}
			taskType := scheduler.TaskType(ts.TaskType)
			selector.TaskPriorities[taskType] = append(selector.TaskPriorities[taskType], ts.RecommendedBackend)
		}
	}
	sched.SetSelector(selector)
	if cfg != nil {
		sched.SetScorerDefaultWeights(cfg.Scheduler.Weights)
	}

	return sched
}

func wireSchedulerBackend(sched *scheduler.Scheduler) {
	if sched == nil {
		return
	}
	pipeline.ScheduleBackend = func(req pipeline.ScheduleRequest) (*pipeline.ScheduleResult, error) {
		requestedModel := req.RequestedModel
		// 节点未指定模型时，用系统默认模型作为调度偏好（与 proxy-config.yaml 对齐）
		if requestedModel == "" {
			if cfg := config.Get(); cfg != nil {
				requestedModel = cfg.Proxy.DefaultModel
			}
		}
		decision, err := sched.ScheduleWithStrategy(req.Question, requestedModel, req.Strategy)
		if err != nil {
			return nil, err
		}
		if decision == nil {
			return &pipeline.ScheduleResult{}, nil
		}
		result := &pipeline.ScheduleResult{
			BackendID:          decision.RecommendedBackendID,
			Model:              decision.RecommendedModel,
			Reason:             decision.Reason,
			EstimatedCost:      decision.EstimatedCost,
			EstimatedLatencyMs: decision.EstimatedLatencyMs,
		}
		if decision.Intent != nil {
			result.TaskType = string(decision.Intent.TaskType)
		}
		return result, nil
	}
}