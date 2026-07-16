package agent

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// ============================================================================
// TUIAgentTemplate Tests
// ============================================================================

func TestTUIAgentTemplate_RenderStatusBar(t *testing.T) {
	tpl := NewTUIAgentTemplate("test-tui", "Test", "desc")

	tests := []struct {
		name     string
		info     *StatusBarInfo
		contains []string
	}{
		{"nil info", nil, []string{"-- TUI Agent --"}},
		{"normal mode", &StatusBarInfo{Mode: "NORMAL", FilePath: "main.go", Position: 42, Encoding: "UTF-8"}, []string{"[NORMAL]", "main.go:42", "UTF-8"}},
		{"insert mode modified", &StatusBarInfo{Mode: "INSERT", FilePath: "app.ts", Position: 10, Modified: true, Encoding: "UTF-8"}, []string{"[INSERT]", "+", "app.ts:10"}},
		{"command mode", &StatusBarInfo{Mode: "COMMAND", FilePath: "", Position: 0, Encoding: "ASCII"}, []string{"[COMMAND]", ":0"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tpl.RenderStatusBar(tt.info)
			for _, c := range tt.contains {
				if !strings.Contains(result, c) {
					t.Errorf("result %q missing %q", result, c)
				}
			}
		})
	}
}

func TestTUIAgentTemplate_RenderProgress(t *testing.T) {
	tpl := NewTUIAgentTemplate("test-tui", "Test", "desc")

	tests := []struct {
		name     string
		info     *ProgressInfo
		empty    bool
		contains []string
	}{
		{"nil info", nil, true, nil},
		{"zero total", &ProgressInfo{Total: 0}, true, nil},
		{"half progress", &ProgressInfo{Current: 50, Total: 100}, false, []string{"50.0%", "50/100"}},
		{"complete", &ProgressInfo{Current: 10, Total: 10}, false, []string{"100.0%"}},
		{"custom message", &ProgressInfo{Current: 3, Total: 5, Message: "processing"}, false, []string{"60.0%", "processing"}},
		{"overflow clamped", &ProgressInfo{Current: 200, Total: 100}, false, []string{"200.0%"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tpl.RenderProgress(tt.info)
			if tt.empty && result != "" {
				t.Errorf("expected empty, got %q", result)
			}
			if !tt.empty {
				for _, c := range tt.contains {
					if !strings.Contains(result, c) {
						t.Errorf("result %q missing %q", result, c)
					}
				}
			}
		})
	}
}

func TestTUIAgentTemplate_RenderCodeHighlight(t *testing.T) {
	tpl := NewTUIAgentTemplate("test-tui", "Test", "desc")

	code := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"

	t.Run("no options", func(t *testing.T) {
		result := tpl.RenderCodeHighlight(code, nil)
		if result != code {
			t.Errorf("no options should return raw code, got %q", result)
		}
	})

	t.Run("with line numbers", func(t *testing.T) {
		opts := &CodeHighlightOptions{ShowLineNumbers: true}
		result := tpl.RenderCodeHighlight(code, opts)
		if !strings.Contains(result, "1 │ package main") {
			t.Errorf("should contain line numbers, got %q", result)
		}
		lines := splitLines(result)
		if len(lines) != 5 {
			t.Errorf("expected 5 lines, got %d", len(lines))
		}
	})

	t.Run("empty code", func(t *testing.T) {
		result := tpl.RenderCodeHighlight("", &CodeHighlightOptions{ShowLineNumbers: true})
		if result != "" {
			t.Errorf("empty code should return empty, got %q", result)
		}
	})

	t.Run("single line", func(t *testing.T) {
		result := tpl.RenderCodeHighlight("single line", &CodeHighlightOptions{ShowLineNumbers: true})
		if !strings.Contains(result, "1 │ single line") {
			t.Errorf("single line should have number, got %q", result)
		}
	})
}

