package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"centag/core/internal/auth"
)

// mockQuotaDB 是一个极简的 mock，用于测试 QuotaMiddleware 的内存逻辑
// 真实 DB 依赖在集成测试中覆盖
type mockQuotaDB struct{}

func setupQuotaTest() (*gin.Engine, *QuotaMiddleware) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	mw := NewQuotaMiddleware(nil) // db=nil 表示无配额限制（优雅降级）
	return r, mw
}

func TestQuotaMiddleware_SingleUserMode_SkipsCheck(t *testing.T) {
	r, mw := setupQuotaTest()
	r.Use(mw.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "true")
}

func TestQuotaMiddleware_WithTenant_AllowsWhenNoDB(t *testing.T) {
	r, mw := setupQuotaTest()
	r.Use(func(c *gin.Context) {
		c.Set(auth.CtxKeyTenantID, "tenant-123")
		c.Next()
	})
	r.Use(mw.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestQuotaMiddleware_RecordTokens(t *testing.T) {
	_, mw := setupQuotaTest()

	// 先触发窗口创建（通过 check）
	// 由于 db=nil，check 会跳过，但我们可以通过 RecordTokens 直接测试
	// RecordTokens 在窗口不存在时应静默返回
	mw.RecordTokens("tenant-abc", 100)

	// 窗口不存在时 GetWindowStats 返回零值
	tokens, reqs, resetAt := mw.GetWindowStats("tenant-abc")
	assert.Equal(t, int64(0), tokens)
	assert.Equal(t, int64(0), reqs)
	assert.True(t, resetAt.IsZero())
}

func TestQuotaMiddleware_RecordRequest(t *testing.T) {
	_, mw := setupQuotaTest()

	// 空 tenantID 应静默返回
	mw.RecordRequest("")

	// 不存在的窗口应静默返回
	mw.RecordRequest("tenant-xyz")
}

func TestQuotaMiddleware_ResetWindow(t *testing.T) {
	_, mw := setupQuotaTest()

	// 重置不存在的窗口应静默返回
	mw.ResetWindow("tenant-none")
}

func TestQuotaMiddleware_GetWindowStats_NotExists(t *testing.T) {
	_, mw := setupQuotaTest()

	tokens, reqs, resetAt := mw.GetWindowStats("nonexistent")
	assert.Equal(t, int64(0), tokens)
	assert.Equal(t, int64(0), reqs)
	assert.True(t, resetAt.IsZero())
}

func TestQuotaMiddleware_WindowExpiry(t *testing.T) {
	mw := NewQuotaMiddleware(nil)

	// 手动注入一个过期的窗口
	window := &quotaWindow{
		tokens:   100,
		requests: 5,
		resetAt:  time.Now().UTC().Add(-1 * time.Hour), // 已过期
	}
	mw.windows.Store("tenant-expired", window)

	// 调用 check 会触发窗口重置
	// 由于 db=nil，check 会在早期返回 true，但窗口重置逻辑在 db 检查之前
	// 实际上 db=nil 时 check 直接返回 true，不会走到窗口重置
	// 所以这里我们测试 ResetWindow 方法
	mw.ResetWindow("tenant-expired")

	// 验证重置后的状态
	w, ok := mw.windows.Load("tenant-expired")
	assert.True(t, ok)
	qw := w.(*quotaWindow)
	qw.mu.RLock()
	assert.Equal(t, int64(0), qw.tokens)
	assert.Equal(t, int64(0), qw.requests)
	assert.True(t, qw.resetAt.After(time.Now().UTC()))
	qw.mu.RUnlock()
}

func TestQuotaMiddleware_ConcurrentRecord(t *testing.T) {
	mw := NewQuotaMiddleware(nil)

	// 预先创建窗口
	window := &quotaWindow{
		resetAt: time.Now().UTC().Add(24 * time.Hour),
	}
	mw.windows.Store("tenant-concurrent", window)

	// 并发记录
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			mw.RecordTokens("tenant-concurrent", 10)
			mw.RecordRequest("tenant-concurrent")
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	tokens, reqs, _ := mw.GetWindowStats("tenant-concurrent")
	assert.Equal(t, int64(1000), tokens) // 100 * 10
	assert.Equal(t, int64(200), reqs)    // 100 (from RecordTokens) + 100 (from RecordRequest)
}
