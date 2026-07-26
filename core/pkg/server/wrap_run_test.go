package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/internal/edition"
	"centag/core/pkg/config"

	"github.com/gin-gonic/gin"
)

func TestParseWrapArgv_FromArgv(t *testing.T) {
	got, err := parseWrapArgv([]string{"opencode", "--help"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "opencode" || got[1] != "--help" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseWrapArgv_FromCommand(t *testing.T) {
	got, err := parseWrapArgv(nil, "  claude   --resume  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "claude" || got[1] != "--resume" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseWrapArgv_RejectsMeta(t *testing.T) {
	cases := []string{
		"opencode; rm -rf /",
		"opencode && id",
		"opencode | cat",
		"opencode $(id)",
		"opencode `id`",
	}
	for _, c := range cases {
		if _, err := parseWrapArgv(nil, c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
	if _, err := parseWrapArgv([]string{"opencode;x"}, ""); err == nil {
		t.Fatal("expected error for argv meta")
	}
}

func TestParseWrapArgv_Empty(t *testing.T) {
	if _, err := parseWrapArgv(nil, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildWrapRunCommandLine(t *testing.T) {
	got, err := buildWrapRunCommandLine("/usr/local/bin/centag-personal", "http://127.0.0.1:20060", "llmproxy_x", []string{"opencode"})
	if err != nil {
		t.Fatal(err)
	}
	wantParts := []string{
		"/usr/local/bin/centag-personal",
		"wrap", "run",
		"--server", "http://127.0.0.1:20060",
		"--token", "llmproxy_x",
		"--", "opencode",
	}
	for _, p := range wantParts {
		if !strings.Contains(got, p) {
			t.Fatalf("missing %q in %q", p, got)
		}
	}
}

func TestBuildWrapRunUserCommand(t *testing.T) {
	got, err := buildWrapRunUserCommand("http://127.0.0.1:20060", "", []string{"opencode"})
	if err != nil {
		t.Fatal(err)
	}
	want := "centag wrap run --server http://127.0.0.1:20060 -- opencode"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	if shellQuote("opencode") != "opencode" {
		t.Fatalf("simple: %q", shellQuote("opencode"))
	}
	if got := shellQuote("a b"); got != "'a b'" {
		t.Fatalf("space: %q", got)
	}
	if got := shellQuote("it's"); !strings.Contains(got, `'\''`) {
		t.Fatalf("quote escape: %q", got)
	}
}

func TestIsLoopbackIP(t *testing.T) {
	if !isLoopbackIP("127.0.0.1") || !isLoopbackIP("::1") || !isLoopbackIP("localhost") {
		t.Fatal("expected loopback")
	}
	if isLoopbackIP("192.168.1.1") || isLoopbackIP("") {
		t.Fatal("expected non-loopback")
	}
}

func TestWrapPresetByID(t *testing.T) {
	p, ok := wrapPresetByID("opencode")
	if !ok || p.Argv[0] != "opencode" {
		t.Fatalf("got %#v ok=%v", p, ok)
	}
	// Desktop GUI apps are not wrap targets; companion CLIs may be (codebuddy → CLI argv).
	if _, ok := wrapPresetByID("claude-desktop"); ok {
		t.Fatal("desktop GUI presets must not be listed")
	}
	cb, ok := wrapPresetByID("codebuddy")
	if !ok || len(cb.Argv) == 0 || cb.Argv[0] != "codebuddy" {
		t.Fatalf("codebuddy should wrap CLI argv, got %#v ok=%v", cb, ok)
	}
}

func TestEnsureWrapRunAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	personal := &Server{edition: edition.Personal, cfg: &config.Config{}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "192.168.1.9:1234"
	if err := personal.ensureWrapRunAllowed(c); err != nil {
		t.Fatalf("personal should allow: %v", err)
	}

	team := &Server{edition: edition.Team, cfg: &config.Config{}}
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c2.Request.RemoteAddr = "192.168.1.9:1234"
	// ClientIP may fall back to RemoteAddr host
	if err := team.ensureWrapRunAllowed(c2); err == nil {
		// Gin ClientIP from RemoteAddr 192.168.1.9 should reject
		t.Fatal("team remote should reject")
	}

	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c3.Request.RemoteAddr = "127.0.0.1:9999"
	if err := team.ensureWrapRunAllowed(c3); err != nil {
		t.Fatalf("team loopback should allow: %v", err)
	}
}
