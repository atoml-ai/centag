package scheduler

import (
	"testing"

	"centag/core/pkg/backend"
)

func TestScoreWeights_ByPolicy(t *testing.T) {
	tests := []struct {
		policy RoutingPolicyType
		want   ScoreWeights
	}{
		{RoutingPolicyCostOptimal, CostOptimalWeights()},
		{RoutingPolicyQualityFirst, QualityFirstWeights()},
		{RoutingPolicyLatencyFirst, LatencyFirstWeights()},
		{RoutingPolicyBalanced, DefaultScoreWeights()},
		{"unknown", DefaultScoreWeights()},
	}

	for _, tt := range tests {
		t.Run(string(tt.policy), func(t *testing.T) {
			got := GetScoreWeightsByPolicy(tt.policy)
			if got != tt.want {
				t.Errorf("GetScoreWeightsByPolicy(%s) = %+v, want %+v", tt.policy, got, tt.want)
			}
		})
	}
}

func TestScoreCandidate_DynamicCost(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	intent := &ClassificationResult{TaskType: TaskSimpleChat}
	weights := DefaultWeights()

	tests := []struct {
		name        string
		costPer1k   float64
		wantPrice   float64
	}{
		{"very low", 0.005, 0.9},
		{"low", 0.02, 0.8},
		{"medium", 0.07, 0.6},
		{"high", 0.2, 0.4},
		{"very high", 1.0, 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := &BackendCandidate{
				BackendID:        "test-backend",
				Model:            "test-model",
				DynamicCostPer1k: tt.costPer1k,
				Enabled:          true,
			}
			score := scorer.ScoreCandidate(candidate, intent, weights)
			if score.Dimensions.PriceScore != tt.wantPrice {
				t.Errorf("PriceScore = %v, want %v", score.Dimensions.PriceScore, tt.wantPrice)
			}
			if score.EstimatedCost != tt.costPer1k {
				t.Errorf("EstimatedCost = %v, want %v", score.EstimatedCost, tt.costPer1k)
			}
		})
	}
}

func TestScoreCandidate_ZeroCost(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	intent := &ClassificationResult{TaskType: TaskSimpleChat}
	weights := DefaultWeights()

	// Zero cost goes to else branch (static price table)
	candidate := &BackendCandidate{
		BackendID:        "test-backend",
		Model:            "test-model",
		DynamicCostPer1k: 0,
		Enabled:          true,
	}
	score := scorer.ScoreCandidate(candidate, intent, weights)
	// Should use static price table (default 0.5)
	if score.Dimensions.PriceScore != 0.5 {
		t.Errorf("PriceScore = %v, want 0.5", score.Dimensions.PriceScore)
	}
}

func TestScoreCandidates_FilterDisabled(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	intent := &ClassificationResult{TaskType: TaskSimpleChat}
	weights := DefaultWeights()

	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.01, Enabled: true},
		{BackendID: "b", Model: "m2", DynamicCostPer1k: 0.02, Enabled: false},
		{BackendID: "c", Model: "m3", DynamicCostPer1k: 0.03, Enabled: true},
	}

	scores := scorer.ScoreCandidates(candidates, intent, weights)
	if len(scores) != 2 {
		t.Fatalf("expected 2 scored candidates, got %d", len(scores))
	}
	// Should be sorted by score descending (lower cost = higher price score)
	if scores[0].BackendID != "a" {
		t.Errorf("expected first score to be 'a', got %s", scores[0].BackendID)
	}
}

func TestScoreCandidates_Empty(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	intent := &ClassificationResult{TaskType: TaskSimpleChat}
	weights := DefaultWeights()

	scores := scorer.ScoreCandidates([]*BackendCandidate{}, intent, weights)
	if len(scores) != 0 {
		t.Errorf("expected 0 scores, got %d", len(scores))
	}
}

func TestRoutingSelector_CostOptimal(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	selector := NewRoutingSelector(scorer)

	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.05, Enabled: true},
		{BackendID: "b", Model: "m2", DynamicCostPer1k: 0.01, Enabled: true},
		{BackendID: "c", Model: "m3", DynamicCostPer1k: 0.10, Enabled: true},
	}

	best := selector.SelectByRoutingPolicy(candidates, RoutingPolicyCostOptimal)
	if best == nil {
		t.Fatal("expected a candidate, got nil")
	}
	if best.BackendID != "b" {
		t.Errorf("expected cheapest candidate 'b', got %s", best.BackendID)
	}
}

