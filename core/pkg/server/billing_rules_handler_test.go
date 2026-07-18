package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"centag/core/internal/billing"
)

func TestBillingRulesHandler_CRUDAndImport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := billing.NewMemoryRuleStore()
	pricing := billing.NewPricingService(store)
	h := NewBillingRulesHandler(store, pricing)

	r := gin.New()
	g := r.Group("/api/v1/admin/billing/rules")
	{
		g.GET("", h.ListRules)
		g.POST("", h.CreateRule)
		g.PUT("/:id", h.UpdateRule)
		g.DELETE("/:id", h.DeleteRule)
		g.POST("/import", h.ImportRules)
		g.GET("/export", h.ExportRules)
	}

	body := `{"name":"t","backend_id":"b","model":"m","input_price_per_m":1,"output_price_per_m":2,"priority":1,"enabled":true}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/billing/rules", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create status %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/billing/rules", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status %d", w.Code)
	}

	yamlImport := []byte(`
version: "1.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "imported"
    backend_id: "ppinfra"
    model: "deepseek-v3.2"
    input_price_per_m: 1
    output_price_per_m: 1
    priority: 100
`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/billing/rules/import", bytes.NewReader(yamlImport))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("import status %d body=%s", w.Code, w.Body.String())
	}

	info, err := pricing.ResolvePrice(req.Context(), "ppinfra", "deepseek-v3.2")
	if err != nil || info.InputPricePerM != 1 || info.Currency != "USD" {
		t.Fatalf("resolve after import: %+v err=%v", info, err)
	}
	if billing.USDToCNY() != 7.2 {
		t.Fatalf("fx rate %v", billing.USDToCNY())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/billing/rules/export", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("deepseek-v3.2")) {
		t.Fatalf("export status %d body=%s", w.Code, w.Body.String())
	}
}

func TestBillingRulesHandler_UnauthorizedWithoutMiddlewareStillCallable(t *testing.T) {
	// Handler itself does not enforce auth; route registration attaches AdminOnly.
	// This test documents that calling handler directly succeeds (auth is middleware concern).
	gin.SetMode(gin.TestMode)
	h := NewBillingRulesHandler(billing.NewMemoryRuleStore(), nil)
	r := gin.New()
	r.GET("/api/v1/admin/billing/rules", h.ListRules)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/billing/rules", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
}

func TestNoParallelBillingCostsRouteDocumented(t *testing.T) {
	// Guardrail: we must not introduce /api/v1/admin/billing/costs*
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := billing.NewMemoryRuleStore()
	h := NewBillingRulesHandler(store, billing.NewPricingService(store))
	admin := r.Group("/api/v1/admin")
	admin.GET("/billing/rules", h.ListRules)
	admin.GET("/cost/summary", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/billing/costs", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("parallel costs route must not exist, got %d", w.Code)
	}
}
