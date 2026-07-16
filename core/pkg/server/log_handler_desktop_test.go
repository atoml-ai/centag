package server

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"

	"centag/core/pkg/config"
)

func TestParseLogLine_DesktopCompactConsole(t *testing.T) {
	line := `2026-06-10T12:49:29.539+0800infologger/logger.go:128request{"method": "GET", "path": "/api/v1/logs?page=1&limit=100&from=2026-06-10T03:49:29.520Z&category=llm", "status": 200, "ip": "192.0.2.1", "latency": 0.018462188}`
	entry, err := parseLogLine(line)
	if err != nil {
		t.Fatalf("parseLogLine: %v", err)
	}
	if entry.Message != "request" {
		t.Fatalf("message = %q, want request", entry.Message)
	}
	if entry.Path == "" {
		t.Fatal("expected path from compact console tail JSON")
	}
	if entry.StatusCode != 200 {
		t.Fatalf("status_code = %d, want 200", entry.StatusCode)
	}
	if entry.Timestamp == "" {
		t.Fatal("expected parsed timestamp")
	}
}

func TestMatchesFilters_HandlerStartupNotLLM(t *testing.T) {
	h := &LogHandler{}
	lines := []string{
		`{"level":"info","timestamp":"2026-06-10T14:05:19.018+0800","caller":"logger/logger.go:149","message":"[Handler] ModeDispatcher initialized with pipeline engine","backend":"ollama-local","model":"qwen2.5:1.5b"}`,
		`{"level":"info","timestamp":"2026-06-10T14:05:19.019+0800","caller":"logger/logger.go:149","message":"[Handler] ModeDispatcher store injected","backend":"ollama-local","model":"qwen2.5:1.5b"}`,
	}
	for _, line := range lines {
		entry, err := parseLogLine(line)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		propagateRequestContext([]LogEntry{entry})
		if h.matchesFilters(entry, LogQueryRequest{Category: "llm"}) {
			t.Fatalf("handler startup should not match llm: %q", entry.Message)
		}
		if !h.matchesFilters(entry, LogQueryRequest{Category: "system"}) {
			t.Fatalf("handler startup should match system: %q", entry.Message)
		}
	}
}

func TestParseLogLine_ExtraFieldsFromGenerator(t *testing.T) {
	line := `2026-06-10T12:28:07.120+0800infologger/logger.go:128[generator] 发送请求{"node_id": "generator", "model": "glm-4-flash", "message_count": 1, "user_input_preview": "hi"}`
	entry, err := parseLogLine(line)
	if err != nil {
		t.Fatalf("parseLogLine: %v", err)
	}
	if entry.Extra["user_input_preview"] != "hi" {
		t.Fatalf("user_input_preview = %q", entry.Extra["user_input_preview"])
	}
	if entry.Extra["node_id"] != "generator" {
		t.Fatalf("node_id = %q", entry.Extra["node_id"])
	}
}

func TestMatchesFilters_DesktopConsoleNoSpaceFormat(t *testing.T) {
	h := &LogHandler{}
	line := `2026-06-10T12:49:29.539+0800infologger/logger.go:128request{"method": "GET", "path": "/api/v1/logs?page=1&limit=100&from=2026-06-10T03:49:29.520Z&category=llm", "status": 200, "ip": "192.0.2.1", "latency": 0.018462188}`
	entry, err := parseLogLine(line)
	if err != nil {
		t.Fatalf("parseLogLine: %v", err)
	}
	if h.matchesFilters(entry, LogQueryRequest{Category: "llm"}) {
		t.Fatal("logs API polling should not match llm category")
	}
}

