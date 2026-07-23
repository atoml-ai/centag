package hostproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"centag/core/internal/cert"
	"centag/core/internal/proxy"
	"centag/core/pkg/logger"
	"go.uber.org/zap"
)

// Server 基于Host劫持的代理服务器
type Server struct {
	httpServer    *http.Server
	httpsServer   *http.Server
	HttpPort      int                    // 导出字段,供API访问
	HttpsPort     int                    // 导出字段,供API访问
	BackendAddr   string                 // 导出字段,供API访问
	certManager   *cert.CertManager
	domainMapping map[string]string // 域名映射: api.openai.com -> http://127.0.0.1:20060
	pathPatterns  []string
	enabled       bool
	tlsConfig     *tls.Config
	mu            sync.RWMutex
}

// Config 配置
type Config struct {
	HTTPPort      int                    // HTTP监听端口,通常为80
	HTTPSPort     int                    // HTTPS监听端口,通常为443
	BackendAddr   string                 // 后端LLM Proxy地址
	CACertPath    string
	CAKeyPath     string
	CertDir       string
	CertValidDays int
	DomainMapping map[string]string     // 域名映射: api.openai.com -> http://127.0.0.1:20060
	PathPatterns  []string              // 需要代理的路径模式
}

// NewServer 创建Host代理服务器
func NewServer(config *Config) (*Server, error) {
	// 创建证书管理器
	certManager, err := cert.NewCertManager(
		config.CACertPath,
		config.CAKeyPath,
		config.CertDir,
		config.CertValidDays,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cert manager: %w", err)
	}

	// 准备TLS配置
	tlsConfig := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			serverName := hello.ServerName
			if serverName == "" {
				if hello.Conn != nil {
					host, _, err := net.SplitHostPort(hello.Conn.RemoteAddr().String())
					if err == nil {
						serverName = host
					}
				}
			}
			if serverName == "" {
				serverName = "localhost"
			}

			logger.Debug("Generating certificate for host", zap.String("host", serverName))

			certBytes, privKey, err := certManager.GenerateCertForDomain(serverName)
			if err != nil {
				return nil, fmt.Errorf("failed to generate certificate for %s: %w", serverName, err)
			}

			return &tls.Certificate{
				Certificate: [][]byte{certBytes},
				PrivateKey:  privKey,
				Leaf:        nil,
			}, nil
		},
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1", "h2"},
	}

	return &Server{
		HttpPort:      config.HTTPPort,
		HttpsPort:     config.HTTPSPort,
		BackendAddr:   config.BackendAddr,
		certManager:   certManager,
		domainMapping: config.DomainMapping,
		pathPatterns:  config.PathPatterns,
		enabled:       false, // 默认禁用,通过API或配置启用
		tlsConfig:     tlsConfig,
	}, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Info("Starting Host proxy servers",
		zap.Int("http_port", s.HttpPort),
		zap.Int("https_port", s.HttpsPort),
		zap.Bool("enabled", s.enabled))

	// 如果未启用，不绑定端口，直接返回
	if !s.enabled {
		logger.Info("Host proxy is disabled, skipping port binding")
		return nil
	}

	// 启动HTTPS服务器（在goroutine中，避免阻塞）
	go func() {
		httpsAddr := fmt.Sprintf(":%d", s.HttpsPort)
		s.httpsServer = &http.Server{
			Addr:         httpsAddr,
			Handler:      s,
			TLSConfig:    s.tlsConfig,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		logger.Info("Host proxy HTTPS server starting", zap.String("addr", httpsAddr))
		if err := s.httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			logger.Error("Host proxy HTTPS server error", zap.Error(err))
		}
		logger.Info("Host proxy HTTPS server stopped")
	}()

	// 启动HTTP服务器（在goroutine中，避免阻塞）
	go func() {
		httpAddr := fmt.Sprintf(":%d", s.HttpPort)
		s.httpServer = &http.Server{
			Addr:         httpAddr,
			Handler:      s,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		logger.Info("Host proxy HTTP server starting", zap.String("addr", httpAddr))
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Host proxy HTTP server error", zap.Error(err))
		}
		logger.Info("Host proxy HTTP server stopped")
	}()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("Stopping host proxy servers...")

	s.mu.RLock()
	httpServer := s.httpServer
	httpsServer := s.httpsServer
	s.mu.RUnlock()

	logger.Info("Host proxy servers status",
		zap.Bool("http_server_nil", httpServer == nil),
		zap.Bool("https_server_nil", httpsServer == nil))

	var errs []error

	if httpServer != nil {
		logger.Info("Shutting down HTTP server")
		if err := httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTP server shutdown error: %w", err))
			logger.Error("HTTP server shutdown failed", zap.Error(err))
		} else {
			logger.Info("HTTP server shutdown completed")
		}
	}

	if httpsServer != nil {
		logger.Info("Shutting down HTTPS server")
		if err := httpsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTPS server shutdown error: %w", err))
			logger.Error("HTTPS server shutdown failed", zap.Error(err))
		} else {
			logger.Info("HTTPS server shutdown completed")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	logger.Info("Host proxy servers stopped successfully")
	return nil
}

