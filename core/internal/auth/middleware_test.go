package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	_ = logger.Init(logger.Config{Level: "error", Format: "text"})
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// ── isModelAllowed ───────────────────────────────────────────────────────────

func TestIsModelAllowed_JSONArray_Match(t *testing.T) {
	if !isModelAllowed("gpt-4", `["gpt-4","gpt-3.5-turbo"]`) {
		t.Error("model in JSON array should match")
	}
}

func TestIsModelAllowed_JSONArray_NoMatch(t *testing.T) {
	if isModelAllowed("claude-3", `["gpt-4","gpt-3.5-turbo"]`) {
		t.Error("model not in JSON array should not match")
	}
}

func TestIsModelAllowed_CommaSeparated_Match(t *testing.T) {
	if !isModelAllowed("gpt-4", "gpt-4, gpt-3.5-turbo") {
		t.Error("comma-separated whitelist should match")
	}
}

func TestIsModelAllowed_CommaSeparated_NoMatch(t *testing.T) {
	if isModelAllowed("claude-3", "gpt-4, gpt-3.5-turbo") {
		t.Error("model not in comma-separated whitelist should not match")
	}
}

func TestIsModelAllowed_EmptyList(t *testing.T) {
	if isModelAllowed("gpt-4", "[]") {
		t.Error("empty JSON array should not match any model")
	}
}

func TestIsModelAllowed_SingleModel(t *testing.T) {
	if !isModelAllowed("gpt-4", `["gpt-4"]`) {
		t.Error("single model in whitelist should match")
	}
}

func TestIsModelAllowed_CaseSensitive(t *testing.T) {
	if isModelAllowed("GPT-4", `["gpt-4"]`) {
		t.Error("model matching should be case-sensitive")
	}
}

// ── checkAPIKeyLimits ────────────────────────────────────────────────────

// mockRateLimiter always allows.
type mockAllowLimiter struct{}

func (m mockAllowLimiter) Allow(ctx context.Context, key string, rpm, tpm int) (bool, int, int, error) {
	return true, rpm, tpm, nil
}

// mockDenyLimiter always denies.
type mockDenyLimiter struct{}

func (m mockDenyLimiter) Allow(ctx context.Context, key string, rpm, tpm int) (bool, int, int, error) {
	return false, 0, 0, nil
}