func TestMatchesFilters_ExcludeStartupAndCacheInit(t *testing.T) {
	h := &LogHandler{}
	cases := []struct {
		name string
		line string
	}{
		{
			name: "semantic cache json",
			line: `{"level":"info","timestamp":"2026-06-10T12:49:08.963+0800","caller":"logger/logger.go:149","message":"Semantic cache ready - provider: ollama, model: bge-m3:latest, auto_embedding: true"}`,
		},
		{
			name: "starting server json",
			line: `{"level":"info","timestamp":"2026-06-10T12:49:08.979+0800","caller":"logger/logger.go:149","message":"Starting server on 127.0.0.1:20060"}`,
		},
		{
			name: "auth refresh compact",
			line: `2026-06-10T12:49:09.598+0800infologger/logger.go:128auth/refresh: looking up refresh token{"hash_prefix": "b91f9c89"}`,
		},
		{
			name: "static asset request compact",
			line: `2026-06-10T12:49:12.438+0800infologger/logger.go:128request{"method": "GET", "path": "/static/assets/LogViewer-DVIv_J_6.js", "status": 200, "ip": "192.0.2.1", "latency": 0.00020059}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := parseLogLine(tc.line)
			if err != nil {
				t.Fatalf("parseLogLine: %v", err)
			}
			if h.matchesFilters(entry, LogQueryRequest{Category: "llm"}) {
				t.Fatalf("%s should not match llm category", tc.name)
			}
			if !h.matchesFilters(entry, LogQueryRequest{Category: "system"}) {
				t.Fatalf("%s should match system category", tc.name)
			}
		})
	}
}

func TestMatchesFilters_RealDesktopLogFile_NoPollution(t *testing.T) {
	logPath := os.Getenv("CENTAG_LOG_FILE")
	if logPath == "" {
		logPath = os.Getenv("PROXYCLAW_LOG_FILE")
	}
	if logPath == "" {
		logPath = os.ExpandEnv("$HOME/Library/Application Support/Centag/logs/centag.log")
	}
	f, err := os.Open(logPath)
	if err != nil {
		t.Skip("desktop log file not available:", err)
	}
	defer f.Close()

	h := &LogHandler{}
	bad := []string{
		`"path": "/api/v1/logs`,
		`"path": "/static/assets/`,
		"auth/refresh:",
		"Semantic cache ready",
		"Starting server on",
	}
	var llmMatched int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		entry, err := parseLogLine(line)
		if err != nil {
			continue
		}
		if !h.matchesFilters(entry, LogQueryRequest{Category: "llm"}) {
			continue
		}
		llmMatched++
		lower := strings.ToLower(entry.Message)
		for _, fragment := range bad {
			if strings.Contains(lower, strings.ToLower(fragment)) {
				t.Fatalf("llm category matched noise: fragment=%q message=%q", fragment, entry.Message)
			}
		}
	}
	t.Logf("llm category matches in file: %d", llmMatched)
}
func TestRecentLogLines_LLMFilterSimulation(t *testing.T) {
	h := &LogHandler{}
	lines := []string{
		`2026-06-10T12:54:50.193+0800infologger/logger.go:128request{"method": "GET", "path": "/api/v1/status", "status": 200, "ip": "127.0.0.1", "latency": 0.000149214}`,
		`2026-06-10T12:54:26.000+0800infoserver/server.go:100Starting server on 127.0.0.1:20060`,
		`2026-06-10T12:54:27.000+0800infologger/logger.go:128auth/refresh: looking up refresh token{"hash_prefix": "7bd4d428"}`,
		`{"level":"info","timestamp":"2026-06-10T12:54:26.000+0800","caller":"logger/logger.go:149","message":"Proxy mode middleware registered for LLM proxy routes"}`,
	}
	for _, line := range lines {
		entry, err := parseLogLine(line)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		match := h.matchesFilters(entry, LogQueryRequest{Category: "llm"})
		t.Logf("match=%v msg=%q path=%q", match, entry.Message, entry.Path)
		if match {
			t.Fatalf("should not match llm: %q", entry.Message)
		}
	}
}

func TestReadAndFilterLogs_DesktopLogPath_LLMScope(t *testing.T) {
	logDir := os.ExpandEnv("$HOME/Library/Application Support/ProxyClaw/logs")
	if _, err := os.Stat(logDir); err != nil {
		t.Skip("desktop log dir not available:", err)
	}
	cfg := &config.Config{}
	cfg.Log.File.Path = logDir
	cfg.Log.File.Filename = "centag.log"
	cfg.Log.Output = "file"

	h := NewLogHandler(cfg)
	from := time.Now().Add(-1 * time.Hour)
	logs, total, err := h.readAndFilterLogs(from, time.Now(), LogQueryRequest{
		Category: "llm",
		Limit:    100,
		Page:     1,
	}, 1, 100)
	if err != nil {
		t.Fatalf("readAndFilterLogs: %v", err)
	}
	t.Logf("llm scope: total=%d page=%d", total, len(logs))

	noiseFragments := []string{
		"/api/v1/logs", "/api/v1/status", "/static/assets/", "auth/refresh",
		"starting server", "semantic cache", "proxy mode middleware",
		"host proxy", "listen tcp", "plugin registry",
	}
	for _, entry := range logs {
		hay := strings.ToLower(strings.Join([]string{entry.Message, entry.Path}, " "))
		for _, frag := range noiseFragments {
			if strings.Contains(hay, frag) {
				t.Fatalf("noise in llm results: frag=%q entry=%+v", frag, entry)
			}
		}
	}
}
