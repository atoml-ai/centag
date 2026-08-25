package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
)

func TestReadConfigTool_Execute(t *testing.T) {
	dataDir := t.TempDir()
	inner := filepath.Join(dataDir, "config.yaml")
	if err := os.WriteFile(inner, []byte("port: 20060\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := NewReadConfigTool(dataDir)

	t.Run("read text file", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{"path": "config.yaml"})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("unexpected error: %s", res.Content)
		}
		if res.Content != "port: 20060\n" {
			t.Errorf("content = %q, want %q", res.Content, "port: 20060\n")
		}
	})

	t.Run("read json formatted", func(t *testing.T) {
		jsonPath := filepath.Join(dataDir, "app.json")
		if err := os.WriteFile(jsonPath, []byte(`{"a":1}`), 0o644); err != nil {
			t.Fatal(err)
		}
		res, err := tool.Execute(context.Background(), map[string]any{"path": "app.json"})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("unexpected error: %s", res.Content)
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(res.Content), &parsed); err != nil {
			t.Errorf("content not valid JSON: %v -> %s", err, res.Content)
		}
	})

	t.Run("path escape rejected", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{"path": "../../etc/passwd"})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Error("path escape should be rejected")
		}
	})

	t.Run("empty path lists candidates", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("unexpected error: %s", res.Content)
		}
		if res.Content == "" {
			t.Error("empty path should list candidates")
		}
	})
}

func TestReadLogTool_Execute(t *testing.T) {
	dataDir := t.TempDir()
	logPath := filepath.Join(dataDir, "app.log")
	lines := []string{"line one", "line two", "line three", "error found", "line five"}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := NewReadLogTool(dataDir)

	t.Run("read all lines", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{"path": "app.log"})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("unexpected error: %s", res.Content)
		}
		if len(res.Content) == 0 {
			t.Error("log content empty")
		}
	})

	t.Run("filter", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{"path": "app.log", "filter": "error"})
		if err != nil {
			t.Fatal(err)
		}
		if res.Content != "error found" {
			t.Errorf("filtered content = %q, want %q", res.Content, "error found")
		}
	})

	t.Run("limit lines", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{"path": "app.log", "lines": float64(2)})
		if err != nil {
			t.Fatal(err)
		}
		if got := len(res.Content); got == 0 {
			t.Error("limited log content empty")
		}
	})

	t.Run("no match", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{"path": "app.log", "filter": "nonexistent"})
		if err != nil {
			t.Fatal(err)
		}
		if res.Content != "没有找到匹配的日志条目" {
			t.Errorf("content = %q, want no-match message", res.Content)
		}
	})

	t.Run("path escape rejected", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{"path": "../outside.log"})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Error("path escape should be rejected")
		}
	})
}

func TestWriteConfigTool_Execute(t *testing.T) {
	dataDir := t.TempDir()
	tool := NewWriteConfigTool(dataDir)

	t.Run("write valid json", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{
			"path":    "config/app.json",
			"content": `{"port": 20060}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("unexpected error: %s", res.Content)
		}
		b, err := os.ReadFile(filepath.Join(dataDir, "config", "app.json"))
		if err != nil {
			t.Fatalf("file not written: %v", err)
		}
		var parsed interface{}
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Errorf("written content not JSON: %v", err)
		}
	})

	t.Run("invalid json rejected", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{
			"path":    "config/bad.json",
			"content": "{not json}",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Error("invalid json should be rejected")
		}
	})

	t.Run("missing params", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Error("missing params should error")
		}
	})

	t.Run("path escape rejected", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), map[string]any{
			"path":    "../../escape.json",
			"content": `{}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Error("path escape should be rejected")
		}
	})
}

func TestToolRegistry_ListTools(t *testing.T) {
	r := NewToolRegistry(t.TempDir(), nil, nil)
	tools := r.ListTools()
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.Name()] = true
	}
	for _, want := range []string{"read_config", "read_log", "read_database", "write_config", "analyze"} {
		if !names[want] {
			t.Errorf("tool %s not registered", want)
		}
	}
	if r.GetRegistry() == nil {
		t.Error("GetRegistry() nil")
	}
}

func TestReadConfigTool_Metadata(t *testing.T) {
	tool := NewReadConfigTool(t.TempDir())
	if tool.Name() != "read_config" {
		t.Errorf("Name() = %q, want read_config", tool.Name())
	}
	if !tool.IsReadOnly() {
		t.Error("read_config should be read-only")
	}
	if tool.ParamSchema() == nil || tool.Description() == "" {
		t.Error("metadata incomplete")
	}
}

func TestToolResult_Metadata(t *testing.T) {
	res := &agentcore.ToolResult{Content: "x", IsError: true}
	if res.Content != "x" || !res.IsError {
		t.Error("ToolResult fields wrong")
	}
}
