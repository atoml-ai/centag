package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestDeps(t *testing.T) Deps {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "agent.log")
	if err := os.WriteFile(logPath, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Deps{
		DataDir: dir,
		Version: "test",
	}
}

// TC-MCP-SEC-001 mcp.enabled=false → 不实例化，挂载方不注册路由（404 由挂载方保证）。
func TestObservationServer_Disabled(t *testing.T) {
	s := NewObservationServer(false, newTestDeps(t), nil)
	if s != nil {
		t.Fatal("expected nil server when disabled")
	}
}

// TC-MCP-SEC-002 未鉴权 401。
func TestObservationServer_Unauthorized(t *testing.T) {
	deps := newTestDeps(t)
	deps.Authorize = func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer good"
	}
	s := NewObservationServer(true, deps, nil)

	srv := httptest.NewServer(s.StreamableHandler())
	defer srv.Close()

	// 无 token → 401
	resp, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	// 有效 token → 非 401
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer good")
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode == http.StatusUnauthorized {
		t.Fatalf("expected non-401 with valid token, got %d", resp2.StatusCode)
	}
}

func connectClient(t *testing.T, deps Deps, allowed []string) (clientSession *gomcp.ClientSession, close func()) {
	t.Helper()
	s := NewObservationServer(true, deps, allowed)
	srv := httptest.NewServer(s.StreamableHandler())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client := gomcp.NewClient(&gomcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	sess, err := client.Connect(ctx, &gomcp.StreamableClientTransport{Endpoint: srv.URL, DisableStandaloneSSE: true}, nil)
	if err != nil {
		srv.Close()
		cancel()
		t.Fatalf("connect: %v", err)
	}
	return sess, func() {
		_ = sess.Close()
		srv.Close()
		cancel()
	}
}

// TC-MCP-SEC-003 allowed_tools 白名单裁剪：tools/list 不可见 + tools/call 403。
func TestObservationServer_AllowedTools(t *testing.T) {
	deps := newTestDeps(t)
	sess, cleanup := connectClient(t, deps, []string{"read_log"})
	defer cleanup()

	names, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tl := range names.Tools {
		got = append(got, tl.Name)
	}
	if len(got) != 1 || got[0] != "read_log" {
		t.Fatalf("expected only read_log, got %v", got)
	}
}

// TestObservationServer_CallDisallowedTool 白名单外 tools/call 直接 403。
func TestObservationServer_CallDisallowedTool(t *testing.T) {
	deps := newTestDeps(t)
	s := NewObservationServer(true, deps, []string{"read_log"})
	srv := httptest.NewServer(s.StreamableHandler())
	defer srv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_database","arguments":{}}}`
	resp, err := http.Post(srv.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// TC-MCP-INT-001 Streamable HTTP 全流程：initialize → tools/list → tools/call → close。
func TestObservationServer_FullFlow(t *testing.T) {
	deps := newTestDeps(t)
	sess, cleanup := connectClient(t, deps, nil)
	defer cleanup()

	names, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"centag_info": true, "read_log": true, "read_database": true}
	var got []string
	for _, tl := range names.Tools {
		got = append(got, tl.Name)
	}
	if len(got) != len(want) {
		t.Fatalf("expected tools %v, got %v", want, got)
	}

	// 路径（相对 dataDir）+ lines 调 read_log，返回响应内容含 prefilled 行。
	res, err := sess.CallTool(context.Background(), &gomcp.CallToolParams{
		Name:      "read_log",
		Arguments: map[string]any{"path": "agent.log", "lines": 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*gomcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "line1") {
		t.Fatalf("expected log content, got %q", text)
	}
}

// TC-MCP-INT-002（R03 冒烟）：SSE 端点返回 text/event-stream 且首 event 为 endpoint。
func TestObservationServer_SSE(t *testing.T) {
	deps := newTestDeps(t)
	deps.Authorize = func(r *http.Request) bool { return true }
	s := NewObservationServer(true, deps, nil)
	srv := httptest.NewServer(s.SSEHandler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	head := string(buf[:n])
	if !strings.Contains(head, "event: endpoint") {
		t.Fatalf("expected endpoint event, got %q", head)
	}
}