func TestTUIAgentTemplate_RenderDiff(t *testing.T) {
	tpl := NewTUIAgentTemplate("test-tui", "Test", "desc")

	t.Run("same content", func(t *testing.T) {
		result := tpl.RenderDiff("line1\nline2\n", "line1\nline2\n", nil)
		// All lines unchanged
		if strings.Contains(result, "+ ") || strings.Contains(result, "- ") {
			t.Errorf("same content should not show diff, got:\n%s", result)
		}
	})

	t.Run("modified content", func(t *testing.T) {
		result := tpl.RenderDiff("old line\n", "new line\n", nil)
		if !strings.Contains(result, "- old line") {
			t.Errorf("should show removed line, got:\n%s", result)
		}
		if !strings.Contains(result, "+ new line") {
			t.Errorf("should show added line, got:\n%s", result)
		}
	})

	t.Run("added line", func(t *testing.T) {
		result := tpl.RenderDiff("line1\n", "line1\nline2\n", nil)
		if !strings.Contains(result, "+ line2") {
			t.Errorf("should show added line, got:\n%s", result)
		}
	})

	t.Run("removed line", func(t *testing.T) {
		result := tpl.RenderDiff("line1\nline2\n", "line1\n", nil)
		if !strings.Contains(result, "- line2") {
			t.Errorf("should show removed line, got:\n%s", result)
		}
	})

	t.Run("nil options", func(t *testing.T) {
		result := tpl.RenderDiff("a\nb\n", "a\nc\n", nil)
		if !strings.Contains(result, "+ c") {
			t.Errorf("nil options should work, got:\n%s", result)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		result := tpl.RenderDiff("", "new line\n", nil)
		if !strings.Contains(result, "+ new line") {
			t.Errorf("empty old should show added, got:\n%s", result)
		}
	})
}

func TestTUIAgentTemplate_PromptUserChoice(t *testing.T) {
	tpl := NewTUIAgentTemplate("test-tui", "Test", "desc")

	t.Run("empty choices", func(t *testing.T) {
		idx, err := tpl.PromptUserChoice(nil)
		if err == nil {
			t.Error("expected error for nil choices")
		}
		if idx != -1 {
			t.Errorf("expected -1, got %d", idx)
		}
	})

	t.Run("no selection", func(t *testing.T) {
		choices := []UserChoice{
			{Key: "a", Label: "Option A"},
			{Key: "b", Label: "Option B"},
		}
		idx, err := tpl.PromptUserChoice(choices)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx != 0 {
			t.Errorf("expected first item, got %d", idx)
		}
	})

	t.Run("with pre-selected", func(t *testing.T) {
		choices := []UserChoice{
			{Key: "a", Label: "Option A"},
			{Key: "b", Label: "Option B", Selected: true},
			{Key: "c", Label: "Option C"},
		}
		idx, err := tpl.PromptUserChoice(choices)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx != 1 {
			t.Errorf("expected selected item (1), got %d", idx)
		}
	})

	t.Run("multiple selected returns first", func(t *testing.T) {
		choices := []UserChoice{
			{Key: "a", Label: "Option A", Selected: true},
			{Key: "b", Label: "Option B", Selected: true},
		}
		idx, err := tpl.PromptUserChoice(choices)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx != 0 {
			t.Errorf("expected first selected (0), got %d", idx)
		}
	})
}

func TestTUIAgentTemplate_ShowNotification(t *testing.T) {
	tpl := NewTUIAgentTemplate("test-tui", "Test", "desc")

	t.Run("nil notification", func(t *testing.T) {
		result := tpl.ShowNotification(nil)
		if result != "" {
			t.Errorf("nil notification should return empty, got %q", result)
		}
	})

	tests := []struct {
		name     string
		level    NotificationLevel
		message  string
		contains string
	}{
		{"info", NotificationInfo, "Info message", "ℹ"},
		{"success", NotificationSuccess, "Success!", "✓"},
		{"warning", NotificationWarning, "Warning!", "⚠"},
		{"error", NotificationError, "Error!", "✗"},
		{"unknown level", NotificationLevel(99), "Unknown", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tpl.ShowNotification(&Notification{
				Level:   tt.level,
				Message: tt.message,
			})
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected %q in %q", tt.contains, result)
			}
			if !strings.Contains(result, tt.message) {
				t.Errorf("expected message %q in %q", tt.message, result)
			}
		})
	}
}

