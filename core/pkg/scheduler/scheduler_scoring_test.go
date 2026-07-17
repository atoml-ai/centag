package scheduler

import (
	"testing"

	"centag/core/pkg/backend"
)

func TestScheduleWithStrategy_UsesScoringByDefault(t *testing.T) {
	mgr := backend.NewManager()
	mgr.Add(&backend.BackendConfig{
		ID:      "cheap",
		Name:    "Cheap",
		Enabled: true,
		Type:    "openai",
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "gpt-4o-mini", ActualModel: "gpt-4o-mini"},
		},
	})
	mgr.Add(&backend.BackendConfig{
		ID:      "premium",
		Name:    "Premium",
		Enabled: true,
		Type:    "openai",
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "gpt-4", ActualModel: "gpt-4"},
		},
	})

	cfg := DefaultSchedulerConfig()
	cfg.IntentClassifier.Enabled = false
	cfg.EnableLogging = false
	sched := NewScheduler(cfg, mgr)

	decision, err := sched.ScheduleWithStrategy("写一段 Python 排序算法", "gpt-4o-mini", "balance")
	if err != nil {
		t.Fatalf("ScheduleWithStrategy: %v", err)
	}
	if decision.RecommendedBackendID == "" {
		t.Fatal("expected scoring to select a backend")
	}
	if decision.Reason == "" {
		t.Fatal("expected scoring reason")
	}
}

func TestFindBestModel_PrefersProbeModelWhenNoRequest(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.IntentClassifier.Enabled = false
	cfg.EnableLogging = false
	sched := NewScheduler(cfg, backend.NewManager())

	b := &backend.BackendConfig{
		ID:         "openai-bigmodel-ai",
		Type:       "openai",
		ProbeModel: "glm-4-flash",
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "glm-4-plus", ActualModel: "glm-4-plus"},
			{RequestedModel: "glm-4-flash", ActualModel: "glm-4-flash"},
		},
	}
	got := sched.findBestModel(b, "", TaskSimpleChat)
	if got != "glm-4-flash" {
		t.Fatalf("got %q, want glm-4-flash from probe_model (not first supported model)", got)
	}
}

func TestScheduleWithStrategy_LegacyUsesSelector(t *testing.T) {
	mgr := backend.NewManager()
	mgr.Add(&backend.BackendConfig{
		ID:      "ollama-local",
		Name:    "Ollama",
		Enabled: true,
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "qwen2.5", ActualModel: "qwen2.5"},
		},
	})

	cfg := DefaultSchedulerConfig()
	cfg.IntentClassifier.Enabled = false
	cfg.EnableLogging = false
	sched := NewScheduler(cfg, mgr)

	decision, err := sched.ScheduleWithStrategy("你好", "", "legacy")
	if err != nil {
		t.Fatalf("ScheduleWithStrategy legacy: %v", err)
	}
	if decision.RecommendedBackendID == "" {
		t.Fatal("expected legacy selector to pick a backend")
	}
}