package server

import (
	"fmt"
	"os"

	"centag/core/pkg/backend"
	"centag/core/internal/cache"
	"centag/core/pkg/database"
	"centag/core/pkg/metrics"
	"centag/core/pkg/plugin"
	"centag/core/internal/stats"
	"centag/core/pkg/storage"

	"github.com/gin-gonic/gin"
)

// MetricsHandler 监控指标处理器
type MetricsHandler struct {
	pluginManager  *plugin.Manager
	cacheManager   *cache.Manager
	backendManager *backend.Manager
	storageManager *storage.Manager
}

// NewMetricsHandler 创建监控处理器
func NewMetricsHandler(pluginManager *plugin.Manager, cacheManager *cache.Manager, backendManager *backend.Manager, storageManager *storage.Manager) *MetricsHandler {
	return &MetricsHandler{
		pluginManager:  pluginManager,
		cacheManager:   cacheManager,
		backendManager: backendManager,
		storageManager: storageManager,
	}
}

// GetRequestStats 获取请求统计
func (h *MetricsHandler) GetRequestStats(c *gin.Context) {
	if metrics.GlobalMetrics == nil {
		c.JSON(500, gin.H{"error": "Metrics not initialized"})
		return
	}

	stats := metrics.GlobalMetrics.GetStats()
	c.JSON(200, stats)
}

// GetRouteBackendStats 获取 selected_route + backend_id 维度统计
func (h *MetricsHandler) GetRouteBackendStats(c *gin.Context) {
	if metrics.GlobalRouteBackendMetrics == nil {
		c.JSON(500, gin.H{"error": "Route-backend metrics not initialized"})
		return
	}
	c.JSON(200, gin.H{
		"items": metrics.GlobalRouteBackendMetrics.GetStats(),
	})
}

// GetCacheStats 获取缓存统计
func (h *MetricsHandler) GetCacheStats(c *gin.Context) {
	if stats.GlobalUnifiedStats == nil {
		c.JSON(500, gin.H{"error": "Unified stats not initialized"})
		return
	}

	uniStats := stats.GlobalUnifiedStats.GetStats()
	c.JSON(200, uniStats)
}

// GetPluginStatus 获取插件状态
func (h *MetricsHandler) GetPluginStatus(c *gin.Context) {
	if h.pluginManager == nil {
		c.JSON(500, gin.H{"error": "Plugin manager not initialized"})
		return
	}

	plugins := h.pluginManager.List()
	result := make([]PluginStatus, 0, len(plugins))

	for _, p := range plugins {
		status := PluginStatus{
			Name:        p.Name,
			Type:        string(p.Type),
			Version:     p.Version,
			Enabled:     true,
			Status:      string(p.Status),
		}
		result = append(result, status)
	}

	c.JSON(200, result)
}

// GetDashboardStats 获取仪表板综合统计
func (h *MetricsHandler) GetDashboardStats(c *gin.Context) {
	dashboard := &DashboardStats{}

	// 请求统计 - 使用旧的 metrics 系统作为兼容层
	if metrics.GlobalMetrics != nil {
		reqStats := metrics.GlobalMetrics.GetStats()
		dashboard.Request = RequestStats{
			TotalRequests:  reqStats.TotalRequests,
			SuccessRequests: reqStats.SuccessRequests,
			ErrorRequests:   reqStats.ErrorRequests,
			QPS:            reqStats.QPS,
			AvgLatency:     reqStats.AvgLatency,
			ErrorRate:      reqStats.ErrorRate,
			Uptime:         reqStats.Uptime,
		}
		if reqStats.ModelStats != nil {
			dashboard.ModelStats = reqStats.ModelStats
		}
	}

	// 缓存统计 - 使用统一统计系统，但条目数从缓存管理器获取
	cacheEntries := int64(0)
	if h.cacheManager != nil {
		cacheManagerStats := h.cacheManager.Stats()
		cacheEntries = cacheManagerStats.TotalEntries
	}

	if stats.GlobalUnifiedStats != nil {
		dashboard.RecentRequests = stats.GlobalUnifiedStats.RecentRequests()
		uniStats := stats.GlobalUnifiedStats.GetStats()
		dashboard.Cache = CacheStatsSummary{
			Hits:         uniStats.HitExact + uniStats.HitSemantic,
			Misses:       uniStats.Miss,
			HitRate:      uniStats.HitRate,
			Entries:      cacheEntries, // 从缓存管理器获取
			Evictions:    0, // 暂不支持淘汰统计
			Uptime:       uniStats.Uptime,
			TotalRequests: uniStats.TotalRequests,
		}
	}

	// 插件状态
	if h.pluginManager != nil {
		plugins := h.pluginManager.List()
		dashboard.PluginCount = len(plugins)

		runningCount := 0
		for _, p := range plugins {
			if p.Status == plugin.StatusRunning {
				runningCount++
			}
		}
		dashboard.PluginRunning = runningCount
	}

	// 数据库信息
	if database.IsInitialized() {
		dbManager := database.Get()
		dbInfo := DatabaseInfo{
			Driver: dbManager.DriverName(),
			Status: "connected",
		}
		
		// 获取数据库地址信息
		dbInfo.Address = getDatabaseAddress(dbManager.DriverName())
		
		dashboard.Database = dbInfo
	} else {
		dashboard.Database = DatabaseInfo{
			Driver: "unknown",
			Status: "disconnected",
		}
	}

	c.JSON(200, dashboard)
}

