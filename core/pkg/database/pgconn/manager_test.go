package pgconn

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Host != "localhost" {
		t.Errorf("expected host 'localhost', got '%s'", config.Host)
	}

	if config.Port != 5432 {
		t.Errorf("expected port 5432, got %d", config.Port)
	}

	if config.User != "postgres" {
		t.Errorf("expected user 'postgres', got '%s'", config.User)
	}

	if config.Database != "centag" {
		t.Errorf("expected database 'centag', got '%s'", config.Database)
	}

	if config.SSLMode != "disable" {
		t.Errorf("expected ssl_mode 'disable', got '%s'", config.SSLMode)
	}
}

func TestConfigDSN(t *testing.T) {
	config := &Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "password",
		Database: "centag",
		SSLMode:  "disable",
	}

	dsn := config.DSN()

	if dsn == "" {
		t.Error("expected non-empty DSN")
	}

	// 验证 DSN 包含必要的参数
	if !contains(dsn, "host=localhost") {
		t.Error("DSN should contain host=localhost")
	}

	if !contains(dsn, "port=5432") {
		t.Error("DSN should contain port=5432")
	}

	if !contains(dsn, "user=postgres") {
		t.Error("DSN should contain user=postgres")
	}

	if !contains(dsn, "password=password") {
		t.Error("DSN should contain password=password")
	}

	if !contains(dsn, "dbname=centag") {
		t.Error("DSN should contain dbname=centag")
	}

	if !contains(dsn, "sslmode=disable") {
		t.Error("DSN should contain sslmode=disable")
	}
}

func TestEnvFirst(t *testing.T) {
	// 测试空值
	result := envFirst()
	if result != "" {
		t.Errorf("expected empty string, got '%s'", result)
	}
}

func TestEnvIntFirst(t *testing.T) {
	// 测试默认值
	result := envIntFirst(42)
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestNewManager(t *testing.T) {
	manager := NewManager()

	if manager == nil {
		t.Error("expected non-nil manager")
	}

	if manager.config == nil {
		t.Error("expected non-nil config")
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	config := &Config{
		Host: "testhost",
		Port: 1234,
	}

	manager := NewManagerWithConfig(config)

	if manager == nil {
		t.Error("expected non-nil manager")
	}

	if manager.config != config {
		t.Error("expected manager to use provided config")
	}
}

func TestGetConfig(t *testing.T) {
	config := &Config{
		Host: "testhost",
		Port: 1234,
	}

	manager := NewManagerWithConfig(config)
	result := manager.GetConfig()

	if result != config {
		t.Error("expected GetConfig to return the provided config")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

