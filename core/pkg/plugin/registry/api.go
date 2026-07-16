package registry

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 插件注册表 HTTP 处理器
type Handler struct {
	store Store
}

// NewHandler 创建处理器
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	registry := r.Group("/registry")
	{
		// 插件管理
		registry.POST("/plugins", h.RegisterPlugin)
		registry.GET("/plugins", h.ListPlugins)
		registry.GET("/plugins/:id", h.GetPlugin)
		registry.DELETE("/plugins/:id", h.DeletePlugin)

		// 版本管理
		registry.GET("/plugins/:id/versions", h.ListVersions)
		registry.GET("/plugins/:id/versions/:version", h.GetVersion)

		// 评分
		registry.POST("/plugins/:id/ratings", h.RatePlugin)
		registry.GET("/plugins/:id/ratings", h.GetPluginRating)

		// 下载
		registry.POST("/plugins/:id/download", h.DownloadPlugin)
	}
}

// RegisterPlugin 注册插件
func (h *Handler) RegisterPlugin(c *gin.Context) {
	var req RegisterPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 创建插件元数据
	plugin := &PluginMetadata{
		Name:         req.Name,
		Version:      req.Version,
		Description:  req.Description,
		Author:       req.Author,
		Email:        req.Email,
		URL:          req.URL,
		Category:     req.Category,
		Tags:         req.Tags,
		Permissions:  req.Permissions,
		Dependencies: req.Dependencies,
		DownloadURL:  req.DownloadURL,
		Checksum:     req.Checksum,
		Signature:    req.Signature,
		Size:         req.Size,
	}

	if err := h.store.Register(c.Request.Context(), plugin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, RegisterPluginResponse{
		ID:      plugin.ID,
		Message: "Plugin registered successfully",
	})
}

// ListPlugins 列出插件
func (h *Handler) ListPlugins(c *gin.Context) {
	req := ListPluginsRequest{
		Category:  c.Query("category"),
		Author:    c.Query("author"),
		Search:    c.Query("search"),
		SortBy:    c.DefaultQuery("sort_by", "name"),
		SortOrder: c.DefaultQuery("sort_order", "asc"),
	}

	// 解析标签
	if tags := c.QueryArray("tags"); len(tags) > 0 {
		req.Tags = tags
	}

	// 解析分页
	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && page > 0 {
		req.Page = page
	} else {
		req.Page = 1
	}

	if pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil && pageSize > 0 {
		req.PageSize = pageSize
	} else {
		req.PageSize = 20
	}

	resp, err := h.store.List(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetPlugin 获取插件详情
func (h *Handler) GetPlugin(c *gin.Context) {
	id := c.Param("id")

	plugin, err := h.store.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plugin)
}

// DeletePlugin 删除插件
func (h *Handler) DeletePlugin(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plugin deleted successfully"})
}

// ListVersions 列出插件版本
func (h *Handler) ListVersions(c *gin.Context) {
	id := c.Param("id")

	versions, err := h.store.ListVersions(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plugin_id": id,
		"versions":  versions,
	})
}

// GetVersion 获取特定版本
func (h *Handler) GetVersion(c *gin.Context) {
	id := c.Param("id")
	version := c.Param("version")

	plugin, err := h.store.GetVersion(c.Request.Context(), id, version)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plugin)
}

// RatePlugin 评分插件
func (h *Handler) RatePlugin(c *gin.Context) {
	id := c.Param("id")

	var req RatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 从上下文获取用户 ID（需要认证中间件设置）
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous" // 匿名用户
	}

	if err := h.store.Rate(c.Request.Context(), id, userID, req.Score, req.Comment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rating submitted successfully"})
}

// GetPluginRating 获取插件评分
func (h *Handler) GetPluginRating(c *gin.Context) {
	id := c.Param("id")

	rating, count, err := h.store.GetRating(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plugin_id":    id,
		"rating":       rating,
		"rating_count": count,
	})
}

// DownloadPlugin 下载插件
func (h *Handler) DownloadPlugin(c *gin.Context) {
	id := c.Param("id")

	// 获取插件信息
	plugin, err := h.store.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 增加下载计数
	if err := h.store.IncrementDownloadCount(c.Request.Context(), id); err != nil {
		// 记录错误但不影响下载
		// logger.Warnf("Failed to increment download count: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"plugin_id":    id,
		"download_url": plugin.DownloadURL,
		"checksum":     plugin.Checksum,
		"signature":    plugin.Signature,
	})
}
