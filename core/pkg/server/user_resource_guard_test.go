package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/pkg/groupmodel"

	"github.com/gin-gonic/gin"
)

// newGuardTestContext 构造带 JSON body 与可选 X-Pipeline-ID 头的测试请求。
func newGuardTestContext(t *testing.T, model string, headerPipelineID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"model":"` + model + `","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if headerPipelineID != "" {
		req.Header.Set("X-Pipeline-ID", headerPipelineID)
	}
	c.Request = req
	return c, w
}

func guardPolicy() *groupmodel.EffectivePolicy {
	return &groupmodel.EffectivePolicy{
		Mode:                groupmodel.PolicyModeGroup,
		GroupID:             "g1",
		AllowPipelines:      []string{"transparent"},
		AllowModels:         []string{"glm-4-flash"},
		ResourcesConfigured: true,
	}
}

// TestGuardHeaderPipelineStillChecksModel 是 P1-1 的回归：
// 携带白名单内 X-Pipeline-ID 的请求仍须校验 body model，
// 堵住「header 放行即跳过 IsAllowedModel」的计划内绕过面。
func TestGuardHeaderPipelineStillChecksModel(t *testing.T) {
	s := &Server{}

	// 白名单 pipeline + 白名单外 model → 必须 403
	c, w := newGuardTestContext(t, "forbidden-model", "transparent")
	s.enforcePolicyAllowLists(c, guardPolicy())
	if w.Code != http.StatusForbidden {
		t.Fatalf("disallowed model via allowed pipeline header must 403, got %d body=%s", w.Code, w.Body.String())
	}
	if !c.IsAborted() {
		t.Fatal("request must be aborted on denial")
	}

	// 白名单 pipeline + 白名单内 model → 放行
	c2, w2 := newGuardTestContext(t, "glm-4-flash", "transparent")
	s.enforcePolicyAllowLists(c2, guardPolicy())
	if w2.Code == http.StatusForbidden || c2.IsAborted() {
		t.Fatalf("allowed model must pass, got %d body=%s", w2.Code, w2.Body.String())
	}

	// 占位符/auto model 不拦（内置 agent 场景）
	c3, w3 := newGuardTestContext(t, "auto", "transparent")
	s.enforcePolicyAllowLists(c3, guardPolicy())
	if c3.IsAborted() {
		t.Fatalf("placeholder model must not be blocked, got %d", w3.Code)
	}

	// 白名单外 pipeline → 403（原有语义保持）
	c4, w4 := newGuardTestContext(t, "glm-4-flash", "cache-pipeline")
	s.enforcePolicyAllowLists(c4, guardPolicy())
	if w4.Code != http.StatusForbidden {
		t.Fatalf("disallowed pipeline must 403, got %d", w4.Code)
	}
}
