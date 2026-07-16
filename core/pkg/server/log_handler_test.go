package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLogLine_JSONBackendAlias(t *testing.T) {
	line := `{"timestamp":"2026-06-10T10:00:00.123Z","level":"info","message":"forward","request_id":"r1","backend":"bigmodel","model":"glm-4"}`
	entry, err := parseLogLine(line)
	if err != nil {
		t.Fatalf("parseLogLine: %v", err)
	}
	if entry.BackendID != "bigmodel" {
		t.Fatalf("backend_id = %q, want bigmodel", entry.BackendID)
	}
	if entry.Model != "glm-4" {
		t.Fatalf("model = %q", entry.Model)
	}
}

func TestPropagateRequestContext(t *testing.T) {
	logs := []LogEntry{
		{Timestamp: "2026-06-10T10:00:00Z", Message: "[Chat Completions] request started", RequestID: "req-1"},
		{Timestamp: "2026-06-10T10:00:01Z", Message: "[Config] proxy mode"},
		{Timestamp: "2026-06-10T10:00:02Z", Message: "[Request] details", Model: "glm-4"},
		{Timestamp: "2026-06-10T10:00:03Z", Message: "strategy step"},
	}
	propagateRequestContext(logs)
	if logs[1].RequestID != "req-1" {
		t.Fatalf("line 1 request_id = %q", logs[1].RequestID)
	}
	if logs[3].RequestID != "req-1" {
		t.Fatalf("line 3 request_id = %q", logs[3].RequestID)
	}
	if logs[3].Model != "glm-4" {
		t.Fatalf("line 3 model = %q", logs[3].Model)
	}
}

func TestParseLogLine_JSONWithRequestID(t *testing.T) {
	line := `{"timestamp":"2026-06-10T10:00:00.123Z","level":"info","message":"forward","request_id":"req-abc","backend_id":"bigmodel","model":"glm-4","status_code":200,"duration_ms":842,"client_ip":"127.0.0.1"}`
	entry, err := parseLogLine(line)
	if err != nil {
		t.Fatalf("parseLogLine: %v", err)
	}
	if entry.RequestID != "req-abc" {
		t.Fatalf("request_id = %q, want req-abc", entry.RequestID)
	}
	if entry.BackendID != "bigmodel" {
		t.Fatalf("backend_id = %q", entry.BackendID)
	}
	if entry.StatusCode != 200 {
		t.Fatalf("status_code = %d", entry.StatusCode)
	}
	if entry.DurationMs != 842 {
		t.Fatalf("duration_ms = %d", entry.DurationMs)
	}
	if entry.ClientIP != "127.0.0.1" {
		t.Fatalf("client_ip = %q", entry.ClientIP)
	}
}

func TestParseLogLine_ConsoleRequestMessage(t *testing.T) {
	line := `2026-06-10T10:00:00.123+0800	info	proxy/handler.go:108	[Request] ID: 1718001234567890 | Method: POST | Path: /v1/chat/completions`
	entry, err := parseLogLine(line)
	if err != nil {
		t.Fatalf("parseLogLine: %v", err)
	}
	if entry.RequestID != "1718001234567890" {
		t.Fatalf("request_id = %q", entry.RequestID)
	}
	if entry.Level != "info" {
		t.Fatalf("level = %q", entry.Level)
	}
}

func TestParseLogLine_ConsoleWithTailJSON(t *testing.T) {
	line := `2026-06-10T10:00:00.123Z	info	strategy_logger.go:23	=== start ===	{"request_id":"rid-1","requested_model":"gpt-4"}`
	entry, err := parseLogLine(line)
	if err != nil {
		t.Fatalf("parseLogLine: %v", err)
	}
	if entry.RequestID != "rid-1" {
		t.Fatalf("request_id = %q", entry.RequestID)
	}
	if entry.Model != "gpt-4" {
		t.Fatalf("model = %q", entry.Model)
	}
}

func TestMatchesFilters_QAndRequestID(t *testing.T) {
	h := &LogHandler{}
	entry := LogEntry{
		Message:   "[Request] ID: abc123 | Path: /v1/chat/completions",
		RequestID: "abc123",
		Level:     "info",
	}

	if !h.matchesFilters(entry, LogQueryRequest{Q: "chat/completions"}) {
		t.Fatal("expected q match")
	}
	if !h.matchesFilters(entry, LogQueryRequest{RequestID: "abc"}) {
		t.Fatal("expected request_id partial match")
	}
	if h.matchesFilters(entry, LogQueryRequest{Q: "not-found"}) {
		t.Fatal("expected q mismatch")
	}
}

