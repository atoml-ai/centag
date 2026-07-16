package proxy

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gin-gonic/gin"
)

const contextKeyRawRequestBody = "centag_raw_request_body"

// cacheRawRequestBody reads and caches the HTTP body before protocol parsing consumes it.
// Transparent (#t) forwarding and tool_calls passthrough rely on the original bytes.
func cacheRawRequestBody(c *gin.Context) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return
	}
	if _, exists := c.Get(contextKeyRawRequestBody); exists {
		return
	}
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Set(contextKeyRawRequestBody, bodyBytes)
}

func rawRequestBodyFromContext(c *gin.Context) []byte {
	if c == nil {
		return nil
	}
	if v, ok := c.Get(contextKeyRawRequestBody); ok {
		if b, ok := v.([]byte); ok {
			return b
		}
	}
	return nil
}

func rawBodyJSONFromProxyRequest(rawBody any) string {
	if rawBody == nil {
		return ""
	}
	switch v := rawBody.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}