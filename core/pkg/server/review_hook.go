package server

import (
	"context"

	"centag/core/pkg/pipeline"
)

// wireReviewContent 设置内容审核钩子。
//
// TODO: 审核功能应通过 business plugin registry 注册，
// 而不是直接引用 reviewer 插件。当前暂用 stub 实现。
func wireReviewContent(broker pipeline.CapabilityBroker) {
	if broker == nil {
		return
	}
	pipeline.ReviewContent = func(ctx context.Context, req pipeline.ContentReviewRequest) (*pipeline.ContentReviewResult, error) {
		// Stub: 默认通过审核，评分 0.5
		// 完整实现应在 dist/main.go 中通过 reviewer 插件注册
		return &pipeline.ContentReviewResult{
			Score:  0.5,
			Passed: true,
		}, nil
	}
}
