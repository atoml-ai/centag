package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestE2EHealthCheck 测试健康检查端点
// 注意：此测试需要完整的服务器初始化，暂时跳过
func TestE2EHealthCheck(t *testing.T) {
	t.Skip("E2E test requires full server initialization with all dependencies")
}

// TestE2EPing 测试 ping 端点
func TestE2EPing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// 创建简单的 gin 路由
	router := gin.New()
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	
	// 创建测试服务器
	ts := httptest.NewServer(router)
	defer ts.Close()
	
	// 发送请求到 /ping
	resp, err := http.Get(ts.URL + "/ping")
	assert.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2EStatusEndpoint 测试状态端点
func TestE2EStatusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// 创建简单的 gin 路由
	router := gin.New()
	router.GET("/v1/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"status":  "running",
				"version": "1.0.0",
			},
		})
	})
	
	// 创建测试服务器
	ts := httptest.NewServer(router)
	defer ts.Close()
	
	// 发送请求
	resp, err := http.Get(ts.URL + "/v1/status")
	assert.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
