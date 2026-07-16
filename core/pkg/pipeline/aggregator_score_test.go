package pipeline

import (
	"context"
	"testing"
)

func TestAggregatorNode_ScoreStrategy(t *testing.T) {
	ReviewContent = func(ctx context.Context, req ContentReviewRequest) (*ContentReviewResult, error) {
		score := 0.6
		if req.Answer == "answer-b" {
			score = 0.9
		}
		return &ContentReviewResult{Score: score, Passed: true}, nil
	}
	defer func() { ReviewContent = nil }()

	node, err := NewAggregatorNode(NodeConfig{
		Backend: "test-backend",
		Model:   "gpt-4",
		CustomConfig: map[string]interface{}{
			"strategy": "score",
		},
	})
	if err != nil {
		t.Fatalf("NewAggregatorNode: %v", err)
	}
	agg := node.(*AggregatorNode)
	agg.BaseNode.id = "aggregator"

	output, err := agg.Execute(context.Background(), &NodeInput{
		Content: "question",
		Metadata: map[string]interface{}{
			"question": "question",
			"gen-a": map[string]interface{}{
				"content": "answer-a",
				"metadata": map[string]interface{}{
					"model": "model-a",
				},
			},
			"gen-b": map[string]interface{}{
				"content": "answer-b",
				"metadata": map[string]interface{}{
					"model": "model-b",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if output.Content != "answer-b" {
		t.Fatalf("content = %q, want answer-b", output.Content)
	}
	if output.Metadata["selected_node"] != "gen-b" {
		t.Fatalf("selected_node = %v", output.Metadata["selected_node"])
	}
}

func TestAggregatorNode_ScoreStrategyValidation(t *testing.T) {
	node, _ := NewAggregatorNode(NodeConfig{
		CustomConfig: map[string]interface{}{"strategy": "score"},
	})
	agg := node.(*AggregatorNode)
	if err := agg.Validate(); err == nil {
		t.Fatal("expected validation error without backend/model")
	}
}