// ServeHTTP 实现http.Handler接口
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 首先检查Host Proxy是否启用
	s.mu.RLock()
	enabled := s.enabled
	s.mu.RUnlock()
	
	if !enabled {
		// Host Proxy未启用，返回503错误
		logger.Debug("Host proxy is disabled, returning 503",
			zap.String("host", r.Host),
			zap.String("path", r.URL.Path))
		http.Error(w, "Host Proxy Service Unavailable - Please enable in settings", http.StatusServiceUnavailable)
		return
	}

	host := r.Host
	if host == "" {
		host = r.URL.Host
	}

	// 去掉端口号
	hostWithoutPort := host
	if idx := strings.Index(hostWithoutPort, ":"); idx != -1 {
		hostWithoutPort = hostWithoutPort[:idx]
	}

	logger.Info("Host proxy request received",
		zap.String("method", r.Method),
		zap.String("host", host),
		zap.String("path", r.URL.Path))

	// 检查是否是需要代理的LLM请求
	if s.shouldProxy(hostWithoutPort, r.URL.Path) {
		// 转发到后端LLM Proxy
		if err := s.forwardToBackend(w, r); err != nil {
			logger.Error("Failed to forward to backend",
				zap.String("host", host),
				zap.String("path", r.URL.Path),
				zap.Error(err))
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}
		return
	}

	// 不需要代理的请求,返回404
	logger.Debug("Request not proxied, returning 404",
		zap.String("host", host),
		zap.String("path", r.URL.Path))
	http.NotFound(w, r)
}

// shouldProxy 判断请求是否需要代理
func (s *Server) shouldProxy(host, path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 检查域名是否在映射中
	_, exists := s.domainMapping[host]
	if !exists {
		return false
	}

	// 检查路径是否匹配
	for _, pattern := range s.pathPatterns {
		if strings.HasPrefix(path, pattern) {
			return true
		}
	}

	return false
}

// forwardToBackend 转发请求到后端LLM Proxy
func (s *Server) forwardToBackend(w http.ResponseWriter, r *http.Request) error {
	// 构建后端URL
	backendURL, err := url.Parse(s.BackendAddr)
	if err != nil {
		return fmt.Errorf("invalid backend address: %w", err)
	}

	// 使用httputil.ReverseProxy
	reverseProxy := httputil.NewSingleHostReverseProxy(backendURL)

	// 自定义错误处理
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Reverse proxy error",
			zap.String("backend", s.BackendAddr),
			zap.String("path", r.URL.Path),
			zap.Error(err))
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	// 自定义Director修改请求
	originalDirector := reverseProxy.Director
	reverseProxy.Director = func(req *http.Request) {
		originalDirector(req)
		logger.Info("Forwarding request to backend",
			zap.String("backend", s.BackendAddr),
			zap.String("path", req.URL.Path),
			zap.String("method", req.Method),
			zap.String("original_host", r.Host))

		// 添加原始Host头
		req.Host = r.Host
		req.Header.Set("X-Original-Host", r.Host)

		// 添加原始路径
		req.Header.Set("X-Original-Path", r.URL.Path)

		// 路径转换: /v3/openai/chat/completions -> /v1/chat/completions
		if req.URL.Path == "/v3/openai/chat/completions" {
			req.URL.Path = "/v1/chat/completions"
			logger.Info("Path converted",
				zap.String("original", "/v3/openai/chat/completions"),
				zap.String("converted", req.URL.Path))
		}

		// 添加代理标识头
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Real-IP", getClientIP(r))
		req.Header.Set("X-Proxy-Type", "host-hijack")
		// 触发 Centag #j 跳板模式（固定出站）流水线
		req.Header.Set(proxy.HeaderCentagResolvedMode, "fixed-egress")
	}

	// 自定义ModifyResponse修改响应
	reverseProxy.ModifyResponse = func(resp *http.Response) error {
		logger.Info("Backend response received",
			zap.Int("status", resp.StatusCode),
			zap.String("content_length", resp.Header.Get("Content-Length")))
		return nil
	}

	// 执行反向代理
	logger.Info("Starting reverse proxy request")
	reverseProxy.ServeHTTP(w, r)
	logger.Info("Reverse proxy request completed")

	return nil
}

// getClientIP 获取客户端IP
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For可能包含多个IP,取第一个
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}

// Restart 热重启 Host 代理服务器，在不重启主服务的前提下更新监听端口。
// 调用方应确保在更新完 cfg.HostProxy 后再调用此方法。
func (s *Server) Restart(ctx context.Context, httpPort, httpsPort int) error {
	logger.Info("Restarting host proxy servers with new ports",
		zap.Int("http_port", httpPort),
		zap.Int("https_port", httpsPort))

	// 停止现有监听器
	if err := s.Stop(ctx); err != nil {
		logger.Warnf("Error stopping host proxy during restart: %v", err)
	}

	// 更新端口，清空旧的 server 引用
	s.mu.Lock()
	s.HttpPort = httpPort
	s.HttpsPort = httpsPort
	s.httpServer = nil
	s.httpsServer = nil
	s.mu.Unlock()

	// 在新端口上重新启动
	return s.Start()
}

// SetEnabled 设置服务器启用状态
func (s *Server) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	logger.Info("Host proxy enabled status changed", zap.Bool("enabled", enabled))
}

// IsEnabled 检查服务器是否启用
func (s *Server) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// UpdateDomainMapping 更新域名映射
func (s *Server) UpdateDomainMapping(mapping map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.domainMapping = mapping
	logger.Info("Domain mapping updated", zap.Int("domains", len(mapping)))
}

// GetDomainMapping 获取域名映射
func (s *Server) GetDomainMapping() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mapping := make(map[string]string)
	for k, v := range s.domainMapping {
		mapping[k] = v
	}
	return mapping
}

// GetCACertPEM 获取CA证书PEM格式
func (s *Server) GetCACertPEM() ([]byte, error) {
	return s.certManager.GetCACertPEM()
}
