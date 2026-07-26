package agent

import "testing"

func TestNormalize_DeriveFromLegacy(t *testing.T) {
	m := AgentSetupMeta{
		Category:  AgentCategoryCLI,
		WriteMode: WriteModeMerge,
		CompanionCLI: &CompanionCLI{
			Binary: "hermes",
		},
	}.Normalize()
	if !m.HasAccess(AccessWriteConfig) || !m.HasAccess(AccessWrapCLI) {
		t.Fatalf("expected write+wrap, got %#v", m.AccessMethods)
	}
	if len(m.CompanionCLI.Argv) != 1 || m.CompanionCLI.Argv[0] != "hermes" {
		t.Fatalf("argv not filled: %#v", m.CompanionCLI)
	}
}

func TestNormalize_BuiltinTUI(t *testing.T) {
	m := AgentSetupMeta{
		Category:  AgentCategoryTUI,
		WriteMode: WriteModeNone,
	}.Normalize()
	if !m.HasAccess(AccessBuiltin) || m.HasAccess(AccessWriteConfig) {
		t.Fatalf("got %#v", m.AccessMethods)
	}
}

func TestResolveRequestURL(t *testing.T) {
	if got := ResolveRequestURL(RequestURLOpenAIBase, "localhost", 20060); got != "http://localhost:20060/v1" {
		t.Fatalf("openai_base: %s", got)
	}
	if got := ResolveRequestURL(RequestURLChatCompletions, "localhost", 20060); got != "http://localhost:20060/v1/chat/completions" {
		t.Fatalf("chat_completions: %s", got)
	}
	if got := ResolveRequestURL("", "localhost", 20060); got != "http://localhost:20060/v1" {
		t.Fatalf("default: %s", got)
	}
}

func TestGuideOnly(t *testing.T) {
	m := AgentSetupMeta{
		WriteMode:     WriteModeNone,
		AccessMethods: []AccessMethod{AccessUIGuide},
		UIGuide:       &UIGuide{Title: "x"},
	}
	if !m.GuideOnly() {
		t.Fatal("expected guide only")
	}
}

func TestWrapLaunchTargets(t *testing.T) {
	r := NewTemplateRegistry()
	targets := WrapLaunchTargets(r)
	if len(targets) == 0 {
		t.Fatal("expected wrap targets from registry")
	}
	byID := map[string]WrapLaunchInfo{}
	for _, x := range targets {
		byID[x.ID] = x
	}
	if _, ok := byID["claude-desktop"]; ok {
		t.Fatal("desktop without companion must not be wrap target")
	}
	if _, ok := byID["trae"]; ok {
		t.Fatal("trae (ui_guide only) must not be wrap target")
	}
	cb, ok := byID["codebuddy"]
	if !ok || len(cb.Argv) == 0 || cb.Argv[0] != "codebuddy" {
		t.Fatalf("codebuddy wrap: %#v ok=%v", cb, ok)
	}
	if _, ok := byID["opencode"]; !ok {
		t.Fatal("opencode should wrap")
	}
}
