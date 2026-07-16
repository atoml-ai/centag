package server

import (
	"context"

	"centag/core/internal/abeval"
	"centag/core/pkg/pipeline"
)

func wireABEvalPersistence(svc *abeval.Service) {
	if svc == nil {
		return
	}
	pipeline.PersistABEval = func(ctx context.Context, req pipeline.ABEvalPersistRequest) {
		_ = svc.RecordResult(ctx, &abeval.Record{
			PipelineID:     req.PipelineID,
			RequestID:      req.RequestID,
			Question:       req.Question,
			Strategy:       req.Strategy,
			WinnerNode:     req.WinnerNode,
			CandidateANode: req.CandidateANode,
			CandidateBNode: req.CandidateBNode,
			ModelA:         req.ModelA,
			ModelB:         req.ModelB,
			ScoreA:         req.ScoreA,
			ScoreB:         req.ScoreB,
			LatencyAMs:     req.LatencyAMs,
			LatencyBMs:     req.LatencyBMs,
			CostAUSD:       req.CostAUSD,
			CostBUSD:       req.CostBUSD,
		})
	}
}