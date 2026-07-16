package logger

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"invalid", zapcore.InfoLevel},
		{"", zapcore.InfoLevel},
	}

	for _, tt := range tests {
		level, err := parseLevel(tt.input)
		if err != nil {
			t.Errorf("parseLevel(%q) returned error: %v", tt.input, err)
		}

		if level != tt.expected {
			t.Errorf("parseLevel(%q) = %v, want %v", tt.input, level, tt.expected)
		}
	}
}

func TestParseLevel_CaseSensitive(t *testing.T) {
	upperCases := []string{"DEBUG", "INFO", "WARN", "ERROR"}

	for _, input := range upperCases {
		level, _ := parseLevel(input)
		if level != zapcore.InfoLevel {
			t.Errorf("parseLevel(%q) should return InfoLevel (default) for uppercase input, got %v", input, level)
		}
	}
}

func TestInit(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if Logger == nil {
		t.Error("Logger should not be nil after Init")
	}
	if Sugar == nil {
		t.Error("Sugar should not be nil after Init")
	}
}

func TestInit_InvalidLevel(t *testing.T) {
	cfg := Config{
		Level:  "invalid_level",
		Format: "console",
		Output: "stdout",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init should not fail with invalid level (should default to info): %v", err)
	}
}

func TestInit_JsonFormat(t *testing.T) {
	cfg := Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init with json format failed: %v", err)
	}
}

func TestLogFunctions_NoPanic(t *testing.T) {
	cfg := Config{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	}
	Init(cfg)

	Debug("test debug message")
	Info("test info message")
	Warn("test warn message")
	Error("test error message")
}

func TestLogfFunctions_NoPanic(t *testing.T) {
	cfg := Config{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	}
	Init(cfg)

	Debugf("test debug: %s", "formatted")
	Infof("test info: %s", "formatted")
	Warnf("test warn: %s", "formatted")
	Errorf("test error: %s", "formatted")
}

func TestSync(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	}
	Init(cfg)

	err := Sync()
	if err != nil {
		t.Logf("Sync returned error (expected in test env): %v", err)
	}
}

func TestSync_NilLogger(t *testing.T) {
	originalLogger := Logger
	Logger = nil

	err := Sync()
	if err != nil {
		t.Errorf("Sync should not error when Logger is nil, got: %v", err)
	}

	Logger = originalLogger
}