func TestTUIAgentTemplate_AgentTemplateMethods(t *testing.T) {
	tpl := NewTUIAgentTemplate(AgentCodingTUI, "Coding TUI", "A coding TUI agent")

	if tpl.AgentType() != AgentCodingTUI {
		t.Errorf("AgentType = %q, want %q", tpl.AgentType(), AgentCodingTUI)
	}
	if tpl.DisplayName() != "Coding TUI" {
		t.Errorf("DisplayName = %q", tpl.DisplayName())
	}
	if tpl.Description() != "A coding TUI agent" {
		t.Errorf("Description = %q", tpl.Description())
	}

	// Default implementations
	files, err := tpl.ConfigFiles(nil)
	if err != nil || len(files) != 0 {
		t.Errorf("ConfigFiles should return nil, nil")
	}
	if tpl.SetupCommand(nil) != "" {
		t.Error("SetupCommand should return empty")
	}
	if tpl.PlatformCommands(nil) != (PlatformCommands{}) {
		t.Error("PlatformCommands should return empty")
	}
	if tpl.VerifyCommand(nil) != "" {
		t.Error("VerifyCommand should return empty")
	}
	if steps := tpl.Steps(nil); len(steps) != 0 {
		t.Error("Steps should return nil")
	}
	if err := tpl.WriteConfig(nil); err != nil {
		t.Errorf("WriteConfig should return nil, got %v", err)
	}
}

// ============================================================================
// CodingTUIAgent Tests
// ============================================================================

func TestCodingTUIAgent_New(t *testing.T) {
	agent := NewCodingTUIAgent()
	if agent.AgentType() != AgentCodingTUI {
		t.Errorf("AgentType = %q, want %q", agent.AgentType(), AgentCodingTUI)
	}
}

func TestCodingTUIAgent_RenderStatusBar(t *testing.T) {
	agent := NewCodingTUIAgent()

	t.Run("nil info", func(t *testing.T) {
		if result := agent.RenderStatusBar(nil); result != "-- Coding TUI --" {
			t.Errorf("nil info = %q", result)
		}
	})

	t.Run("with language", func(t *testing.T) {
		info := &StatusBarInfo{
			Mode:     "NORMAL",
			FilePath: "main.go",
			Position: 100,
			Encoding: "UTF-8",
			Language: "go",
		}
		result := agent.RenderStatusBar(info)
		if !strings.Contains(result, "go") {
			t.Errorf("should contain language, got %q", result)
		}
	})

	t.Run("without language defaults to code", func(t *testing.T) {
		info := &StatusBarInfo{
			Mode:     "NORMAL",
			FilePath: "main.go",
			Position: 1,
			Encoding: "UTF-8",
		}
		result := agent.RenderStatusBar(info)
		if !strings.Contains(result, "code") {
			t.Errorf("should default to 'code', got %q", result)
		}
	})

	t.Run("modified file", func(t *testing.T) {
		info := &StatusBarInfo{
			Mode:     "INSERT",
			FilePath: "app.ts",
			Position: 42,
			Encoding: "UTF-8",
			Modified: true,
		}
		result := agent.RenderStatusBar(info)
		if !strings.Contains(result, "+") {
			t.Errorf("modified file should show '+', got %q", result)
		}
	})
}

