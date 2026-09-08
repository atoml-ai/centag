package pgconn

import (
	"net/url"
	"strconv"
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

	// URL 形态：postgres://user:pass@host:port/db?...
	if !contains(dsn, "postgres://") {
		t.Error("DSN should be a postgres:// URL")
	}
	if u, err := url.Parse(dsn); err != nil {
		t.Fatalf("DSN should parse as URL: %v", err)
	} else {
		if u.Hostname() != "localhost" {
			t.Error("DSN host should be localhost")
		}
		if p, err := strconv.Atoi(u.Port()); err != nil || p != 5432 {
			t.Errorf("DSN port should be 5432, got %d", p)
		}
		if u.Path != "/centag" {
			t.Errorf("database path should be /centag, got %q", u.Path)
		}
		if u.User == nil || u.User.Username() != "postgres" {
			t.Error("DSN userinfo username should be postgres")
		}
	}
	if !contains(dsn, "sslmode=disable") {
		t.Error("DSN should contain sslmode=disable")
	}
}

func TestConfigDSNSpecialCharsEscaped(t *testing.T) {
	config := &Config{
		Host:     "db.example.com",
		Port:     5432,
		User:     "svc user",
		Password: "p@ss\"word / x",
		Database: "centdb/1",
		SSLMode:  "disable",
	}

	dsn := config.DSN()

	// URL 转义后，空格/斜杠/引号不再破坏结构
	if u, err := url.Parse(dsn); err != nil {
		t.Fatalf("DSN should parse as URL: %v", err)
	} else {
		if u.User == nil {
			t.Error("DSN should carry userinfo")
		} else {
			if pw, ok := u.User.Password(); !ok || pw != `p@ss"word / x` {
				t.Errorf("password should round-trip via URL escaping, got %q", pw)
			}
			if u.User.Username() != "svc user" {
				t.Errorf("username should round-trip, got %q", u.User.Username())
			}
		}
		if u.Path != "/centdb/1" {
			t.Errorf("database path should round-trip, got %q", u.Path)
		}
	}
}

func TestConfigDSNUnixSocket(t *testing.T) {
	config := &Config{
		Host:     "/var/run/postgresql",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Database: "centag",
		SSLMode:  "disable",
	}

	dsn := config.DSN()

	if u, err := url.Parse(dsn); err != nil {
		t.Fatalf("DSN should parse as URL: %v", err)
	} else {
		if u.Path != "/centag" {
			t.Errorf("database path expected /centag, got %q", u.Path)
		}
		if host := u.Query().Get("host"); host != "/var/run/postgresql" {
			t.Errorf("unix socket dir should be host query param, got %q", host)
		}
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

