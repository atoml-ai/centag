package proxy

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveSessionID_PrefersSessionHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-Session-ID", "sess-abc")
	c.Request.Header.Set("X-Request-ID", "req-1")
	if got := resolveSessionID(c); got != "sess-abc" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSessionID_FallsBackToRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-Request-ID", "req-9")
	if got := resolveSessionID(c); got != "req_req-9" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSessionID_Empty(t *testing.T) {
	if resolveSessionID(nil) != "" {
		t.Fatal("nil context should return empty")
	}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	if got := resolveSessionID(c); got != "" {
		t.Fatalf("got %q", got)
	}
}
