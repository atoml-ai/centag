package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSplitForcedRoutePipelineID(t *testing.T) {
	cases := []struct {
		in              string
		wantBase        string
		wantForcedRoute string
	}{
		{"agent-skill-router:status-check", "agent-skill-router", "status-check"},
		{"agent-skill-router", "agent-skill-router", ""},
		{" agent-skill-router : config-analysis ", "agent-skill-router", "config-analysis"},
		{"direct-backend", "direct-backend", ""},
		{"", "", ""},
	}
	for _, tc := range cases {
		base, forced := splitForcedRoutePipelineID(tc.in)
		if base != tc.wantBase || forced != tc.wantForcedRoute {
			t.Errorf("splitForcedRoutePipelineID(%q) = (%q,%q), want (%q,%q)",
				tc.in, base, forced, tc.wantBase, tc.wantForcedRoute)
		}
	}
}

// TestForcedRouteMetadata 验证 X-Pipeline-ID 带强制路由后缀时：
//  1. buildMetadata 将强制路由写入 forced_route，且 pipeline_id 为去后缀后的 base；
//  2. 无后缀时 forced_route 为空。
func TestForcedRouteMetadata(t *testing.T) {
	d := &ModeDispatcher{}
	headers := map[string]string{"X-Pipeline-ID": "agent-skill-router:status-check"}
	meta := d.buildMetadata(testForcedRouteGinContext(headers), ModeDefault, headers, map[string]string{})
	if meta["pipeline_id"] != "agent-skill-router" {
		t.Errorf("pipeline_id = %v, want agent-skill-router", meta["pipeline_id"])
	}
	if meta["forced_route"] != "status-check" {
		t.Errorf("forced_route = %v, want status-check", meta["forced_route"])
	}

	headers2 := map[string]string{"X-Pipeline-ID": "agent-skill-router"}
	meta2 := d.buildMetadata(testForcedRouteGinContext(headers2), ModeDefault, headers2, map[string]string{})
	if meta2["pipeline_id"] != "agent-skill-router" {
		t.Errorf("pipeline_id = %v, want agent-skill-router", meta2["pipeline_id"])
	}
	if _, ok := meta2["forced_route"]; ok {
		t.Errorf("forced_route should be absent without suffix, got %v", meta2["forced_route"])
	}
}

func testForcedRouteGinContext(headers map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	for k, v := range headers {
		c.Request.Header.Set(k, v)
	}
	return c
}
