package tokenusage

import "testing"

func TestEstimateCost(t *testing.T) {
	cost := EstimateCost("openai", "gpt-4", 1000, 500)
	if cost <= 0 {
		t.Fatalf("expected positive cost, got %v", cost)
	}
	if EstimateCost("", "", 0, 0) != 0 {
		t.Fatal("zero tokens should yield zero cost")
	}
}