func TestMatchesFilters_ExcludeLogsPollingAccess(t *testing.T) {
	h := &LogHandler{}
	poll := LogEntry{
		Message:   `request {"method": "GET", "path": "/api/v1/logs?page=1&category=llm", "status": 200}`,
		BackendID: "ollama-local",
		Model:     "bge-m3:latest",
	}
	if h.matchesFilters(poll, LogQueryRequest{Category: "llm"}) {
		t.Fatal("logs API polling should not match llm category")
	}
	chat := LogEntry{
		Message: "request",
		Path:    "/v1/chat/completions",
	}
	if !h.matchesFilters(chat, LogQueryRequest{Category: "llm"}) {
		t.Fatal("chat completions access should match llm category")
	}
}

func TestMatchesFilters_LLMServiceCategory(t *testing.T) {
	h := &LogHandler{}

	apiEntry := LogEntry{Message: "[Chat Completions] request started", RequestID: "r1"}
	if !h.matchesFilters(apiEntry, LogQueryRequest{Category: "llm"}) {
		t.Fatal("expected llm category match")
	}
	if !h.matchesFilters(apiEntry, LogQueryRequest{Category: "api"}) {
		t.Fatal("expected api alias match")
	}

	chainEntry := LogEntry{Message: "[Config] proxy mode"}
	if !h.matchesFilters(chainEntry, LogQueryRequest{Category: "llm"}) {
		t.Fatal("expected propagated-style entry without id to match by message")
	}

	noise := LogEntry{Message: "bootstrap: first-run complete"}
	if h.matchesFilters(noise, LogQueryRequest{Category: "llm"}) {
		t.Fatal("expected llm category mismatch for bootstrap noise")
	}
	if !h.matchesFilters(noise, LogQueryRequest{Category: "system"}) {
		t.Fatal("expected system category match for bootstrap noise")
	}
}

func TestFilterHaystackIncludesRequestID(t *testing.T) {
	hay := filterHaystack(LogEntry{RequestID: "xyz", Message: "hello"})
	if !strings.Contains(hay, "xyz") {
		t.Fatalf("haystack missing request_id: %q", hay)
	}
}

func TestClearLogs_ResetsFileStats(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "centag.log")
	lines := []string{
		`{"timestamp":"2026-06-10T10:00:00.123Z","level":"warn","message":"disk warning"}`,
		`{"timestamp":"2026-06-10T10:01:00.123Z","level":"info","message":"[Chat Completions] request started","request_id":"r1"}`,
		`{"timestamp":"2026-06-10T10:02:00.123Z","level":"error","message":"backend down"}`,
	}
	if err := os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	h := &LogHandler{logPath: logPath}
	from := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)

	allStats, err := h.collectFileStats(logPath, from, to, LogQueryRequest{})
	if err != nil {
		t.Fatalf("collectFileStats: %v", err)
	}
	if allStats.TotalLogs != 3 {
		t.Fatalf("total before clear = %d, want 3", allStats.TotalLogs)
	}

	llmStats, err := h.collectFileStats(logPath, from, to, LogQueryRequest{Category: "llm"})
	if err != nil {
		t.Fatalf("collectFileStats llm: %v", err)
	}
	if llmStats.TotalLogs != 1 {
		t.Fatalf("llm total before clear = %d, want 1", llmStats.TotalLogs)
	}

	files, err := h.findLogFiles()
	if err != nil {
		t.Fatalf("findLogFiles: %v", err)
	}
	for _, path := range files {
		if path == h.logPath {
			if err := os.Truncate(path, 0); err != nil {
				t.Fatalf("truncate: %v", err)
			}
			continue
		}
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove %s: %v", path, err)
		}
	}

	afterAll, err := h.collectFileStats(logPath, from, to, LogQueryRequest{})
	if err != nil {
		t.Fatalf("collectFileStats after clear: %v", err)
	}
	if afterAll.TotalLogs != 0 || afterAll.WarnCount != 0 || afterAll.ErrorCount != 0 {
		t.Fatalf("stats after clear = %+v, want zeros", afterAll)
	}
}