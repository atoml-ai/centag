package proxy

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCacheRawRequestBody_PreservedForTransparentMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"gpt-4","messages":[],"tools":[{"type":"function"}],"tool_choice":"auto"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))

	cacheRawRequestBody(c)

	got := rawRequestBodyFromContext(c)
	if string(got) != body {
		t.Fatalf("cached body = %q, want %q", string(got), body)
	}

	meta := map[string]interface{}{}
	attachTransparentRequestMetadata(c, meta, nil)
	if meta["raw_request_body"] != body {
		t.Fatalf("raw_request_body = %v, want original JSON with tools", meta["raw_request_body"])
	}

	// Body stream should remain readable after attach.
	buf := make([]byte, len(got))
	n, err := c.Request.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("re-read body: %v", err)
	}
	if n != len(got) || !bytes.Equal(buf[:n], got) {
		t.Fatalf("body stream changed after metadata attach")
	}
}