func TestCodingTUIAgent_RenderCodeHighlight(t *testing.T) {
	agent := NewCodingTUIAgent()
	code := "fmt.Println(\"hello\")"

	t.Run("default options", func(t *testing.T) {
		result := agent.RenderCodeHighlight(code, nil)
		if !strings.Contains(result, "```auto") {
			t.Errorf("should show auto language, got:\n%s", result)
		}
		if !strings.Contains(result, "1 │ fmt.Println") {
			t.Errorf("should show line numbers, got:\n%s", result)
		}
	})

	t.Run("custom language", func(t *testing.T) {
		opts := &CodeHighlightOptions{Language: "go", ShowLineNumbers: false}
		result := agent.RenderCodeHighlight(code, opts)
		if !strings.Contains(result, "```go") {
			t.Errorf("should show go language, got:\n%s", result)
		}
	})

	t.Run("empty language uses auto", func(t *testing.T) {
		opts := &CodeHighlightOptions{Language: ""}
		result := agent.RenderCodeHighlight(code, opts)
		if !strings.Contains(result, "```auto") {
			t.Errorf("should default to auto, got:\n%s", result)
		}
	})
}

func TestCodingTUIAgent_RenderDiff(t *testing.T) {
	agent := NewCodingTUIAgent()

	t.Run("side by side default", func(t *testing.T) {
		result := agent.RenderDiff("old", "new", nil)
		// Side-by-side uses │ separator
		if !strings.Contains(result, "│") {
			t.Errorf("side-by-side should contain │, got:\n%s", result)
		}
		if !strings.Contains(result, "- old") {
			t.Errorf("should show removed, got:\n%s", result)
		}
		if !strings.Contains(result, "+ new") {
			t.Errorf("should show added, got:\n%s", result)
		}
	})

	t.Run("unified mode", func(t *testing.T) {
		opts := &DiffViewOptions{SideBySide: false}
		result := agent.RenderDiff("old", "new", opts)
		if strings.Contains(result, "│") {
			t.Errorf("unified should not contain │, got:\n%s", result)
		}
	})

	t.Run("same content side by side", func(t *testing.T) {
		result := agent.RenderDiff("same\n", "same\n", nil)
		if strings.Contains(result, "+ ") || strings.Contains(result, "- ") {
			t.Errorf("same content should not show diff, got:\n%s", result)
		}
	})

	t.Run("added line side by side", func(t *testing.T) {
		result := agent.RenderDiff("line1\n", "line1\nline2\n", nil)
		if !strings.Contains(result, "+ line2") {
			t.Errorf("should show added line, got:\n%s", result)
		}
	})
}

// ============================================================================
// EducationTUIAgent Tests
// ============================================================================

func TestEducationTUIAgent_New(t *testing.T) {
	agent := NewEducationTUIAgent()
	if agent.AgentType() != AgentEducationTUI {
		t.Errorf("AgentType = %q, want %q", agent.AgentType(), AgentEducationTUI)
	}
}

func TestEducationTUIAgent_RenderStatusBar(t *testing.T) {
	agent := NewEducationTUIAgent()

	t.Run("nil info", func(t *testing.T) {
		if result := agent.RenderStatusBar(nil); result != "-- Education TUI --" {
			t.Errorf("nil info = %q", result)
		}
	})

	tests := []struct {
		mode     string
		expected string
	}{
		{"NORMAL", "学习"},
		{"INSERT", "练习"},
		{"COMMAND", "操作"},
		{"UNKNOWN", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			info := &StatusBarInfo{
				Mode:     tt.mode,
				FilePath: "lesson1.md",
				Position: 15,
				Encoding: "UTF-8",
			}
			result := agent.RenderStatusBar(info)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("mode %q: expected %q in result %q", tt.mode, tt.expected, result)
			}
		})
	}
}

func TestEducationTUIAgent_RenderProgress(t *testing.T) {
	agent := NewEducationTUIAgent()

	t.Run("nil info", func(t *testing.T) {
		result := agent.RenderProgress(nil)
		if result != "" {
			t.Errorf("nil info should return empty, got %q", result)
		}
	})

	t.Run("zero total", func(t *testing.T) {
		result := agent.RenderProgress(&ProgressInfo{Total: 0})
		if result != "" {
			t.Errorf("zero total should return empty, got %q", result)
		}
	})

	t.Run("with progress", func(t *testing.T) {
		result := agent.RenderProgress(&ProgressInfo{
			Current: 3,
			Total:   10,
			Message: "学习阶段",
		})
		if !strings.Contains(result, "📖") {
			t.Errorf("should contain book emoji, got %q", result)
		}
		if !strings.Contains(result, "30%") {
			t.Errorf("should contain 30%%, got %q", result)
		}
		if !strings.Contains(result, "学习阶段") {
			t.Errorf("should contain message, got %q", result)
		}
	})

	t.Run("default message shows stages", func(t *testing.T) {
		result := agent.RenderProgress(&ProgressInfo{
			Current: 2,
			Total:   5,
		})
		if !strings.Contains(result, "第 2/5 阶段") {
			t.Errorf("should contain stage info, got %q", result)
		}
	})
}

