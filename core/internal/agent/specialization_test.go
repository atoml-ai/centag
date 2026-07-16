package agent

import (
	"sort"
	"testing"
)

// ============================================================================
// Specialization Registry Tests (Task 2.7)
// ============================================================================

func TestSpecializedAgentRegistry_SeedDefaults(t *testing.T) {
	r := NewSpecializedAgentRegistry()
	r.SeedDefaults()

	if r.Count() != 4 {
		t.Errorf("expected 4 specializations, got %d", r.Count())
	}

	// 验证 TUI Agent 数量
	allTUI := r.GetAllTUIAgents()
	if len(allTUI) != 2 {
		t.Errorf("expected 2 TUI agents, got %d", len(allTUI))
	}

	// 验证 Web Agent 数量
	allWeb := r.GetAllWebAgents()
	if len(allWeb) != 2 {
		t.Errorf("expected 2 Web agents, got %d", len(allWeb))
	}
}

func TestSpecializedAgentRegistry_ListByCategory(t *testing.T) {
	r := NewSpecializedAgentRegistry()
	r.SeedDefaults()

	tuiAgents := r.ListByCategory(AgentCategoryTUI)
	if len(tuiAgents) != 2 {
		t.Errorf("expected 2 TUI specializations, got %d", len(tuiAgents))
	}
	for _, s := range tuiAgents {
		if s.Category != AgentCategoryTUI {
			t.Errorf("expected category tui, got %s", s.Category)
		}
	}

	webAgents := r.ListByCategory(AgentCategoryWeb)
	if len(webAgents) != 2 {
		t.Errorf("expected 2 Web specializations, got %d", len(webAgents))
	}

	cliAgents := r.ListByCategory(AgentCategoryCLI)
	if len(cliAgents) != 0 {
		t.Errorf("expected 0 CLI specializations, got %d", len(cliAgents))
	}
}

func TestSpecializedAgentRegistry_DiscoverCapabilities(t *testing.T) {
	r := NewSpecializedAgentRegistry()
	r.SeedDefaults()

	// 能力 "code_highlight" 应该只存在于 coding-tui
	agents := r.DiscoverCapabilities("code_highlight")
	if len(agents) != 1 {
		t.Errorf("expected 1 agent with code_highlight, got %d", len(agents))
	}
	if agents[0].Type != AgentCodingTUI {
		t.Errorf("expected coding-tui, got %s", agents[0].Type)
	}

	// 能力 "progress_tracking" 应该存在于 2 个 Agent
	progressAgents := r.DiscoverCapabilities("progress_tracking")
	if len(progressAgents) != 2 {
		t.Errorf("expected 2 agents with progress_tracking, got %d", len(progressAgents))
	}

	// 不存在的应有返回空
	noMatch := r.DiscoverCapabilities("nonexistent_capability")
	if len(noMatch) != 0 {
		t.Errorf("expected 0 agents, got %d", len(noMatch))
	}
}

func TestSpecializedAgentRegistry_Register(t *testing.T) {
	r := NewSpecializedAgentRegistry()

	spec := &AgentSpecialization{
		Type:     "test-agent",
		Category: AgentCategoryCLI,
		Capabilities: []string{"test_cap", "demo"},
		Metadata: map[string]string{"version": "1.0"},
	}
	r.Register(spec)

	got, ok := r.Get("test-agent")
	if !ok {
		t.Fatal("expected specialization to be registered")
	}
	if got.Type != "test-agent" {
		t.Errorf("expected type test-agent, got %s", got.Type)
	}
	if got.Category != AgentCategoryCLI {
		t.Errorf("expected category cli, got %s", got.Category)
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(got.Capabilities))
	}
}

func TestSpecializedAgentRegistry_GetNotFound(t *testing.T) {
	r := NewSpecializedAgentRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent specialization")
	}
}

func TestSpecializedAgentRegistry_RegisterTUIAgent(t *testing.T) {
	r := NewSpecializedAgentRegistry()

	agent := NewCodingTUIAgent()
	r.RegisterTUIAgent(AgentCodingTUI, agent)

	got, ok := r.GetTUIAgent(AgentCodingTUI)
	if !ok {
		t.Fatal("expected TUI agent to be registered")
	}
	if got.AgentType() != AgentCodingTUI {
		t.Errorf("expected coding-tui, got %s", got.AgentType())
	}
}

func TestSpecializedAgentRegistry_RegisterWebAgent(t *testing.T) {
	r := NewSpecializedAgentRegistry()

	agent := NewCodingWebAgent(nil)
	r.RegisterWebAgent(AgentCodingWeb, agent)

	got, ok := r.GetWebAgent(AgentCodingWeb)
	if !ok {
		t.Fatal("expected Web agent to be registered")
	}
	if got.AgentType() != AgentCodingWeb {
		t.Errorf("expected coding-web, got %s", got.AgentType())
	}
}

func TestSpecializedAgentRegistry_Count(t *testing.T) {
	r := NewSpecializedAgentRegistry()
	if r.Count() != 0 {
		t.Errorf("expected 0 specializations, got %d", r.Count())
	}

	r.SeedDefaults()
	if r.Count() != 4 {
		t.Errorf("expected 4 specializations, got %d", r.Count())
	}

	r.CountByCategory(AgentCategoryTUI)
	if r.Count() != 4 {
		t.Errorf("CountByCategory should not modify count")
	}
}

func TestSpecializedAgentRegistry_ListSorted(t *testing.T) {
	r := NewSpecializedAgentRegistry()
	r.SeedDefaults()

	list := r.List()
	if len(list) != 4 {
		t.Errorf("expected 4 specializations, got %d", len(list))
	}
	if !sort.SliceIsSorted(list, func(i, j int) bool {
		return list[i].Type < list[j].Type
	}) {
		t.Error("List should return sorted results")
	}
}
