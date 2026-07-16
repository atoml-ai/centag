package cache

import (
	"context"

	evalmanager "centag/core/internal/cache/evaluation/manager"
	"centag/core/internal/cache/evaluation/plugin"
)

// EvaluationManagerWrapper 包装evaluation.Manager以适配cache包的使用
type EvaluationManagerWrapper struct {
	evalMgr *evalmanager.Manager
}

func NewEvaluationManagerWrapper(evalMgr *evalmanager.Manager) *EvaluationManagerWrapper {
	return &EvaluationManagerWrapper{
		evalMgr: evalMgr,
	}
}

func (w *EvaluationManagerWrapper) Execute(ctx context.Context, input *plugin.EvalInput) (*plugin.EvalOutput, error) {
	result, err := w.evalMgr.ExecutePipeline(ctx, input)
	if err != nil {
		return nil, err
	}
	return result.FinalOutput, nil
}

func (w *EvaluationManagerWrapper) IsEnabled() bool {
	return w.evalMgr.HasEnabledPlugins()
}
