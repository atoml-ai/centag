package backend

import "testing"

func TestPreferredDefaultModel(t *testing.T) {
	if PreferredDefaultModel(nil) != "" {
		t.Fatal("nil config")
	}
	if PreferredDefaultModel(&BackendConfig{}) != "" {
		t.Fatal("empty config")
	}
	if got := PreferredDefaultModel(&BackendConfig{ProbeModel: " probe "}); got != "probe" {
		t.Fatalf("probe=%q", got)
	}
	got := PreferredDefaultModel(&BackendConfig{
		SupportedModels: []ModelMapping{
			{RequestedModel: "req", ActualModel: "act"},
		},
	})
	if got != "act" {
		t.Fatalf("supported actual=%q", got)
	}
	got = PreferredDefaultModel(&BackendConfig{
		SupportedModels: []ModelMapping{{RequestedModel: "req-only"}},
	})
	if got != "req-only" {
		t.Fatalf("supported requested=%q", got)
	}
	// ProbeModel wins over supported list
	got = PreferredDefaultModel(&BackendConfig{
		ProbeModel: "probe",
		SupportedModels: []ModelMapping{
			{ActualModel: "act"},
		},
	})
	if got != "probe" {
		t.Fatalf("probe should win, got %q", got)
	}
}
