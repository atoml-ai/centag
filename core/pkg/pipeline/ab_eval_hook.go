package pipeline

import "context"

// ABEvalPersistRequest is passed to the optional A/B evaluation persistence hook.
type ABEvalPersistRequest struct {
	PipelineID     string
	RequestID      string
	Question       string
	Strategy       string
	WinnerNode     string
	CandidateANode string
	CandidateBNode string
	ModelA         string
	ModelB         string
	ScoreA         float64
	ScoreB         float64
	LatencyAMs     int64
	LatencyBMs     int64
	CostAUSD       float64
	CostBUSD       float64
}

// PersistABEval optionally records aggregator A/B comparison outcomes.
var PersistABEval func(ctx context.Context, req ABEvalPersistRequest)

func persistABEvalFromScore(ctx context.Context, input *NodeInput, req ABEvalPersistRequest) {
	if PersistABEval == nil || len(req.CandidateANode) == 0 || len(req.CandidateBNode) == 0 {
		return
	}
	if input != nil && input.Metadata != nil {
		if req.PipelineID == "" {
			if v, ok := input.Metadata["pipeline_id"].(string); ok {
				req.PipelineID = v
			}
		}
		if req.RequestID == "" {
			if v, ok := input.Metadata["request_id"].(string); ok {
				req.RequestID = v
			}
		}
	}
	go func() { PersistABEval(context.Background(), req) }()
}