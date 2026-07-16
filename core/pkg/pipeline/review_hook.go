package pipeline

import "context"

// ContentReviewRequest is passed to the optional reviewer hook.
type ContentReviewRequest struct {
	Question string
	Answer   string
	Backend  string
	Model    string
}

// ContentReviewResult carries reviewer output for aggregator score strategy.
type ContentReviewResult struct {
	Score    float64
	Passed   bool
	Feedback string
}

// ReviewContent optionally scores an answer via business.reviewer.
var ReviewContent func(ctx context.Context, req ContentReviewRequest) (*ContentReviewResult, error)