func TestEducationTUIAgent_ShowNotification(t *testing.T) {
	agent := NewEducationTUIAgent()

	tests := []struct {
		level   NotificationLevel
		message string
		emoji   string
	}{
		{NotificationInfo, "课程开始", "📝"},
		{NotificationSuccess, "答题正确", "✅"},
		{NotificationWarning, "请注意", "💡"},
		{NotificationError, "答题错误", "❌"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			result := agent.ShowNotification(&Notification{
				Level:   tt.level,
				Message: tt.message,
			})
			if !strings.Contains(result, tt.emoji) {
				t.Errorf("expected emoji %q in %q", tt.emoji, result)
			}
			if !strings.Contains(result, tt.message) {
				t.Errorf("expected message %q in %q", tt.message, result)
			}
		})
	}

	t.Run("nil notification", func(t *testing.T) {
		if result := agent.ShowNotification(nil); result != "" {
			t.Errorf("nil should return empty, got %q", result)
		}
	})
}

// ============================================================================
// NotificationLevel Tests
// ============================================================================

func TestNotificationLevel_String(t *testing.T) {
	tests := []struct {
		level    NotificationLevel
		expected string
	}{
		{NotificationInfo, "info"},
		{NotificationSuccess, "success"},
		{NotificationWarning, "warning"},
		{NotificationError, "error"},
		{NotificationLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("NotificationLevel(%d).String() = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// ConfigTemplate Tests
// ============================================================================

func TestTUIConfigTemplate(t *testing.T) {
	tpl := newTUIConfigTemplate(AgentCodingTUI, "Coding TUI", "desc")

	if tpl.AgentType() != AgentCodingTUI {
		t.Errorf("AgentType = %q", tpl.AgentType())
	}
	if tpl.DisplayName() != "Coding TUI" {
		t.Errorf("DisplayName = %q", tpl.DisplayName())
	}
	if tpl.Description() != "desc" {
		t.Errorf("Description = %q", tpl.Description())
	}

	files, err := tpl.ConfigFiles(nil)
	if err != nil || len(files) != 0 {
		t.Error("ConfigFiles should return nil, nil")
	}
	if tpl.SetupCommand(nil) != "" {
		t.Error("SetupCommand should return empty")
	}
	if cmds := tpl.PlatformCommands(nil); cmds != (PlatformCommands{}) {
		t.Error("PlatformCommands should return empty")
	}
	if tpl.VerifyCommand(nil) != "" {
		t.Error("VerifyCommand should return empty")
	}
	if steps := tpl.Steps(nil); len(steps) != 0 {
		t.Error("Steps should return nil")
	}
	if err := tpl.WriteConfig(nil); err != nil {
		t.Errorf("WriteConfig should return nil, got %v", err)
	}
}

func TestWebConfigTemplate(t *testing.T) {
	tpl := newWebConfigTemplate(AgentCodingWeb, "Coding Web", "desc")

	if tpl.AgentType() != AgentCodingWeb {
		t.Errorf("AgentType = %q", tpl.AgentType())
	}
	if tpl.DisplayName() != "Coding Web" {
		t.Errorf("DisplayName = %q", tpl.DisplayName())
	}
	if tpl.Description() != "desc" {
		t.Errorf("Description = %q", tpl.Description())
	}

	files, err := tpl.ConfigFiles(nil)
	if err != nil || len(files) != 0 {
		t.Error("ConfigFiles should return nil, nil")
	}
	if tpl.SetupCommand(nil) != "" {
		t.Error("SetupCommand should return empty")
	}
}

// ============================================================================
// TemplateRegistry Tests (TUI/Web additions)
// ============================================================================

func TestTemplateRegistry_TUIWebAgents(t *testing.T) {
	reg := NewTemplateRegistry()

	// 验证 TUI Agent 可被获取
	for _, at := range []AgentType{AgentCodingTUI, AgentEducationTUI} {
		tmpl, ok := reg.Get(at)
		if !ok {
			t.Errorf("TUI agent %s should be registered", at)
		}
		if tmpl == nil {
			t.Errorf("TUI agent %s should not be nil", at)
		}
		if tmpl.AgentType() != at {
			t.Errorf("TUI agent type mismatch: %s vs %s", tmpl.AgentType(), at)
		}
	}

	// 验证 Web Agent 可被获取
	for _, at := range []AgentType{AgentCodingWeb, AgentEducationWeb} {
		tmpl, ok := reg.Get(at)
		if !ok {
			t.Errorf("Web agent %s should be registered", at)
		}
		if tmpl == nil {
			t.Errorf("Web agent %s should not be nil", at)
		}
	}

	// 验证 List 包含新类型
	allTypes := reg.List()
	foundTUI := false
	foundWeb := false
	for _, at := range allTypes {
		if at == AgentCodingTUI || at == AgentEducationTUI {
			foundTUI = true
		}
		if at == AgentCodingWeb || at == AgentEducationWeb {
			foundWeb = true
		}
	}
	if !foundTUI {
		t.Error("List should contain TUI agent types")
	}
	if !foundWeb {
		t.Error("List should contain Web agent types")
	}
}

// ============================================================================
// Specialization Registry Concurrent Tests (Task 3.2 补充)
// ============================================================================

func TestSpecializedAgentRegistry_ConcurrentAccess(t *testing.T) {
	r := NewSpecializedAgentRegistry()
	r.SeedDefaults()

	var wg sync.WaitGroup
	workers := 20
	opsPerWorker := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				// 并发读操作
				r.Get(AgentCodingTUI)
				r.GetTUIAgent(AgentCodingTUI)
				r.GetWebAgent(AgentCodingWeb)
				r.List()
				r.ListByCategory(AgentCategoryTUI)
				r.DiscoverCapabilities("code_highlight")
				r.Count()
				r.CountByCategory(AgentCategoryWeb)
				r.GetAllTUIAgents()
				r.GetAllWebAgents()
				// 并发写操作
				if j%5 == 0 {
					r.Register(&AgentSpecialization{
						Type:     AgentType(fmt.Sprintf("concurrent-test-%d-%d", id, j)),
						Category: AgentCategoryCLI,
						Capabilities: []string{"test"},
					})
				}
			}
		}(i)
	}
	wg.Wait()

	// 验证最终一致性：原来的 4 个 + 新注册的至少还在
	if r.Count() < 4 {
		t.Errorf("count should be >= 4, got %d", r.Count())
	}
}

func TestSpecializedAgentRegistry_CountByCategory(t *testing.T) {
	r := NewSpecializedAgentRegistry()
	r.SeedDefaults()

	if c := r.CountByCategory(AgentCategoryTUI); c != 2 {
		t.Errorf("TUI count = %d, want 2", c)
	}
	if c := r.CountByCategory(AgentCategoryWeb); c != 2 {
		t.Errorf("Web count = %d, want 2", c)
	}
	if c := r.CountByCategory(AgentCategoryCLI); c != 0 {
		t.Errorf("CLI count = %d, want 0", c)
	}
	if c := r.CountByCategory(AgentCategoryDesktop); c != 0 {
		t.Errorf("Desktop count = %d, want 0", c)
	}
}

func TestSpecializedAgentRegistry_SeedDefaultsIdempotent(t *testing.T) {
	r := NewSpecializedAgentRegistry()
	r.SeedDefaults()
	c1 := r.Count()
	r.SeedDefaults()
	c2 := r.Count()
	if c1 != c2 {
		t.Errorf("SeedDefaults should be idempotent: %d vs %d", c1, c2)
	}
}

func TestSpecializedAgentRegistry_DiscoverCapabilities_CaseSensitive(t *testing.T) {
	r := NewSpecializedAgentRegistry()
	r.SeedDefaults()

	// 大小写敏感
	agents := r.DiscoverCapabilities("Code_Highlight")
	if len(agents) != 0 {
		t.Errorf("DiscoverCapabilities should be case-sensitive, got %d agents", len(agents))
	}
}

func TestSpecializedAgentRegistry_GetTUIAgent_NotExist(t *testing.T) {
	r := NewSpecializedAgentRegistry()

	_, ok := r.GetTUIAgent(AgentCodingTUI)
	if ok {
		t.Error("should not find unregistered TUI agent")
	}

	_, ok = r.GetWebAgent(AgentCodingWeb)
	if ok {
		t.Error("should not find unregistered Web agent")
	}
}

// ============================================================================
// splitLines Tests
// ============================================================================

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty", "", 0},
		{"single line", "hello", 1},
		{"two lines", "hello\nworld", 2},
		{"trailing newline", "hello\n", 1},
		{"empty line between", "a\n\nb", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLines(tt.input)
			if len(result) != tt.expected {
				t.Errorf("splitLines(%q) = %d lines, want %d", tt.input, len(result), tt.expected)
			}
		})
	}
}