func TestRoutingSelector_QualityFirst(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	selector := NewRoutingSelector(scorer)

	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.05, Enabled: true},
		{BackendID: "b", Model: "m2", DynamicCostPer1k: 0.01, Enabled: true},
	}

	best := selector.SelectByRoutingPolicy(candidates, RoutingPolicyQualityFirst)
	if best == nil {
		t.Fatal("expected a candidate, got nil")
	}
	// QualityFirst selects first candidate (simplified implementation)
	if best.BackendID != "a" {
		t.Errorf("expected first candidate 'a', got %s", best.BackendID)
	}
}

func TestRoutingSelector_FilterDisabled(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	selector := NewRoutingSelector(scorer)

	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.01, Enabled: false},
		{BackendID: "b", Model: "m2", DynamicCostPer1k: 0.05, Enabled: true},
	}

	best := selector.SelectByRoutingPolicy(candidates, RoutingPolicyCostOptimal)
	if best == nil {
		t.Fatal("expected a candidate, got nil")
	}
	if best.BackendID != "b" {
		t.Errorf("expected enabled candidate 'b', got %s", best.BackendID)
	}
}

func TestRoutingSelector_AllDisabled(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	selector := NewRoutingSelector(scorer)

	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.01, Enabled: false},
		{BackendID: "b", Model: "m2", DynamicCostPer1k: 0.05, Enabled: false},
	}

	best := selector.SelectByRoutingPolicy(candidates, RoutingPolicyCostOptimal)
	if best != nil {
		t.Errorf("expected nil for all disabled, got %+v", best)
	}
}

func TestRoutingSelector_EmptyCandidates(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	selector := NewRoutingSelector(scorer)

	best := selector.SelectByRoutingPolicy([]*BackendCandidate{}, RoutingPolicyCostOptimal)
	if best != nil {
		t.Errorf("expected nil for empty candidates, got %+v", best)
	}
}

func TestRoutingSelector_Balanced(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	selector := NewRoutingSelector(scorer)

	// Balanced should use multi-dimensional scoring
	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.01, Enabled: true},
		{BackendID: "b", Model: "m2", DynamicCostPer1k: 0.05, Enabled: true},
	}

	best := selector.SelectByRoutingPolicy(candidates, RoutingPolicyBalanced)
	if best == nil {
		t.Fatal("expected a candidate, got nil")
	}
	// Should return one of the candidates
	if best.BackendID != "a" && best.BackendID != "b" {
		t.Errorf("expected 'a' or 'b', got %s", best.BackendID)
	}
}

func TestRecommendByRoutingPolicy(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	s := &Scheduler{
		scorer: scorer,
	}

	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.05, Enabled: true},
		{BackendID: "b", Model: "m2", DynamicCostPer1k: 0.01, Enabled: true},
		{BackendID: "c", Model: "m3", DynamicCostPer1k: 0.10, Enabled: true},
	}

	decision := s.RecommendByRoutingPolicy(candidates, RoutingPolicyCostOptimal)
	if decision == nil {
		t.Fatal("expected a decision, got nil")
	}
	if decision.RecommendedBackendID != "b" {
		t.Errorf("expected cheapest 'b', got %s", decision.RecommendedBackendID)
	}
	if decision.EstimatedCost != 0.01 {
		t.Errorf("expected cost 0.01, got %f", decision.EstimatedCost)
	}
	// Should have alternatives
	if len(decision.Alternatives) != 2 {
		t.Errorf("expected 2 alternatives, got %d", len(decision.Alternatives))
	}
}

func TestRecommendByRoutingPolicy_Empty(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	s := &Scheduler{
		scorer: scorer,
	}

	decision := s.RecommendByRoutingPolicy([]*BackendCandidate{}, RoutingPolicyCostOptimal)
	if decision == nil {
		t.Fatal("expected a decision, got nil")
	}
	if decision.Reason != "无可用候选后端" {
		t.Errorf("expected reason '无可用候选后端', got %s", decision.Reason)
	}
}

