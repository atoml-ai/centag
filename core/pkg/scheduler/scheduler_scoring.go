package scheduler

import (
	"strings"

	"centag/core/pkg/backend"
)

func shouldUseScoring(strategy string) bool {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "legacy", "keyword", "heuristic":
		return false
	default:
		return true
	}
}

func weightsForStrategy(strategy string, intent *ClassificationResult, scorer *MultiDimensionScorer) DimensionWeights {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "cost", "price":
		return CostOptimizedWeights()
	case "quality":
		return QualityOptimizedWeights()
	case "latency", "speed":
		return LatencyOptimizedWeights()
	case "balance", "":
		if scorer != nil {
			return scorer.GetWeightsForIntent(intent)
		}
		return DefaultWeights()
	default:
		if scorer != nil {
			return scorer.GetWeightsForIntent(intent)
		}
		return DefaultWeights()
	}
}

func (s *Scheduler) recommendByScoring(intent *ClassificationResult, requestedModel string, strategy string) *ScheduleDecision {
	decision := &ScheduleDecision{Alternatives: make([]BackendAlternative, 0)}
	backends := s.getEnabledBackends()
	if len(backends) == 0 {
		decision.Reason = "无可用后端"
		return decision
	}

	weights := weightsForStrategy(strategy, intent, s.scorer)
	const defaultInputTokens = 512
	const defaultOutputTokens = 256

	var scores []*BackendScore
	for _, b := range backends {
		if b == nil || !b.Enabled {
			continue
		}
		scores = append(scores, s.scorer.Score(&ScoreRequest{
			Backend:      b,
			Model:        requestedModel,
			Intent:       intent,
			InputTokens:  defaultInputTokens,
			OutputTokens: defaultOutputTokens,
			Weights:      weights,
		}))
	}
	if len(scores) == 0 {
		decision.Reason = "评分无可用后端"
		return decision
	}
	sortScores(scores)

	best := scores[0]
	backendCfg := findBackendByID(backends, best.BackendID)
	if backendCfg == nil {
		decision.Reason = "评分后端不可用"
		return decision
	}

	decision.RecommendedBackendID = best.BackendID
	decision.RecommendedModel = s.findBestModel(backendCfg, requestedModel, intent.TaskType)
	decision.Reason = best.Reason
	decision.EstimatedCost = best.EstimatedCost
	if s.scorer != nil && s.scorer.latencyMonitor != nil {
		if avgMs, _, ok := s.scorer.latencyMonitor.GetLatency(best.BackendID); ok {
			decision.EstimatedLatencyMs = avgMs
		}
	}

	for i, sc := range scores {
		if i == 0 {
			continue
		}
		if i > 3 {
			break
		}
		decision.Alternatives = append(decision.Alternatives, BackendAlternative{
			BackendID: sc.BackendID,
			BackendName: sc.BackendName,
			Model:       requestedModel,
			Score:       sc.TotalScore,
			Reason:      sc.Reason,
		})
	}
	return decision
}

func findBackendByID(backends []*backend.BackendConfig, id string) *backend.BackendConfig {
	for _, b := range backends {
		if b != nil && b.ID == id {
			return b
		}
	}
	return nil
}

// SetScorerDefaultWeights applies admin-configured dimension weights (0-100 ints).
func (s *Scheduler) SetScorerDefaultWeights(weights map[string]int) {
	if s == nil || s.scorer == nil || len(weights) == 0 {
		return
	}
	w := DefaultWeights()
	if v, ok := weights["price"]; ok && v > 0 {
		w.Price = float64(v) / 100.0
	}
	if v, ok := weights["performance"]; ok && v > 0 {
		w.Performance = float64(v) / 100.0
	}
	if v, ok := weights["quality"]; ok && v > 0 {
		w.Quality = float64(v) / 100.0
	}
	if v, ok := weights["latency"]; ok && v > 0 {
		w.Latency = float64(v) / 100.0
	}
	if v, ok := weights["privacy"]; ok && v > 0 {
		w.Privacy = float64(v) / 100.0
	}
	if v, ok := weights["match"]; ok && v > 0 {
		w.Match = float64(v) / 100.0
	}
	s.scorer.SetDefaultWeights(w)
}