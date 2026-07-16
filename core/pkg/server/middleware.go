package server

import (
	"net/url"
	"os"
	"strings"
	"time"

	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// corsMiddleware CORS 中间件
func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := parseOriginList(os.Getenv("LLM_PROXY_CORS_ALLOW_ORIGINS"))
	if len(allowedOrigins) == 0 {
		// 向后兼容：默认保持历史行为（全放开）
		allowedOrigins = []string{"*"}
	}
	allowCredentials := parseBool("LLM_PROXY_CORS_ALLOW_CREDENTIALS", false)

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if shouldAllowOrigin(origin, allowedOrigins) {
				if allowCredentials || !containsWildcard(allowedOrigins) {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Vary", "Origin")
				} else {
					c.Header("Access-Control-Allow-Origin", "*")
				}
				if allowCredentials {
					c.Header("Access-Control-Allow-Credentials", "true")
				}
			}
		} else if containsWildcard(allowedOrigins) {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Requested-With, X-Proxy-Mode, X-Target-BaseURL, X-Target-Model")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func parseOriginList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parseBool(key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	return strings.EqualFold(raw, "1") || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes")
}

func containsWildcard(allowed []string) bool {
	for _, item := range allowed {
		if strings.TrimSpace(item) == "*" {
			return true
		}
	}
	return false
}

func shouldAllowOrigin(origin string, allowed []string) bool {
	origin = strings.TrimSpace(origin)
	host := origin
	if parsed, err := url.Parse(origin); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	host = strings.ToLower(host)

	for _, item := range allowed {
		rule := strings.TrimSpace(item)
		switch {
		case rule == "*":
			return true
		case strings.EqualFold(rule, origin):
			return true
		case strings.HasPrefix(rule, "*."):
			suffix := strings.ToLower(strings.TrimPrefix(rule, "*."))
			if strings.HasSuffix(host, "."+suffix) {
				return true
			}
		}
	}
	return false
}

// loggerMiddleware 访问日志中间件：记录每个请求的方法、路径、状态码和耗时
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestPath := c.Request.URL.Path
		path := requestPath
		raw := c.Request.URL.RawQuery
		clientIP := c.ClientIP()
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		if raw != "" {
			path = path + "?" + raw
		}
		// 高频轮询 / 管理接口不写访问日志，避免日志页「实时跟踪」刷屏。
		if isNoisyAccessPath(requestPath) {
			return
		}
		logger.Info("request",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.String("ip", clientIP),
			zap.Duration("latency", latency),
		)
	}
}

func isNoisyAccessPath(path string) bool {
	if strings.HasPrefix(path, "/static/") {
		return true
	}
	noisy := []string{
		"/api/v1/cache/stats",
		"/api/v1/cache/list",
		"/api/v1/logs",
		"/api/v1/logs/stats",
		"/api/v1/logs/stream",
		"/health",
		"/health/ready",
		"/api/v1/status",
		"/api/v1/backends",
		"/api/v1/backends/probe-all-sse",
		"/api/v1/storage",
		"/api/v1/plugins",
		"/api/v1/proxy/status",
		"/api/v1/host-proxy/status",
		"/api/v1/monitor/dashboard",
		"/api/v1/pipelines",
		"/api/v1/pipeline/defaults",
		"/api/auth/refresh",
	}
	for _, p := range noisy {
		if path == p {
			return true
		}
	}
	return false
}

// recoveryMiddleware 恢复中间件
func recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// logger.Errorf("Panic recovered: %v", err)
				c.JSON(500, gin.H{
					"error": "Internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