// getDatabaseAddress 获取数据库连接地址信息
func getDatabaseAddress(driverName string) string {
	// 由于Go的sql.DB没有直接的方法获取连接字符串，我们从环境变量获取配置
	switch driverName {
	case "postgresql":
		host := getEnvOrDefault("PG_HOST", getEnvOrDefault("POSTGRES_HOST", ""))
		port := getEnvOrDefault("PG_PORT", getEnvOrDefault("POSTGRES_PORT", "5432"))
		dbname := getEnvOrDefault("PG_DATABASE", getEnvOrDefault("POSTGRES_DB", ""))
		if host != "" {
			return fmt.Sprintf("%s:%s/%s", host, port, dbname)
		}
		return "postgresql://unknown"
	case "sqlite":
		path := getEnvOrDefault("SQLITE_PATH", "./storage/centag.db")
		return path
	default:
		return fmt.Sprintf("%s://unknown", driverName)
	}
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetConfigInfo 获取配置信息
func (h *MetricsHandler) GetConfigInfo(c *gin.Context) {
	config := gin.H{}

	// 获取后端配置
	if h.backendManager != nil {
		backends := h.backendManager.List()
		enabledBackends := make([]gin.H, 0)
		for _, b := range backends {
			if b.Enabled {
				enabledBackends = append(enabledBackends, gin.H{
					"id":      b.ID,
					"name":    b.Name,
					"type":    b.Type,
					"enabled": b.Enabled,
					"weight":  b.Weight,
				})
			}
		}
		config["backends"] = enabledBackends
	}

	// 获取存储配置
	if h.storageManager != nil {
		storages := h.storageManager.ListStorages()
		enabledStorages := make([]gin.H, 0)
		for _, s := range storages {
			if s.Enabled {
				enabledStorages = append(enabledStorages, gin.H{
					"name":   s.Name,
					"type":   s.Type,
					"default": s.Name == h.storageManager.GetDefaultKVName(),
				})
			}
		}
		config["storages"] = enabledStorages
		config["default_kv"] = h.storageManager.GetDefaultKVName()
	}

	c.JSON(200, config)
}

// ResetStats 重置统计
func (h *MetricsHandler) ResetStats(c *gin.Context) {
	if metrics.GlobalMetrics != nil {
		metrics.GlobalMetrics.Reset()
	}
	if metrics.GlobalRouteBackendMetrics != nil {
		metrics.GlobalRouteBackendMetrics.Reset()
	}

	if h.cacheManager != nil {
		h.cacheManager.ResetStats()
	}

	c.JSON(200, gin.H{"message": "Stats reset successfully"})
}

// DashboardStats 仪表板统计
type DashboardStats struct {
	Request     RequestStats        `json:"request"`
	Cache       CacheStatsSummary   `json:"cache"`
	PluginCount int                 `json:"plugin_count"`
	PluginRunning int               `json:"plugin_running"`
	ModelStats  map[string]*metrics.ModelStatsSnapshot `json:"model_stats"`
	Database    DatabaseInfo        `json:"database"`
	// RecentRequests 窗口内最近请求（P1-11：Team Overview「实时请求」卡片数据源）
	RecentRequests []stats.RequestRecord `json:"recent_requests,omitempty"`
}

// RequestStats 请求统计摘要
type RequestStats struct {
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests  int64   `json:"success_requests"`
	ErrorRequests    int64   `json:"error_requests"`
	QPS              float64 `json:"qps"`
	AvgLatency       int64   `json:"avg_latency_ms"`
	ErrorRate        float64 `json:"error_rate_percent"`
	Uptime           int64   `json:"uptime_ms"`
}

// CacheStatsSummary 缓存统计摘要
type CacheStatsSummary struct {
	Hits         int64   `json:"hits"`
	Misses       int64   `json:"misses"`
	HitRate      float64 `json:"hit_rate_percent"`
	Entries      int64   `json:"entries"`
	Evictions    int64   `json:"evictions"`
	Uptime       int64   `json:"uptime_ms"`
	TotalRequests int64  `json:"total_requests"`
}

// DatabaseInfo 数据库信息
type DatabaseInfo struct {
	Driver   string `json:"driver"`
	Status   string `json:"status"`
	Address  string `json:"address,omitempty"`  // 数据库地址信息
}

// PluginStatus 插件状态
type PluginStatus struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}
