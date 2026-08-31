package tokenusage

import "testing"

func TestEstimateCost(t *testing.T) {
	// Without pricing service configured, cost should be 0
	cost := EstimateCost("openai", "gpt-4", 1000, 500)
	if cost != 0 {
		t.Fatalf("expected zero cost without pricing service, got %v", cost)
	}
	if EstimateCost("", "", 0, 0) != 0 {
		t.Fatal("zero tokens should yield zero cost")
	}
}
