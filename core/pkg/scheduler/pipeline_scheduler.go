package scheduler

import (
	"context"
	"fmt"

	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
)

// SchedulerAdapter 将 PipelineScheduler 接入现有调度系统
// 该适配器使用 scheduler 包的 AuditConfig/OptimizeConfig 类型，
// 避免与 pipeline 包的循环依赖
type SchedulerAdapter struct {
	pipelineScheduler *pipeline.PipelineScheduler
	auditConfig      *AuditConfig
	optimizeConfig   *OptimizeConfig
}

// NewSchedulerAdapter 创建调度适配器
func NewSchedulerAdapter(ps *pipeline.PipelineScheduler) *SchedulerAdapter {
	if ps == nil {
		logger.Error("pipeline scheduler cannot be nil")
		return nil
	}
	return &SchedulerAdapter{
		pipelineScheduler: ps,
		auditConfig:      DefaultAuditConfig(),
		optimizeConfig:   DefaultOptimizeConfig(),
	}
}

// SetAuditConfig 设置审核配置
func (sa *SchedulerAdapter) SetAuditConfig(config *AuditConfig) {
	sa.auditConfig = config
}

// SetOptimizeConfig 设置优化配置
func (sa *SchedulerAdapter) SetOptimizeConfig(config *OptimizeConfig) {
	sa.optimizeConfig = config
}

// BuildAuditPipeline 从审核配置构建流水线
func (sa *SchedulerAdapter) BuildAuditPipeline() *pipeline.AgentPatternPipeline {
	cfg := sa.auditConfig

	return &pipeline.AgentPatternPipeline{
		ID:          "audit-mode-pipeline",
		Name:        "Audit Mode Pipeline",
		Description: "自动从审核配置生成的流水线",
		Version:     "1.0",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:      "generator",
				Name:    "Generator",
				Type:    pipeline.NodeTypeGenerator,
				Backend: cfg.ExecutorBackendID,
				Model:   "", // 使用后端默认模型
			},
			{
				ID:        "auditor",
				Name:      "Auditor",
				Type:      pipeline.NodeTypeReviewer,
				Backend:   cfg.AuditorBackendID,
				Model:     cfg.AuditorModel,
				DependsOn: []string{"generator"},
				Config: pipeline.NodeConfig{
					PromptTemplate: cfg.AuditPrompt,
				},
				Timeout: cfg.AuditTimeoutSec,
				Retry: &pipeline.RetryConfig{
					MaxAttempts:     cfg.MaxRetries,
					BackoffStrategy: "exponential",
				},
			},
		},
		GlobalConfig: pipeline.GlobalPipelineConfig{
			Timeout:       cfg.AuditTimeoutSec * 2,
			MaxRetries:    cfg.MaxRetries,
			ParallelLimit: 1,
		},
	}
}

// BuildOptimizePipeline 从优化配置构建流水线
func (sa *SchedulerAdapter) BuildOptimizePipeline() *pipeline.AgentPatternPipeline {
	cfg := sa.optimizeConfig

	return &pipeline.AgentPatternPipeline{
		ID:          "optimize-mode-pipeline",
		Name:        "Optimize Mode Pipeline",
		Description: "自动从优化配置生成的流水线",
		Version:     "1.0",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:      "generator",
				Name:    "Generator",
				Type:    pipeline.NodeTypeGenerator,
				Backend: cfg.ExecutorBackendID,
				Model:   cfg.ExecutorModel,
			},
			{
				ID:        "optimizer",
				Name:      "Optimizer",
				Type:      pipeline.NodeTypeProcessor,
				Backend:   cfg.OptimizerBackend,
				Model:     cfg.OptimizerModel,
				DependsOn: []string{"generator"},
				Config: pipeline.NodeConfig{
					PromptTemplate: cfg.OptimizePrompt,
					CustomConfig:   map[string]interface{}{"operation": "optimize"},
				},
				Timeout: cfg.OptimizeTimeoutSec,
				Retry: &pipeline.RetryConfig{
					MaxAttempts:     cfg.MaxRetries,
					BackoffStrategy: "exponential",
				},
			},
		},
		GlobalConfig: pipeline.GlobalPipelineConfig{
			Timeout:       cfg.OptimizeTimeoutSec * 2,
			MaxRetries:    cfg.MaxRetries,
			ParallelLimit: 1,
		},
	}
}

