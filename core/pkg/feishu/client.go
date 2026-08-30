// Package feishu provides a reusable Feishu Bitable (多维表格) HTTP client.
//
// It handles tenant_access_token lifecycle (acquire, cache, refresh),
// provides generic Bitable record/field operations, and is used by both
// the telemetry provider and the configsync reader/writer.
//
// External callers should create a Client via NewClient and use the
// exported methods for Bitable CRUD. All HTTP details are encapsulated.
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	defaultBaseURL     = "https://open.feishu.cn/open-apis"
	defaultTokenURL    = defaultBaseURL + "/auth/v3/tenant_access_token/internal"
	defaultBitableURL = defaultBaseURL + "/bitable/v1/apps"
	defaultTimeout    = 15 * time.Second
	// tokenRefreshMargin refreshes the token this many seconds before actual expiry.
	tokenRefreshMargin = 300 * time.Second
	// maxReadBody caps response body reads to avoid OOM on malformed responses.
	maxReadBody = 1 << 20 // 1 MiB
)

// Config holds Feishu Bitable credentials and connection settings.
// App Secret MUST come from runtime env, never from code or images.
type Config struct {
	AppID     string        // Feishu app ID
	AppSecret string        // Feishu app secret (sensitive)
	AppToken  string        // Bitable app token; empty = auto-create base on first use
	TableID   string        // Default table ID within the base
	BaseName  string        // Name used when auto-creating the base (if AppToken empty)
	Timeout   time.Duration // HTTP client timeout; defaults to 15s
}

// Client is a Feishu Bitable HTTP client with token caching.
//
// Safe for concurrent use. A single Client should be shared across
// goroutines for the same app credentials.
type Client struct {
	cfg   Config
	httpc *http.Client

	// tokenURL and bitableURL allow overriding for testing. When empty,
	// the production Feishu API URLs are used.
	tokenURL  string
	bitableURL string

	mu          sync.Mutex
	token       string
	tokenExpire time.Time
}

// NewClient creates a Feishu client. If cfg.Timeout is zero, 15s is used.
func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	return &Client{
		cfg:   cfg,
		httpc: &http.Client{Timeout: cfg.Timeout},
	}
}

// SetEndpoints overrides the token and bitable API base URLs.
// Intended for tests and self-hosted API proxies.
func (c *Client) SetEndpoints(tokenURL, bitableURL string) {
	c.tokenURL = tokenURL
	c.bitableURL = bitableURL
}

func (c *Client) tokenEndpoint() string {
	if c.tokenURL != "" {
		return c.tokenURL
	}
	return defaultTokenURL
}

func (c *Client) bitableEndpoint() string {
	if c.bitableURL != "" {
		return c.bitableURL
	}
	return defaultBitableURL
}

// TenantToken returns a valid tenant_access_token, acquiring or refreshing
// as needed. The returned token is valid for at least 5 minutes.
func (c *Client) TenantToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpire) {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{
		"app_id":     c.cfg.AppID,
		"app_secret": c.cfg.AppSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenEndpoint(), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("feishu token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("feishu token http: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("feishu token decode: %w", err)
	}
	if out.Code != 0 || out.TenantAccessToken == "" {
		return "", fmt.Errorf("feishu token error: code=%d msg=%s", out.Code, out.Msg)
	}
	c.token = out.TenantAccessToken
	// Refresh margin before actual expiry.
	expireSec := out.Expire
	if expireSec > int(tokenRefreshMargin.Seconds()) {
		expireSec -= int(tokenRefreshMargin.Seconds())
	}
	c.tokenExpire = time.Now().Add(time.Duration(expireSec) * time.Second)
	return c.token, nil
}

// DecodeCode checks a Feishu-style {"code":N,"msg":"..."} JSON response body
// and returns an error if code != 0. Used for write/create responses.
func DecodeCode(resp *http.Response, what string) error {
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxReadBody))
	if err != nil {
		return fmt.Errorf("%s: read body: %w", what, err)
	}
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if len(b) > 0 {
		_ = json.Unmarshal(b, &out)
	}
	if out.Code != 0 {
		return fmt.Errorf("%s: code=%d msg=%s", what, out.Code, out.Msg)
	}
	return nil
}

// httpGet performs an authenticated GET and returns the response body bytes.
// Caller must close resp.Body.
func (c *Client) httpGet(ctx context.Context, token, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.httpc.Do(req)
}

// httpPost performs an authenticated POST with a JSON body and returns the
// response. Caller must close resp.Body.
func (c *Client) httpPost(ctx context.Context, token, url string, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return c.httpc.Do(req)
}

// httpPut performs an authenticated PUT with a JSON body. Caller must close resp.Body.
func (c *Client) httpPut(ctx context.Context, token, url string, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return c.httpc.Do(req)
}
