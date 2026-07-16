package logger

import (
	"errors"
	"testing"
)

func TestLogOperation(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	}
	Init(cfg)

	LogOperation("test_operation", GetField("key", "value"))
}

func TestLogError(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	}
	Init(cfg)

	err := errors.New("test error")
	LogError("test_operation", err, GetField("key", "value"))
}

func TestLogOperationWithError_Error(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	}
	Init(cfg)

	err := errors.New("test error")
	LogOperationWithError("test operation", "test_name", err)
}

func TestLogOperationWithError_Success(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	}
	Init(cfg)

	LogOperationWithError("test operation", "test_name", nil)
}

func TestGetField(t *testing.T) {
	field := GetField("test_key", "test_value")

	if field.Key != "test_key" {
		t.Errorf("GetField Key = %s, want test_key", field.Key)
	}
}

func TestGetField_Int(t *testing.T) {
	field := GetField("count", 42)

	if field.Key != "count" {
		t.Errorf("GetField Key = %s, want count", field.Key)
	}
}

func TestGetField_Bool(t *testing.T) {
	field := GetField("enabled", true)

	if field.Key != "enabled" {
		t.Errorf("GetField Key = %s, want enabled", field.Key)
	}
}