// ExecuteAudit 使用流水线执行审核
func (sa *SchedulerAdapter) ExecuteAudit(
	ctx context.Context,
	question string,
	executorResult *pipeline.ExecutorResult,
) (*AuditDecision, error) {
	// 构建输入
	input := &pipeline.PipelineInput{
		Content: question,
		Metadata: map[string]interface{}{
			"question":         question,
			"executor_answer": executorResult.Content,
			"executor_model":  executorResult.Model,
		},
	}

	// 检查是否有自定义流水线
	if p, exists := sa.pipelineScheduler.GetPipelineForMode("#a"); exists {
		output, err := sa.pipelineScheduler.Engine().Execute(ctx, p.ID, input)
		if err != nil {
			return nil, fmt.Errorf("pipeline execution failed: %w", err)
		}
		return sa.convertAuditOutput(output, executorResult)
	}

	// 使用默认流水线
	p := sa.BuildAuditPipeline()
	if err := sa.pipelineScheduler.Registry().Register(p); err != nil {
		return nil, fmt.Errorf("failed to register audit pipeline: %w", err)
	}
	output, err := sa.pipelineScheduler.Engine().Execute(ctx, p.ID, input)
	if err != nil {
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	return sa.convertAuditOutput(output, executorResult)
}

// ExecuteOptimize 使用流水线执行优化
func (sa *SchedulerAdapter) ExecuteOptimize(
	ctx context.Context,
	question string,
	executorResult *pipeline.ExecutorResult,
) (*OptimizeDecision, error) {
	// 构建输入
	input := &pipeline.PipelineInput{
		Content: executorResult.Content,
		Metadata: map[string]interface{}{
			"question":        question,
			"original":        executorResult.Content,
			"executor_model": executorResult.Model,
		},
	}

	// 检查是否有自定义流水线
	if p, exists := sa.pipelineScheduler.GetPipelineForMode("#o"); exists {
		output, err := sa.pipelineScheduler.Engine().Execute(ctx, p.ID, input)
		if err != nil {
			return nil, fmt.Errorf("pipeline execution failed: %w", err)
		}
		return sa.convertOptimizeOutput(output, executorResult)
	}

	// 使用默认流水线
	p := sa.BuildOptimizePipeline()
	if err := sa.pipelineScheduler.Registry().Register(p); err != nil {
		return nil, fmt.Errorf("failed to register optimize pipeline: %w", err)
	}
	output, err := sa.pipelineScheduler.Engine().Execute(ctx, p.ID, input)
	if err != nil {
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	return sa.convertOptimizeOutput(output, executorResult)
}

// convertAuditOutput 将流水线输出转换为审核决策
func (sa *SchedulerAdapter) convertAuditOutput(
	output *pipeline.PipelineOutput,
	executorResult *pipeline.ExecutorResult,
) (*AuditDecision, error) {
	lastNode := output.LastNode
	if lastNode == "" {
		lastNode = "auditor"
	}

	nodeOutput, exists := output.NodeOutputs[lastNode]
	if !exists {
		return nil, fmt.Errorf("no output from auditor node")
	}

	passed := true
	if nodeOutput.Passed != nil {
		passed = *nodeOutput.Passed
	}

	score := 0.9
	if nodeOutput.Score != nil {
		score = *nodeOutput.Score
	}

	feedback := nodeOutput.Feedback
	if feedback == "" {
		feedback = nodeOutput.Content
	}

	action := "pass"
	if !passed {
		action = "reject"
	}

	return &AuditDecision{
		ExecutorBackendID: executorResult.BackendID,
		ExecutorModel:     executorResult.Model,
		AuditorBackendID:  sa.auditConfig.AuditorBackendID,
		AuditorModel:      sa.auditConfig.AuditorModel,
		OriginalAnswer:    executorResult.Content,
		AuditResult: &AuditResult{
			Passed:      passed,
			Score:       score,
			Feedback:    feedback,
			Suggestions: []string{},
			RawResponse: nodeOutput.Content,
		},
		FinalAnswer: executorResult.Content,
		Action:      action,
		Reason:      feedback,
	}, nil
}

// convertOptimizeOutput 将流水线输出转换为优化决策
func (sa *SchedulerAdapter) convertOptimizeOutput(
	output *pipeline.PipelineOutput,
	executorResult *pipeline.ExecutorResult,
) (*OptimizeDecision, error) {
	lastNode := output.LastNode
	if lastNode == "" {
		lastNode = "optimizer"
	}

	nodeOutput, exists := output.NodeOutputs[lastNode]
	if !exists {
		return nil, fmt.Errorf("no output from optimizer node")
	}

	improvements := []string{}
	if improvementsRaw, ok := nodeOutput.Metadata["improvements"]; ok {
		if arr, ok := improvementsRaw.([]string); ok {
			improvements = arr
		}
	}

	return &OptimizeDecision{
		ExecutorBackendID: executorResult.BackendID,
		ExecutorModel:     executorResult.Model,
		OptimizerBackend:  sa.optimizeConfig.OptimizerBackend,
		OptimizerModel:    sa.optimizeConfig.OptimizerModel,
		OriginalAnswer:    executorResult.Content,
		OptimizeResult: &OptimizeResult{
			Optimized:    true,
			Original:     executorResult.Content,
			OptimizedText:   nodeOutput.Content,
			Improvements: improvements,
			RawResponse:  nodeOutput.Content,
		},
		FinalAnswer: nodeOutput.Content,
		Action:      "optimized",
		Reason:      "optimization completed",
	}, nil
}

// IsPipelineMode 检查是否使用流水线模式
func (sa *SchedulerAdapter) IsPipelineMode(modeKey string) bool {
	_, exists := sa.pipelineScheduler.GetPipelineForMode(modeKey)
	return exists
}
