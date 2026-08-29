package handler

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"net"
	"os"
	"strconv"

	"centag/core/internal/cert"
	"centag/core/internal/mitm"
	"centag/core/internal/pac"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"

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

// SetMitmServer updates the MITM server reference without replacing the handler
// (Gin routes bind the original handler instance at registration time).
func (h *ProxyHandler) SetMitmServer(mitmServer *mitm.Server) {
	if h == nil {
		return
	}
	h.mitmServer = mitmServer
	if mitmServer != nil {
		h.certManager = mitmServer.GetCertManager()
		h.syncMITMRouting()
	} else {
		h.certManager = nil
	}
}

// syncMITMRouting pushes current PAC domains/path patterns into the running MITM whitelist.
func (h *ProxyHandler) syncMITMRouting() {
	if h == nil || h.mitmServer == nil || h.pacGen == nil {
		return
	}
	domains := h.pacGen.GetDomains()
	patterns := h.pacGen.GetPathPatterns()
	h.mitmServer.SetRoutingRules(domains, patterns)
	logger.Info("MITM routing rules synced from PAC",
		zap.Int("domains", len(domains)),
		zap.Int("path_patterns", len(patterns)))
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

// RefreshPACConfig replaces PAC generator config while preserving domain/pattern lists when empty in src.
func (h *ProxyHandler) RefreshPACConfig(pacConfig *pac.Config) {
	if pacConfig == nil {
		return
	}
	if h.pacConfig != nil {
		if len(pacConfig.Domains) == 0 {
			pacConfig.Domains = h.pacGen.GetDomains()
		}
		if len(pacConfig.PathPatterns) == 0 {
			pacConfig.PathPatterns = h.pacGen.GetPathPatterns()
		}
	}
	h.pacConfig = pacConfig
	h.pacGen = pac.NewPACGenerator(pacConfig)
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

// GetSetupStatus returns client/onboarding-oriented proxy setup status (auth required).
func (h *ProxyHandler) GetSetupStatus(c *gin.Context) {
	sp := config.GetDefaultSystemProxyConfig()
	apiPort := 20060
	if h.cfg != nil {
		sp = h.cfg.SystemProxy
		config.NormalizeSystemProxyConfig(&sp)
		if h.cfg.Server.Port > 0 {
			apiPort = h.cfg.Server.Port
		}
	}

	apiBase := sp.PublicAPIBase(apiPort)
	status := gin.H{
		"mode":                      sp.SetupMode(),
		"mitm_enabled":              h.mitmServer != nil,
		"listen_addr":               sp.MITMListenAddr(),
		"listen_is_loopback":        sp.ListenIsLoopback(),
		"allow_lan_clients":         sp.AllowLANClients,
		"advertise_host":            sp.AdvertiseHost,
		"suggested_lan_hosts":       config.SuggestLANHosts(),
		"in_container":              config.RunningInContainer(),
		"pac_enabled":               sp.PACEnabled,
		"pac_url":                   apiBase + "/api/v1/proxy/pac",
		"ca_download_url":           apiBase + "/api/v1/proxy/ca.crt",
		"ca_fingerprint_sha256":     h.caFingerprintSHA256(),
		"global_proxy_mode":         !sp.PACEnabled,
		"mitm_proxy":                sp.PACProxyHost() + ":" + strconv.Itoa(sp.ListenPort),
		"egress_api_key_configured": config.ResolveSystemProxyEgressAPIKey(&sp) != "",
		// RequireClientProxyAuth controls whether LAN clients must send Proxy-Authorization.
		// Disable for Bun-based agents (e.g. opencode) that don't extract auth from HTTPS_PROXY URL.
		"proxy_auth_required": sp.RequireClientProxyAuth,
		// write_config_supported reports whether the one-click "write config" action is
		// meaningful for this client. It only makes sense when the dashboard is opened on the
		// same machine that runs centag (the agent reads config from that machine's filesystem),
		// i.e. local mode + loopback access. Remote/LAN deployments must use generate+copy or wrap.
		"write_config_supported": !sp.AllowLANClients && requestHostIsLoopback(c),
		// accessed_remotely is true when the dashboard is reached via a non-loopback host,
		// which for personal edition means the server is deployed remotely relative to this browser.
		"accessed_remotely": !requestHostIsLoopback(c),
	}
	c.JSON(200, status)
}

// requestHostIsLoopback reports whether the dashboard was opened via a loopback
// address (127.0.0.1 / localhost / ::1). This is used to decide if the browser
// and the centag server are on the same machine, which is required for the
// one-click "write config" action to be meaningful.
func requestHostIsLoopback(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	host := c.Request.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return config.IsLoopbackHost(host)
}

func (h *ProxyHandler) caFingerprintSHA256() string {
	var certPEM []byte
	var err error
	if h.certManager != nil {
		certPEM, err = h.certManager.GetCACertPEM()
		if err != nil {
			return ""
		}
	} else if h.certPath != "" {
		certPEM, err = os.ReadFile(h.certPath)
		if err != nil {
			return ""
		}
	} else {
		return ""
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return ""
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
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

	// PAC + MITM 白名单立即同步（无需重启）
	h.syncMITMRouting()

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

	h.syncMITMRouting()

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

// EnsureDefaultPACRules merges default system-proxy domains/path patterns into the live PAC list.
// Existing custom entries are kept; only missing defaults are added.
func (h *ProxyHandler) EnsureDefaultPACRules(c *gin.Context) {
	defaults := config.GetDefaultSystemProxyConfig()
	addedDomains := 0
	addedPatterns := 0

	beforeDomains := len(h.pacGen.GetDomains())
	for _, d := range defaults.Domains {
		h.pacGen.AddDomain(d)
	}
	addedDomains = len(h.pacGen.GetDomains()) - beforeDomains

	beforePatterns := len(h.pacGen.GetPathPatterns())
	for _, p := range defaults.PathPatterns {
		h.pacGen.AddPathPattern(p)
	}
	addedPatterns = len(h.pacGen.GetPathPatterns()) - beforePatterns

	if h.cfg != nil {
		h.cfg.SystemProxy.Domains = h.pacGen.GetDomains()
		h.cfg.SystemProxy.PathPatterns = h.pacGen.GetPathPatterns()
		if err := config.SaveConfig(h.cfg); err != nil {
			logger.Error("Failed to save config after ensuring default PAC rules", zap.Error(err))
			c.JSON(500, gin.H{"error": "Failed to save config: " + err.Error()})
			return
		}
	}

	h.syncMITMRouting()

	logger.Info("Ensured default PAC rules",
		zap.Int("added_domains", addedDomains),
		zap.Int("added_patterns", addedPatterns),
		zap.Int("domains", len(h.pacGen.GetDomains())),
		zap.Int("patterns", len(h.pacGen.GetPathPatterns())))

	c.JSON(200, gin.H{
		"message":         "Default PAC rules ensured",
		"added_domains":   addedDomains,
		"added_patterns":  addedPatterns,
		"domains":         h.pacGen.GetDomains(),
		"path_patterns":   h.pacGen.GetPathPatterns(),
		"domains_count":   len(h.pacGen.GetDomains()),
		"patterns_count":  len(h.pacGen.GetPathPatterns()),
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
	}

	h.syncMITMRouting()

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
	}

	h.syncMITMRouting()

	logger.Info("Path pattern removed from PAC", zap.String("pattern", req.Pattern))

	c.JSON(200, gin.H{
		"message": "Path pattern removed successfully",
		"pattern": req.Pattern,
	})
}