func TestRecommendByRoutingPolicy_AllDisabled(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	s := &Scheduler{
		scorer: scorer,
	}

	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.01, Enabled: false},
	}

	decision := s.RecommendByRoutingPolicy(candidates, RoutingPolicyCostOptimal)
	if decision == nil {
		t.Fatal("expected a decision, got nil")
	}
	if decision.Reason != "路由选择失败" {
		t.Errorf("expected reason '路由选择失败', got %s", decision.Reason)
	}
}

func TestBackendCandidate_Construction(t *testing.T) {
	c := &BackendCandidate{
		BackendID:        "ppinfra",
		Model:            "deepseek-v3.2",
		DynamicCostPer1k: 0.05,
		PriceType:        "cost",
		Tier:             2,
		Enabled:          true,
	}

	if c.BackendID != "ppinfra" {
		t.Errorf("expected BackendID 'ppinfra', got %s", c.BackendID)
	}
	if c.Model != "deepseek-v3.2" {
		t.Errorf("expected Model 'deepseek-v3.2', got %s", c.Model)
	}
	if c.DynamicCostPer1k != 0.05 {
		t.Errorf("expected DynamicCostPer1k 0.05, got %f", c.DynamicCostPer1k)
	}
	if c.PriceType != "cost" {
		t.Errorf("expected PriceType 'cost', got %s", c.PriceType)
	}
}

func TestScoreWeights_Construction(t *testing.T) {
	w := ScoreWeights{
		Cost:    0.5,
		Quality: 0.3,
		Latency: 0.1,
		Match:   0.1,
	}

	if w.Cost != 0.5 {
		t.Errorf("expected Cost 0.5, got %f", w.Cost)
	}
	if w.Quality != 0.3 {
		t.Errorf("expected Quality 0.3, got %f", w.Quality)
	}
	if w.Latency != 0.1 {
		t.Errorf("expected Latency 0.1, got %f", w.Latency)
	}
	if w.Match != 0.1 {
		t.Errorf("expected Match 0.1, got %f", w.Match)
	}
}

func TestGetScoreWeightsByPolicy_AllPolicies(t *testing.T) {
	policies := []RoutingPolicyType{
		RoutingPolicyCostOptimal,
		RoutingPolicyQualityFirst,
		RoutingPolicyLatencyFirst,
		RoutingPolicyBalanced,
	}

	for _, policy := range policies {
		t.Run(string(policy), func(t *testing.T) {
			w := GetScoreWeightsByPolicy(policy)
			// Sum of weights should be close to 1.0
			sum := w.Cost + w.Quality + w.Latency + w.Match
			if sum < 0.99 || sum > 1.01 {
				t.Errorf("weights sum = %f, want ~1.0", sum)
			}
		})
	}
}

func TestRoutingSelector_CostOptimal_TieBreaker(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	selector := NewRoutingSelector(scorer)

	// Same cost - should return first one
	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.01, Enabled: true},
		{BackendID: "b", Model: "m2", DynamicCostPer1k: 0.01, Enabled: true},
	}

	best := selector.SelectByRoutingPolicy(candidates, RoutingPolicyCostOptimal)
	if best == nil {
		t.Fatal("expected a candidate, got nil")
	}
	if best.BackendID != "a" {
		t.Errorf("expected first candidate 'a' for tie, got %s", best.BackendID)
	}
}

func TestBackendCandidate_DisabledNotScored(t *testing.T) {
	scorer := NewMultiDimensionScorer()
	intent := &ClassificationResult{TaskType: TaskSimpleChat}
	weights := DefaultWeights()

	candidates := []*BackendCandidate{
		{BackendID: "a", Model: "m1", DynamicCostPer1k: 0.01, Enabled: false},
	}

	scores := scorer.ScoreCandidates(candidates, intent, weights)
	if len(scores) != 0 {
		t.Errorf("expected 0 scores for disabled candidates, got %d", len(scores))
	}
}

func testBackendConfig(id, name string, enabled bool) *backend.BackendConfig {
	return &backend.BackendConfig{
		ID:      id,
		Name:    name,
		Enabled: enabled,
	}
}
