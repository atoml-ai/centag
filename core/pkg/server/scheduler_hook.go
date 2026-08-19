package server

import (
	"context"

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

		// 优先使用 SchedulerConfig 中的配置
		if cfg.Scheduler.AnalyzerBackendID != "" {
			schedCfg.IntentClassifier.BackendID = cfg.Scheduler.AnalyzerBackendID
		}
		if cfg.Scheduler.AnalyzerModel != "" {
			schedCfg.IntentClassifier.Model = cfg.Scheduler.AnalyzerModel
		}

		// 如果未配置，尝试从系统变量 system.classify_backend / system.classify_model 获取
		if schedCfg.IntentClassifier.BackendID == "" {
			if v, ok := cfg.ModelVariables.SystemVariables["system.classify_backend"]; ok && v != "" {
				schedCfg.IntentClassifier.BackendID = v
			}
		}
		if schedCfg.IntentClassifier.Model == "" {
			if v, ok := cfg.ModelVariables.SystemVariables["system.classify_model"]; ok && v != "" {
				schedCfg.IntentClassifier.Model = v
			}
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

		// 解析模板变量（如 {{system.classify_backend}}）
		resolvedClassifyBackend, _ := pipeline.ResolveVirtualVarsContext(context.Background(), req.ClassifyBackend, "")
		resolvedClassifyModel, _ := pipeline.ResolveVirtualVarsContext(context.Background(), "", req.ClassifyModel)

		// 如果请求中指定了分类配置，使用带分类配置的调度
		if resolvedClassifyBackend != "" {
			decision, err := sched.ScheduleWithClassifyConfig(req.Question, requestedModel, req.Strategy,
				resolvedClassifyBackend, resolvedClassifyModel, req.ClassifyPrompt)
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

		// 否则使用默认调度
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