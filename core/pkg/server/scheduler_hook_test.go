package server

import (
	"testing"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/pipeline"
)

func TestWireSchedulerBackend_SelectsBackend(t *testing.T) {
	mgr := backend.NewManager()
	mgr.Add(&backend.BackendConfig{
		ID:      "bigmodel",
		Name:    "BigModel",
		Enabled: true,
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "glm-4-flash", ActualModel: "glm-4-flash"},
		},
	})
	mgr.Add(&backend.BackendConfig{
		ID:      "ollama-local",
		Name:    "Ollama",
		Enabled: true,
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "qwen2.5", ActualModel: "qwen2.5"},
		},
	})

	cfg := &config.Config{}
	cfg.Scheduler.EnableIntentRecognition = false

	sched := buildScheduler(cfg, mgr)
	wireSchedulerBackend(sched)
	defer func() { pipeline.ScheduleBackend = nil }()

	if pipeline.ScheduleBackend == nil {
		t.Fatal("ScheduleBackend hook not wired")
	}

	result, err := pipeline.ScheduleBackend(pipeline.ScheduleRequest{
		Question: "你好",
		Strategy: "balance",
	})
	if err != nil {
		t.Fatalf("ScheduleBackend: %v", err)
	}
	if result.BackendID == "" {
		t.Fatal("expected non-empty backend_id")
	}
}