// ============================================================================
// BrowserConfig Tests
// ============================================================================

func TestDefaultBrowserConfig(t *testing.T) {
	cfg := DefaultBrowserConfig()
	if cfg == nil {
		t.Fatal("DefaultBrowserConfig should not be nil")
	}
	if !cfg.Headless {
		t.Error("Headless should default to true")
	}
	if cfg.ViewportWidth != 1280 {
		t.Errorf("ViewportWidth = %d, want 1280", cfg.ViewportWidth)
	}
	if cfg.ViewportHeight != 720 {
		t.Errorf("ViewportHeight = %d, want 720", cfg.ViewportHeight)
	}
	if cfg.Timeout != 30 {
		t.Errorf("Timeout = %d, want 30", cfg.Timeout)
	}
}

// ============================================================================
// Interface Compliance Verification
// ============================================================================

func TestInterfaceCompliance(t *testing.T) {
	// TUI Agents must implement TUIAgent (which extends AgentTemplate)
	var _ TUIAgent = (*TUIAgentTemplate)(nil)
	var _ TUIAgent = (*CodingTUIAgent)(nil)
	var _ TUIAgent = (*EducationTUIAgent)(nil)
	var _ AgentTemplate = (*tuiConfigTemplate)(nil)

	// Web Agents must implement WebAgent (which extends AgentTemplate)
	var _ WebAgent = (*WebAgentTemplate)(nil)
	var _ WebAgent = (*CodingWebAgent)(nil)
	var _ WebAgent = (*EducationWebAgent)(nil)
	var _ AgentTemplate = (*webConfigTemplate)(nil)

	t.Log("All interface compliance checks passed")
}

// ============================================================================
// ProgressInfo Edge Cases
// ============================================================================

func TestRenderProgress_EdgeCases(t *testing.T) {
	tpl := NewTUIAgentTemplate("test", "Test", "")

	t.Run("negative current", func(t *testing.T) {
		result := tpl.RenderProgress(&ProgressInfo{Current: -1, Total: 100})
		// Should not panic
		if result == "" {
			t.Error("negative current should produce output (clamped to 0)")
		}
	})

	t.Run("large values", func(t *testing.T) {
		result := tpl.RenderProgress(&ProgressInfo{Current: 999999, Total: 1000000})
		// 999999/1000000 * 40 = 39.999 → truncated to 39 bars filled
		if !strings.Contains(result, "99.9%") && !strings.Contains(result, "100.0%") {
			t.Errorf("large values should render correctly, got %q", result)
		}
	})
}
