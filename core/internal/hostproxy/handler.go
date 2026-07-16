package hostproxy

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"centag/core/pkg/logger"
	"go.uber.org/zap"
)

// Handler Host代理管理API处理器
type Handler struct {
	server *Server
}

// NewHandler 创建处理器
func NewHandler(server *Server) *Handler {
	return &Handler{
		server: server,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	proxyGroup := router.Group("/host-proxy")
	{
		proxyGroup.GET("/status", h.GetStatus)
		proxyGroup.POST("/enable", h.SetEnabled)
		proxyGroup.PUT("/domains", h.UpdateDomains)
		proxyGroup.GET("/ca-cert", h.GetCACert)
	}
}

// GetStatus 获取状态
func (h *Handler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled":      h.server.IsEnabled(),
		"domains":      h.server.GetDomainMapping(),
		"http_port":    h.server.HttpPort,
		"https_port":   h.server.HttpsPort,
		"backend_addr": h.server.BackendAddr,
	})
}

// SetEnabled 设置启用状态
func (h *Handler) SetEnabled(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.server.SetEnabled(req.Enabled)

	logger.Info("Host proxy status changed", zap.Bool("enabled", req.Enabled))
	c.JSON(http.StatusOK, gin.H{
		"message": "Status updated",
		"enabled":  req.Enabled,
	})
}

// UpdateDomains 更新域名映射
func (h *Handler) UpdateDomains(c *gin.Context) {
	var req struct {
		Domains map[string]string `json:"domains"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.server.UpdateDomainMapping(req.Domains)

	logger.Info("Host proxy domains updated", zap.Int("count", len(req.Domains)))
	c.JSON(http.StatusOK, gin.H{
		"message": "Domains updated",
		"domains": req.Domains,
	})
}

// GetCACert 获取CA证书
func (h *Handler) GetCACert(c *gin.Context) {
	certPEM, err := h.server.GetCACertPEM()
	if err != nil {
		logger.Error("Failed to get CA cert", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get CA certificate"})
		return
	}

	c.Header("Content-Type", "application/x-pem-file")
	c.Header("Content-Disposition", "attachment; filename=ca.crt")
	c.Data(http.StatusOK, "application/x-pem-file", certPEM)
}