func setupGinTest(body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestCheckAPIKeyLimits_ModelNotAllowed(t *testing.T) {
	key := &database.APIKey{
		ID:             1,
		ModelWhitelist: `["gpt-4"]`,
	}
	c, w := setupGinTest(`{"model":"claude-3","messages":[]}`)
	if checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("should reject model not in whitelist")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCheckAPIKeyLimits_ModelAllowed(t *testing.T) {
	key := &database.APIKey{
		ID:             1,
		ModelWhitelist: `["gpt-4"]`,
	}
	c, w := setupGinTest(`{"model":"gpt-4","messages":[]}`)
	if !checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("should allow model in whitelist")
	}
	// Body should be restored
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCheckAPIKeyLimits_WildcardWhitelist(t *testing.T) {
	key := &database.APIKey{
		ID:             1,
		ModelWhitelist: "*",
		RateLimitRPM:   10,
	}
	c, w := setupGinTest(`{"model":"any-model","messages":[]}`)
	if !checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("wildcard whitelist should allow any model")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCheckAPIKeyLimits_EmptyWhitelist(t *testing.T) {
	key := &database.APIKey{
		ID:             1,
		ModelWhitelist: "", // empty = wildcard
		RateLimitRPM:   10,
	}
	c, w := setupGinTest(`{"model":"any-model","messages":[]}`)
	if !checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("empty whitelist should allow any model")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCheckAPIKeyLimits_RateLimitExceeded(t *testing.T) {
	key := &database.APIKey{
		ID:           1,
		RateLimitRPM: 60,
	}
	c, w := setupGinTest(`{"model":"gpt-4"}`)
	if checkAPIKeyLimits(c, key, mockDenyLimiter{}, "h") {
		t.Error("should reject when rate limit exceeded")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestCheckAPIKeyLimits_RateLimitHeaders(t *testing.T) {
	key := &database.APIKey{
		ID:           1,
		RateLimitRPM: 60,
		RateLimitTPM: 100000,
	}
	c, w := setupGinTest(`{"model":"gpt-4"}`)
	if !checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("should allow when within rate limit")
	}
	if w.Header().Get("X-RateLimit-RPM-Remaining") == "" {
		t.Error("missing X-RateLimit-RPM-Remaining header")
	}
	if w.Header().Get("X-RateLimit-TPM-Remaining") == "" {
		t.Error("missing X-RateLimit-TPM-Remaining header")
	}
}

func TestCheckAPIKeyLimits_NoRateLimit(t *testing.T) {
	// When both RPM and TPM are 0, the rate limit check is skipped entirely.
	key := &database.APIKey{ID: 1}
	c, w := setupGinTest(`{"model":"gpt-4"}`)
	if !checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("should pass when no rate limits set")
	}
	if w.Header().Get("X-RateLimit-RPM-Remaining") != "" {
		t.Error("should not set rate limit headers when limits are 0")
	}
}

func TestCheckAPIKeyLimits_BudgetExhausted(t *testing.T) {
	key := &database.APIKey{
		ID:        1,
		BudgetUSD: 100,
		UsedUSD:   100, // exactly at limit — BudgetChecker rejects ">="
	}
	c, w := setupGinTest(`{"model":"gpt-4"}`)
	if checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("should reject when budget exhausted")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCheckAPIKeyLimits_BudgetOK(t *testing.T) {
	key := &database.APIKey{
		ID:        1,
		BudgetUSD: 100,
		UsedUSD:   50,
	}
	c, w := setupGinTest(`{"model":"gpt-4"}`)
	if !checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("should allow when within budget")
	}
	header := w.Header().Get("X-Budget-Remaining")
	if header == "" {
		t.Error("missing X-Budget-Remaining header")
	}
}

func TestCheckAPIKeyLimits_UnlimitedBudget(t *testing.T) {
	key := &database.APIKey{
		ID:        1,
		BudgetUSD: 0, // unlimited — budget check skipped entirely
		UsedUSD:   999999,
	}
	c, _ := setupGinTest(`{"model":"gpt-4"}`)
	if !checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("unlimited budget should always pass")
	}
	// Budget header is NOT set when BudgetUSD == 0 (check is skipped).
}

func TestCheckAPIKeyLimits_AllPassWithHeaders(t *testing.T) {
	key := &database.APIKey{
		ID:             1,
		ModelWhitelist: `["gpt-4"]`,
		BudgetUSD:      100,
		UsedUSD:        25,
		RateLimitRPM:   60,
		RateLimitTPM:   100000,
	}
	c, w := setupGinTest(`{"model":"gpt-4","messages":[]}`)
	if !checkAPIKeyLimits(c, key, mockAllowLimiter{}, "h") {
		t.Error("should pass when all checks pass")
	}
	if w.Header().Get("X-RateLimit-RPM-Remaining") == "" {
		t.Error("missing RPM header")
	}
	if w.Header().Get("X-RateLimit-TPM-Remaining") == "" {
		t.Error("missing TPM header")
	}
	if w.Header().Get("X-Budget-Remaining") == "" {
		t.Error("missing budget header")
	}
}

// ── extractBearerToken ───────────────────────────────────────────────────────

func TestExtractBearerToken_AcceptsQueryParamKey(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent?key=llmproxy_test_key", nil)
	if got := extractBearerToken(c); got != "llmproxy_test_key" {
		t.Errorf("expected query key 'llmproxy_test_key', got %q", got)
	}
}

func TestExtractBearerToken_BearerOverridesQueryKey(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent?key=query-key", nil)
	c.Request.Header.Set("Authorization", "Bearer bearer-key")
	if got := extractBearerToken(c); got != "bearer-key" {
		t.Errorf("expected bearer token 'bearer-key', got %q", got)
	}
}

func TestExtractBearerToken_AcceptsXGoogApiKeyHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", nil)
	c.Request.Header.Set("x-goog-api-key", "llmproxy_test_key")
	if got := extractBearerToken(c); got != "llmproxy_test_key" {
		t.Errorf("expected x-goog-api-key 'llmproxy_test_key', got %q", got)
	}
}

func TestExtractBearerToken_BearerOverridesXGoogApiKey(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", nil)
	c.Request.Header.Set("Authorization", "Bearer bearer-key")
	c.Request.Header.Set("x-goog-api-key", "goog-key")
	if got := extractBearerToken(c); got != "bearer-key" {
		t.Errorf("expected bearer token 'bearer-key', got %q", got)
	}
}
