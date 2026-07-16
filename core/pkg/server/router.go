package server

import (
	"context"
	"net/http"
	"time"

	"centag/core/internal"
	"centag/core/internal/cache"
	"centag/core/pkg/database"
	"centag/core/internal/monitor"
	"centag/core/internal/stats"

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

	c.JSON(http.StatusOK, gin.H{
		"status":  "ready",
		"service": "centag",
		"edition": s.edition.String(),
		"checks": gin.H{
			"database": gin.H{"status": "ok", "driver": driver},
		},
	})
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

// handleSystemUpdate 处理系统更新
func (s *Server) handleSystemUpdate(c *gin.Context) {
	s.systemUpdate.HandleUpdate(c.Writer, c.Request)
}

// handleUpdateHistory 获取更新历史
func (s *Server) handleUpdateHistory(c *gin.Context) {
	s.systemUpdate.HandleUpdateHistory(c.Writer, c.Request)
}

// handleRollbackUpdate 回退到指定版本
func (s *Server) handleRollbackUpdate(c *gin.Context) {
	s.systemUpdate.HandleRollback(c.Writer, c.Request)
}

// handleDeleteUpdatePackage 删除更新包
func (s *Server) handleDeleteUpdatePackage(c *gin.Context) {
	s.systemUpdate.HandleDelete(c.Writer, c.Request)
}

// handleStatus 获取服务状态
func (s *Server) handleStatus(c *gin.Context) {
	uptime := time.Since(s.startTime).String()

	resp := gin.H{
		"service":    "centag",
		"status":     "healthy",
		"edition":    s.edition.String(),
		"version":    internal.GetVersion(),
		"build_time": internal.GetBuildTime(),
		"start_time": s.startTime.Format("2006-01-02 15:04:05"),
		"uptime":     uptime,
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

	c.JSON(200, resp)
}

