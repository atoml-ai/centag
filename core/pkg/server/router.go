package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"centag/core/internal"
	"centag/core/internal/auth"
	"centag/core/internal/cache"
	"centag/core/internal/monitor"
	"centag/core/internal/stats"
	"centag/core/pkg/circuitbreaker"
	"centag/core/pkg/database"
	"centag/core/pkg/editionmodule"
	"centag/core/pkg/hooks"
	"github.com/gin-gonic/gin"
)

// healthCheck 存活探针（liveness）— 进程可响应即返回 200，不探测依赖。
func (s *Server) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":  "ok",
		"service": "centag",
		"edition": s.edition.String(),
	}

	c.JSON(http.StatusOK, status)
}

// healthReady 就绪探针（readiness）— 验证数据库等核心依赖可用。
func (s *Server) healthReady(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if !database.IsInitialized() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not_ready",
			"service": "centag",
			"edition": s.edition.String(),
			"checks": gin.H{
				"database": gin.H{"status": "fail", "error": "not initialized"},
			},
		})
		return
	}

	db := database.Get()
	driver := db.DriverName()
	if err := db.HealthCheck(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not_ready",
			"service": "centag",
			"edition": s.edition.String(),
			"checks": gin.H{
				"database": gin.H{"status": "fail", "driver": driver, "error": err.Error()},
			},
		})
		return
	}

	// P0-T2：健康就绪聚合熔断状态——探测 /models 通过但 chat 链路熔断打开时，
	// 服务仍可用（其余后端可服务），标记 degraded 让管理员可感知。
	openBreakers := openCircuitBreakers()
	statusCode := http.StatusOK
	overall := "ready"
	if len(openBreakers) > 0 {
		overall = "degraded"
	}

	c.JSON(statusCode, gin.H{
		"status":  overall,
		"service": "centag",
		"edition": s.edition.String(),
		"checks": gin.H{
			"database":        gin.H{"status": "ok", "driver": driver},
			"circuit_breaker": gin.H{"status": boolToStr(len(openBreakers) == 0, "ok", "degraded"), "open_backends": openBreakers},
		},
	})
}

// openCircuitBreakers 返回当前处于 open 状态的后端 ID 列表。
func openCircuitBreakers() []string {
	states := circuitbreaker.GetAllStates()
	var open []string
	for id, st := range states {
		if st == "open" {
			open = append(open, id)
		}
	}
	sort.Strings(open)
	return open
}

func boolToStr(cond bool, t, f string) string {
	if cond {
		return t
	}
	return f
}

// ping Ping 测试
func (s *Server) ping(c *gin.Context) {
	c.String(200, "pong")
}

// getStats 获取统计信息
func (s *Server) getStats(c *gin.Context) {
	// 获取旧版monitor统计
	mon := monitor.GetMonitor()
	monitorStats := mon.GetStats()

	// 获取统一统计
	unifiedStats := stats.GlobalUnifiedStats.GetStats()

	c.JSON(200, gin.H{
		// 旧版统计(向后兼容)
		"total_requests":  monitorStats.TotalRequests,
		"cache_hit_rate":  mon.GetCacheHitRate(),
		"avg_latency":     mon.GetAvgLatency(),

		// 新版统一统计
		"unified_stats": unifiedStats,
	})
}

// getCacheStats 获取详细缓存统计
func (s *Server) getCacheStats(c *gin.Context) {
	// 获取统一统计
	unifiedStats := stats.GlobalUnifiedStats.GetStats()

	// 获取缓存管理器统计
	var cacheStats *cache.CacheStats
	if s.proxyCache != nil {
		cacheStats = s.proxyCache.GetStats()
	}

	c.JSON(200, gin.H{
		"unified": unifiedStats,
		"cache":   cacheStats,
	})
}

// listPlugins 列出插件
func (s *Server) listPlugins(c *gin.Context) {
	plugins := s.pluginManager.List()
	c.JSON(200, gin.H{
		"plugins": plugins,
	})
}

// getPlugin 获取插件信息
func (s *Server) getPlugin(c *gin.Context) {
	name := c.Param("name")
	plugin, err := s.pluginManager.Get(name)
	if err != nil {
		c.JSON(404, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"name":    plugin.Name(),
		"type":    plugin.Type(),
		"version": plugin.Version(),
		"status":  plugin.Status(),
	})
}

// updatePlugin 更新插件配置
func (s *Server) updatePlugin(c *gin.Context) {
	name := c.Param("name")
	c.JSON(200, gin.H{
		"message": "Plugin configuration update not implemented yet",
		"name":    name,
	})
}

// formatUptime 将运行时长格式化为可读短串（秒级截断，避免 Go Duration 小数）。
func formatUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int64(d / time.Second)
	days := total / 86400
	hours := (total % 86400) / 3600
	mins := (total % 3600) / 60
	secs := total % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh%dm", days, hours, mins)
	case hours > 0:
		return fmt.Sprintf("%dh%dm%ds", hours, mins, secs)
	case mins > 0:
		return fmt.Sprintf("%dm%ds", mins, secs)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}

// handleStatus 获取服务状态
func (s *Server) handleStatus(c *gin.Context) {
	resp := gin.H{
		"service":    "centag",
		"status":     "healthy",
		"edition":    s.edition.String(),
		"version":    internal.GetVersion(),
		"build_time": internal.GetBuildTime(),
		"start_time": s.startTime.Format("2006-01-02 15:04:05"),
		"uptime":     formatUptime(time.Since(s.startTime)),
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	if s.cfg != nil {
		resp["port"] = s.cfg.Server.Port
		resp["host"] = s.cfg.Server.Host
	}

	// 对外访问地址（由环境变量 LLM_PROXY_EXTERNAL_URL 配置）
	if s.cfg != nil && s.cfg.Server.ExternalURL != "" {
		resp["external_url"] = s.cfg.Server.ExternalURL
	}

	if caps := editionmodule.EnrichCapabilities(nil); len(caps) > 0 {
		resp["capabilities"] = caps
	}

	// 钩子失败计数（fail-open 累计；非零说明存在计量/计费落库异常，应排查日志）
	if tokenUsed, billingUsage, quotaExceeded := hooks.HookFailureCounts(); tokenUsed+billingUsage+quotaExceeded > 0 {
		resp["hook_failures"] = gin.H{
			"token_used":     tokenUsed,
			"billing_usage":  billingUsage,
			"quota_exceeded": quotaExceeded,
		}
	}

	// API Key 存储健康自检结果（启动时扫描；undecryptable>0 说明历史密钥无法用当前 STORAGE_SECRET 解密）
	if checked, bad := auth.LastKeyAudit(); checked > 0 {
		resp["api_key_storage"] = gin.H{
			"encrypted_keys": checked,
			"undecryptable":  bad,
		}
	}

	c.JSON(200, resp)
}

