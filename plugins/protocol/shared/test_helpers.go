package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

// MockBackend 模拟后端，捕获请求并返回预设响应
type MockBackend struct {
	// 捕获的请求
	CapturedRequest *plugin.ProxyRequest
	CapturedRawBody map[string]interface{}

	// 预设的响应
	Response    *plugin.ProxyResponse
	StreamChunks []*plugin.StreamChunk
	Error       error

	// 调用计数
	CallCount int

	// 验证函数
	ValidateRequest func(req *plugin.ProxyRequest) error
}

// Chat 模拟非流式请求
func (m *MockBackend) Chat(ctx context.Context, req *plugin.ProxyRequest) (*plugin.ProxyResponse, error) {
	m.CapturedRequest = req
	if rawBody, ok := req.RawBody.(map[string]interface{}); ok {
		m.CapturedRawBody = rawBody
	}
	m.CallCount++

	if m.ValidateRequest != nil {
		if err := m.ValidateRequest(req); err != nil {
			return nil, err
		}
	}

	return m.Response, m.Error
}

// ChatStream 模拟流式请求
func (m *MockBackend) ChatStream(ctx context.Context, req *plugin.ProxyRequest) (<-chan *plugin.StreamChunk, error) {
	m.CapturedRequest = req
	if rawBody, ok := req.RawBody.(map[string]interface{}); ok {
		m.CapturedRawBody = rawBody
	}
	m.CallCount++

	if m.ValidateRequest != nil {
		if err := m.ValidateRequest(req); err != nil {
			return nil, err
		}
	}

	if m.Error != nil {
		return nil, m.Error
	}

	ch := make(chan *plugin.StreamChunk, len(m.StreamChunks))
	for _, chunk := range m.StreamChunks {
		ch <- chunk
	}
	close(ch)
	return ch, nil
}

// TestCase 测试用例定义
type TestCase struct {
	Name           string
	RequestJSON    string
	ExpectedFields map[string]interface{}
	ValidateReq    func(t *testing.T, req *plugin.ProxyRequest)
	ValidateResp   func(t *testing.T, resp map[string]interface{})
	MockResponse   *plugin.ProxyResponse
	MockBackend    *MockBackend
	MockError      error
	ExpectedStatus int
}

// ProtocolTestRunner 协议测试运行器
type ProtocolTestRunner struct {
	T             *testing.T
	Protocol      plugin.ProtocolPlugin
	MockBackend   *MockBackend
}

// NewProtocolTestRunner 创建协议测试运行器
func NewProtocolTestRunner(t *testing.T, protocol plugin.ProtocolPlugin) *ProtocolTestRunner {
	return &ProtocolTestRunner{
		T:           t,
		Protocol:    protocol,
		MockBackend: &MockBackend{},
	}
}

// RunRequestResponseTest 运行请求-响应往返测试
func (r *ProtocolTestRunner) RunRequestResponseTest(tc TestCase) {
	r.T.Run(tc.Name, func(t *testing.T) {
		// 1. 创建 HTTP 请求
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tc.RequestJSON))
		c.Request.Header.Set("Content-Type", "application/json")

		// 2. ParseRequest
		proxyReq, err := r.Protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("ParseRequest failed: %v", err)
		}

		// 3. 验证请求字段
		if tc.ValidateReq != nil {
			tc.ValidateReq(t, proxyReq)
		}

		// 4. Mock 后端处理
		r.MockBackend.Response = tc.MockResponse
		r.MockBackend.Error = tc.MockError
		proxyResp, err := r.MockBackend.Chat(context.Background(), proxyReq)
		if err != nil && tc.ExpectedStatus == 200 {
			t.Fatalf("Mock backend failed: %v", err)
		}

		// 5. HandleResponse
		if proxyResp != nil {
			w2 := httptest.NewRecorder()
			c2, _ := gin.CreateTestContext(w2)
			err = r.Protocol.HandleResponse(c2, proxyResp)
			if err != nil {
				t.Fatalf("HandleResponse failed: %v", err)
			}

			// 6. 验证响应
			var respBody map[string]interface{}
			if err := json.Unmarshal(w2.Body.Bytes(), &respBody); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if tc.ValidateResp != nil {
				tc.ValidateResp(t, respBody)
			}
		}
	})
}

// RunStreamTest 运行流式测试
func (r *ProtocolTestRunner) RunStreamTest(tc TestCase) {
	r.T.Run(tc.Name, func(t *testing.T) {
		// 1. 创建 HTTP 请求
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tc.RequestJSON))
		c.Request.Header.Set("Content-Type", "application/json")

		// 2. ParseRequest
		proxyReq, err := r.Protocol.ParseRequest(c)
		if err != nil {
			t.Fatalf("ParseRequest failed: %v", err)
		}

		// 3. 验证请求字段
		if tc.ValidateReq != nil {
			tc.ValidateReq(t, proxyReq)
		}

		// 4. Mock 后端流式处理
		r.MockBackend.StreamChunks = tc.MockBackend.StreamChunks
		r.MockBackend.Error = tc.MockError
		streamCh, err := r.MockBackend.ChatStream(context.Background(), proxyReq)
		if err != nil {
			t.Fatalf("Mock backend stream failed: %v", err)
		}

		// 5. 收集流式响应
		var chunks []string
		for chunk := range streamCh {
			result := r.Protocol.FormatStreamChunk(proxyReq.Model, chunk, len(chunks))
			if result != "" {
				chunks = append(chunks, result)
			}
		}

		// 6. 添加结束标记
		done := r.Protocol.FormatStreamDone()
		if done != "" {
			chunks = append(chunks, done)
		}

		// 7. 验证流式响应
		if tc.ValidateResp != nil {
			respData := map[string]interface{}{
				"chunks": chunks,
				"count":  len(chunks),
			}
			tc.ValidateResp(t, respData)
		}
	})
}

// AssertField 断言字段值
func AssertField(t *testing.T, data map[string]interface{}, key string, expected interface{}) {
	t.Helper()
	actual, ok := data[key]
	if !ok {
		t.Errorf("field %q not found", key)
		return
	}
	if fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected) {
		t.Errorf("field %q: got %v, want %v", key, actual, expected)
	}
}

// AssertFieldExists 断言字段存在
func AssertFieldExists(t *testing.T, data map[string]interface{}, key string) {
	t.Helper()
	if _, ok := data[key]; !ok {
		t.Errorf("field %q not found", key)
	}
}

// AssertFieldNotExists 断言字段不存在
func AssertFieldNotExists(t *testing.T, data map[string]interface{}, key string) {
	t.Helper()
	if _, ok := data[key]; ok {
		t.Errorf("field %q should not exist", key)
	}
}

// AssertArrayNotEmpty 断言数组非空
func AssertArrayNotEmpty(t *testing.T, data []interface{}, name string) {
	t.Helper()
	if len(data) == 0 {
		t.Errorf("%s should not be empty", name)
	}
}

// AssertStringContains 断言字符串包含
func AssertStringContains(t *testing.T, actual, expected, field string) {
	t.Helper()
	if !strings.Contains(actual, expected) {
		t.Errorf("%s: got %q, want contains %q", field, actual, expected)
	}
}

// ParseJSONResponse 解析 JSON 响应
func ParseJSONResponse(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	return result
}

// CreateTestGinContext 创建测试用 Gin 上下文
func CreateTestGinContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// ReadBody 读取请求体
func ReadBody(t *testing.T, body io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}
	return data
}

// ContainsAll 判断字符串是否包含所有子串
func ContainsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// ContainsAny 判断字符串是否包含任一子串
func ContainsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
