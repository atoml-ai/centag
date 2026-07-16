package reasoning

import "testing"

func TestNormalizeEffort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", "none"},
		{"none", "none", "none"},
		{"minimal", "minimal", "minimal"},
		{"low", "low", "low"},
		{"medium", "medium", "medium"},
		{"high", "high", "high"},
		{"xhigh", "xhigh", "xhigh"},
		{"alias off", "off", "none"},
		{"alias med", "med", "medium"},
		{"uppercase", "HIGH", "high"},
		{"mixed case", "Medium", "medium"},
		{"with spaces", "  high  ", "high"},
		{"invalid", "invalid", "none"},
		{"invalid with alias", "OFF", "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeEffort(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeEffort(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEffortToBudget(t *testing.T) {
	tests := []struct {
		name     string
		effort   string
		expected int
	}{
		{"none", "none", 0},
		{"minimal", "minimal", 1024},
		{"low", "low", 2048},
		{"medium", "medium", 4096},
		{"high", "high", 8192},
		{"xhigh", "xhigh", 16384},
		{"alias off", "off", 0},
		{"alias med", "med", 4096},
		{"invalid", "invalid", 0},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EffortToBudget(tt.effort)
			if result != tt.expected {
				t.Errorf("EffortToBudget(%q) = %d, want %d", tt.effort, result, tt.expected)
			}
		})
	}
}

func TestBudgetToEffort(t *testing.T) {
	tests := []struct {
		name     string
		budget   int
		expected string
	}{
		{"zero", 0, "none"},
		{"negative", -100, "none"},
		{"below minimal", 512, "none"},
		{"minimal", 1024, "minimal"},
		{"between minimal and low", 1536, "minimal"},
		{"low", 2048, "low"},
		{"between low and medium", 3000, "low"},
		{"medium", 4096, "medium"},
		{"between medium and high", 6000, "medium"},
		{"high", 8192, "high"},
		{"between high and xhigh", 12000, "high"},
		{"xhigh", 16384, "xhigh"},
		{"above xhigh", 20000, "xhigh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BudgetToEffort(tt.budget)
			if result != tt.expected {
				t.Errorf("BudgetToEffort(%d) = %q, want %q", tt.budget, result, tt.expected)
			}
		})
	}
}

func TestIsValidEffort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"none", "none", true},
		{"minimal", "minimal", true},
		{"low", "low", true},
		{"medium", "medium", true},
		{"high", "high", true},
		{"xhigh", "xhigh", true},
		{"alias off", "off", true},
		{"alias med", "med", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidEffort(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidEffort(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEffortLevels(t *testing.T) {
	levels := EffortLevels()
	if len(levels) != 6 {
		t.Errorf("EffortLevels() returned %d levels, want 6", len(levels))
	}

	expected := []string{"none", "minimal", "low", "medium", "high", "xhigh"}
	for i, level := range levels {
		if level != expected[i] {
			t.Errorf("EffortLevels()[%d] = %q, want %q", i, level, expected[i])
		}
	}
}
