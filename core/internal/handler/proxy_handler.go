package handler

import (
	"os"

	"centag/core/internal/cert"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/internal/mitm"
	"centag/core/internal/pac"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ProxyHandler 代理处理器
type ProxyHandler struct {
	pacConfig   *pac.Config
	pacGen      *pac.PACGenerator
	mitmServer  *mitm.Server
	certManager *cert.CertManager
	cfg         *config.Config
	certPath    string // CA证书路径
	keyPath     string // CA私钥路径
}

// NewProxyHandler 创建代理处理器
func NewProxyHandler(mitmServer *mitm.Server, pacConfig *pac.Config, cfg *config.Config) *ProxyHandler {
	h := &ProxyHandler{
		pacConfig:  pacConfig,
		mitmServer: mitmServer,
		cfg:        cfg,
	}

	if h.pacConfig == nil {
		h.pacConfig = pac.DefaultConfig()
	}

	h.pacGen = pac.NewPACGenerator(h.pacConfig)

	if h.mitmServer != nil {
		h.certManager = h.mitmServer.GetCertManager()
	}

	// 设置证书路径：优先配置，其次默认值；相对路径统一按可执行文件目录解析。
	defaultProxyCfg := config.GetDefaultSystemProxyConfig()
	h.certPath = config.ResolvePathRelativeToExecutable(defaultProxyCfg.CACertPath)
	h.keyPath = config.ResolvePathRelativeToExecutable(defaultProxyCfg.CAKeyPath)
	if cfg != nil {
		if cfg.SystemProxy.CACertPath != "" {
			h.certPath = config.ResolvePathRelativeToExecutable(cfg.SystemProxy.CACertPath)
		}
		if cfg.SystemProxy.CAKeyPath != "" {
			h.keyPath = config.ResolvePathRelativeToExecutable(cfg.SystemProxy.CAKeyPath)
		}
	}

	return h
}

// ServePAC 提供PAC文件
func (h *ProxyHandler) ServePAC(c *gin.Context) {
	logger.Debug("PAC file requested", zap.String("client_ip", c.ClientIP()))

	c.Header("Content-Type", "application/x-ns-proxy-autoconfig")
	c.Header("Cache-Control", "public, max-age=300") // 缓存5分钟
	c.String(200, h.pacGen.Generate())
}

// GetCACert 提供CA证书下载
func (h *ProxyHandler) GetCACert(c *gin.Context) {
	var certPEM []byte
	var err error

	logger.Debug("GetCACert called", zap.Bool("has_cert_manager", h.certManager != nil), zap.String("cert_path", h.certPath))

	// 优先使用certManager
	if h.certManager != nil {
		certPEM, err = h.certManager.GetCACertPEM()
		if err != nil {
			logger.Error("Failed to get CA certificate from certManager", zap.Error(err))
			c.JSON(500, gin.H{"error": "Failed to get CA certificate"})
			return
		}
	} else if h.certPath != "" {
		// 直接从文件读取证书
		certPEM, err = os.ReadFile(h.certPath)
		if err != nil {
			logger.Error("Failed to read CA certificate file", zap.Error(err), zap.String("path", h.certPath))
			c.JSON(500, gin.H{"error": "Failed to read CA certificate file: " + err.Error()})
			return
		}
	} else {
		// 使用默认路径
		defaultPath := config.ResolvePathRelativeToExecutable(config.GetDefaultSystemProxyConfig().CACertPath)
		logger.Info("Using default certificate path", zap.String("path", defaultPath))
		certPEM, err = os.ReadFile(defaultPath)
		if err != nil {
			logger.Error("Failed to read CA certificate from default path", zap.Error(err))
			c.JSON(500, gin.H{"error": "Failed to read CA certificate"})
			return
		}
	}

	logger.Info("CA certificate downloaded", zap.String("client_ip", c.ClientIP()))

	c.Header("Content-Type", "application/x-x509-ca-cert")
	c.Header("Content-Disposition", "attachment; filename=\"centag-ca.crt\"")
	c.Data(200, "application/x-x509-ca-cert", certPEM)
}

// GetProxyStatus 获取代理状态
func (h *ProxyHandler) GetProxyStatus(c *gin.Context) {
	pacEnabled := true
	if h.cfg != nil {
		pacEnabled = h.cfg.SystemProxy.PACEnabled
	}

	status := gin.H{
		"enabled":      h.mitmServer != nil,
		"pac_enabled":  pacEnabled,
		"pac_domains":  h.pacGen.GetDomains(),
		"pac_patterns": h.pacGen.GetPathPatterns(),
	}

	c.JSON(200, status)
}

// AddDomain 添加域名到PAC
func (h *ProxyHandler) AddDomain(c *gin.Context) {
	var req struct {
		Domain string `json:"domain" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// 添加域名到PAC生成器
	h.pacGen.AddDomain(req.Domain)

	// 更新配置并保存到文件
	if h.cfg != nil {
		h.cfg.SystemProxy.Domains = h.pacGen.GetDomains()
		if err := config.SaveConfig(h.cfg); err != nil {
			logger.Error("Failed to save config after adding domain", zap.Error(err), zap.String("domain", req.Domain))
			c.JSON(500, gin.H{"error": "Failed to save config: " + err.Error()})
			return
		}
	}

	logger.Info("Domain added to PAC", zap.String("domain", req.Domain))

	c.JSON(200, gin.H{
		"message": "Domain added successfully",
		"domain":  req.Domain,
	})
}

// RemoveDomain 从PAC移除域名
func (h *ProxyHandler) RemoveDomain(c *gin.Context) {
	var req struct {
		Domain string `json:"domain" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// 从PAC生成器移除域名
	h.pacGen.RemoveDomain(req.Domain)

	// 更新配置并保存到文件
	if h.cfg != nil {
		h.cfg.SystemProxy.Domains = h.pacGen.GetDomains()
		if err := config.SaveConfig(h.cfg); err != nil {
			logger.Error("Failed to save config after removing domain", zap.Error(err), zap.String("domain", req.Domain))
			c.JSON(500, gin.H{"error": "Failed to save config: " + err.Error()})
			return
		}
	}

	logger.Info("Domain removed from PAC", zap.String("domain", req.Domain))

	c.JSON(200, gin.H{
		"message": "Domain removed successfully",
		"domain":  req.Domain,
	})
}

// GetPACDomains 获取PAC域名列表
func (h *ProxyHandler) GetPACDomains(c *gin.Context) {
	domains := h.pacGen.GetDomains()
	c.JSON(200, gin.H{
		"domains": domains,
		"count":   len(domains),
	})
}

// AddPathPattern 添加路径模式
func (h *ProxyHandler) AddPathPattern(c *gin.Context) {
	var req struct {
		Pattern string `json:"pattern" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// 添加路径模式到PAC生成器
	h.pacGen.AddPathPattern(req.Pattern)

	// 更新配置并保存到文件
	if h.cfg != nil {
		h.cfg.SystemProxy.PathPatterns = h.pacGen.GetPathPatterns()
		if err := config.SaveConfig(h.cfg); err != nil {
			logger.Error("Failed to save config after adding path pattern", zap.Error(err), zap.String("pattern", req.Pattern))
			c.JSON(500, gin.H{"error": "Failed to save config: " + err.Error()})
			return
		}

		// 如果MITM服务器存在，需要重启以应用新配置
		if h.mitmServer != nil {
			logger.Info("Path pattern added, MITM server may need restart to apply changes", zap.String("pattern", req.Pattern))
		}
	}

	logger.Info("Path pattern added to PAC", zap.String("pattern", req.Pattern))

	c.JSON(200, gin.H{
		"message": "Path pattern added successfully",
		"pattern": req.Pattern,
	})
}

// RemovePathPattern 移除路径模式
func (h *ProxyHandler) RemovePathPattern(c *gin.Context) {
	var req struct {
		Pattern string `json:"pattern" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// 从PAC生成器移除路径模式
	h.pacGen.RemovePathPattern(req.Pattern)

	// 更新配置并保存到文件
	if h.cfg != nil {
		h.cfg.SystemProxy.PathPatterns = h.pacGen.GetPathPatterns()
		if err := config.SaveConfig(h.cfg); err != nil {
			logger.Error("Failed to save config after removing path pattern", zap.Error(err), zap.String("pattern", req.Pattern))
			c.JSON(500, gin.H{"error": "Failed to save config: " + err.Error()})
			return
		}

		// 如果MITM服务器存在，需要重启以应用新配置
		if h.mitmServer != nil {
			logger.Info("Path pattern removed, MITM server may need restart to apply changes", zap.String("pattern", req.Pattern))
		}
	}

	logger.Info("Path pattern removed from PAC", zap.String("pattern", req.Pattern))

	c.JSON(200, gin.H{
		"message": "Path pattern removed successfully",
		"pattern": req.Pattern,
	})
}
