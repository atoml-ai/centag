package mitm

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"centag/core/internal/cert"
	"centag/core/pkg/logger"

	"go.uber.org/zap"
)

// Server MITM代理服务器
type Server struct {
	addr          string
	certManager   *cert.CertManager
	backendAddr   string // 后端LLM Proxy地址
	httpServer    *http.Server
	tlsConfig     *tls.Config
	targetDomains map[string]bool // 需要代理的目标域名
	pathPatterns  []string        // 需要代理的路径模式
}

// Config MITM服务器配置
type Config struct {
	Addr          string // 监听地址,如 ":8081"
	BackendAddr   string // 后端LLM Proxy地址,如 "127.0.0.1:20060"
	CACertPath    string
	CAKeyPath     string
	CertDir       string
	CertValidDays int
	Domains       []string // 需要代理的域名列表
	PathPatterns  []string // 需要代理的路径模式
}

// NewServer 创建MITM服务器
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
			// 获取服务器名称
			serverName := hello.ServerName
			if serverName == "" {
				// 如果没有SNI,从连接中提取
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

			// 为域名生成证书
			certBytes, privKey, err := certManager.GenerateCertForDomain(serverName)
			if err != nil {
				return nil, fmt.Errorf("failed to generate certificate for %s: %w", serverName, err)
			}

			return &tls.Certificate{
				Certificate: [][]byte{certBytes},
				PrivateKey:  privKey,
				Leaf:        nil, // 将会被自动解析
			}, nil
		},
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1", "h2"},
	}

	// 构建域名映射
	targetDomains := make(map[string]bool)
	for _, domain := range config.Domains {
		targetDomains[domain] = true
	}

	return &Server{
		addr:          config.Addr,
		certManager:   certManager,
		backendAddr:   config.BackendAddr,
		tlsConfig:     tlsConfig,
		targetDomains: targetDomains,
		pathPatterns:  config.PathPatterns,
	}, nil
}

// Start 启动MITM服务器
func (s *Server) Start() error {
	// 使用自定义Handler来处理CONNECT方法
	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      s, // 使用Server本身作为Handler
		TLSConfig:    s.tlsConfig,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("MITM proxy server starting",
		zap.String("addr", s.addr),
		zap.String("backend", s.backendAddr))

	// 监听HTTP请求
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start MITM server: %w", err)
	}

	return nil
}

// ServeHTTP 实现http.Handler接口，根据方法分发请求
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CONNECT方法用于建立HTTPS隧道
	if r.Method == http.MethodConnect {
		s.handleCONNECT(w, r)
		return
	}

	// 其他HTTP方法(GET, POST等)
	s.handleRequest(w, r)
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		logger.Info("MITM proxy server: httpServer is nil, nothing to stop")
		return nil
	}

	logger.Info("MITM proxy server stopping")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		logger.Error("MITM proxy server shutdown failed", zap.Error(err))
		return err
	}
	logger.Info("MITM proxy server stopped successfully")
	return nil
}

// handleRequest 处理HTTP请求
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 记录请求
	logger.Info("MITM HTTP request",
		zap.String("method", r.Method),
		zap.String("host", r.Host),
		zap.String("path", r.URL.Path))

	// 转发到后端LLM Proxy
	if err := s.forwardToBackend(w, r, "http"); err != nil {
		logger.Error("Failed to forward HTTP request", zap.Error(err))
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}
}

// handleCONNECT 处理HTTPS CONNECT方法
func (s *Server) handleCONNECT(w http.ResponseWriter, r *http.Request) {
	// CONNECT请求的目标主机在r.Host中
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}

	logger.Info("MITM CONNECT request", zap.String("host", host))

	// 获取底层连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logger.Error("Failed to hijack connection")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		logger.Error("Failed to hijack connection", zap.Error(err))
		return
	}
	defer clientConn.Close()

	// 告知客户端已建立连接
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		logger.Error("Failed to send connection established", zap.Error(err))
		return
	}

	// 创建TLS连接
	tlsConn := tls.Server(clientConn, s.tlsConfig)
	defer tlsConn.Close()

	// 执行TLS握手
	if err := tlsConn.Handshake(); err != nil {
		logger.Error("TLS handshake failed", zap.Error(err))
		return
	}

	// 读取客户端请求
	br := bufio.NewReader(tlsConn)
	req, err := http.ReadRequest(br)
	if err != nil {
		logger.Error("Failed to read HTTPS request", zap.Error(err))
		return
	}

	// 设置请求的Host
	req.URL.Scheme = "https"
	req.URL.Host = host
	req.Host = host
	req.RemoteAddr = tlsConn.RemoteAddr().String()

	logger.Info("MITM HTTPS request",
		zap.String("method", req.Method),
		zap.String("host", host),
		zap.String("path", req.URL.Path))

	// 创建响应写入器
	rw := &responseWriter{conn: tlsConn}

	// 判断是否需要转发到LLM Proxy后端
	if s.shouldProxyToBackend(req) {
		// 转发到LLM Proxy后端
		logger.Debug("Forwarding to Proxy Claw backend",
			zap.String("host", host),
			zap.String("path", req.URL.Path))
		if err := s.forwardToBackend(rw, req, "https"); err != nil {
			logger.Error("Failed to forward HTTPS request to backend", zap.Error(err))
		}
	} else {
		// 直接转发到原始目标服务器
		logger.Debug("Forwarding to original target",
			zap.String("host", host),
			zap.String("path", req.URL.Path))
		if err := s.forwardToOriginal(rw, req); err != nil {
			logger.Error("Failed to forward HTTPS request to original target", zap.Error(err))
		}
	}
}

// forwardToBackend 转发请求到后端
func (s *Server) forwardToBackend(w http.ResponseWriter, r *http.Request, scheme string) error {
	// 保存原始路径
	originalPath := r.URL.Path

	// 路径转换规则:
	// 1. /openai/v1/* → /v1/* (PPInfra路径 → 标准OpenAI API路径)
	// 2. /openai/chat/completions → /api/v1/openai/chat/completions
	// 3. /openai/models → /v1/models
	// 4. /v3/openai/chat/completions → /api/v1/openai/chat/completions
	// 5. /v1/* → 保持不变 (标准 OpenAI API)
	convertedPath := originalPath

	if strings.HasPrefix(originalPath, "/openai/v1/") {
		// /openai/v1/models → /v1/models
		// /openai/v1/chat/completions → /v1/chat/completions
		convertedPath = "/" + strings.TrimPrefix(originalPath, "/openai/")
	} else if strings.HasPrefix(originalPath, "/v3/openai/") {
		// /v3/openai/chat/completions → /api/v1/openai/chat/completions
		convertedPath = "/api/v1/openai/" + strings.TrimPrefix(originalPath, "/v3/openai/")
	} else if strings.HasPrefix(originalPath, "/openai/") {
		// 其他 /openai/* 路径
		suffix := strings.TrimPrefix(originalPath, "/openai/")
		if suffix == "models" || strings.HasPrefix(suffix, "models/") {
			convertedPath = "/v1/" + suffix
		} else {
			// /openai/chat/completions → /api/v1/openai/chat/completions
			convertedPath = "/api/v1/openai/" + suffix
		}
	}
	// 否则保持原路径不变 (例如 /v1/models, /v1/chat/completions)

	if convertedPath != originalPath {
		logger.Info("Path converted for backend",
			zap.String("original", originalPath),
			zap.String("converted", convertedPath))
		r.URL.Path = convertedPath
	}

	// 构建后端URL
	backendURL := *r.URL
	backendURL.Scheme = "http"
	backendURL.Host = s.backendAddr

	logger.Debug("Forwarding to backend",
		zap.String("backend_url", backendURL.String()),
		zap.String("backend_addr", s.backendAddr),
		zap.String("original_path", originalPath),
		zap.String("converted_path", convertedPath),
		zap.String("method", r.Method))

	// 创建新请求
	var body io.Reader
	if r.Body != nil {
		body = r.Body
	}

	req, err := http.NewRequest(r.Method, backendURL.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create backend request: %w", err)
	}

	// 复制请求头(跳过一些不应该转发的头)
	for key, values := range r.Header {
		if shouldForwardHeader(key) {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// 添加代理标识
	req.Header.Set("X-Forwarded-For", r.RemoteAddr)
	req.Header.Set("X-Real-IP", strings.Split(r.RemoteAddr, ":")[0])
	req.Header.Set("X-Proxy-Scheme", scheme)
	req.Header.Set("X-Original-Host", r.Host)
	req.Header.Set("X-Original-Path", originalPath) // 保存原始路径

	// 发送请求，直连后端（不走系统代理，避免循环）
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Backend request failed",
			zap.Error(err),
			zap.String("backend_url", backendURL.String()),
			zap.String("method", r.Method))
		return fmt.Errorf("backend request failed: %w", err)
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 设置状态码
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy response body: %w", err)
	}

	logger.Info("Request forwarded successfully",
		zap.String("method", r.Method),
		zap.String("original_path", originalPath),
		zap.String("converted_path", convertedPath),
		zap.Int("status", resp.StatusCode))

	return nil
}

// responseWriter 响应写入器包装器
type responseWriter struct {
	conn   net.Conn
	header http.Header
}

func (w *responseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.header != nil {
		// 写入响应头
		headers := ""
		for k, vs := range w.header {
			for _, v := range vs {
				headers += fmt.Sprintf("%s: %s\r\n", k, v)
			}
		}
		if headers != "" {
			w.conn.Write([]byte(headers))
			w.conn.Write([]byte("\r\n"))
		}
		w.header = nil
	}
	return w.conn.Write(b)
}

func (w *responseWriter) WriteHeader(statusCode int) {
	// HTTP/1.1 响应状态行
	w.conn.Write([]byte(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, http.StatusText(statusCode))))
}

// shouldProxyToBackend 判断请求是否应该转发到LLM Proxy后端
func (s *Server) shouldProxyToBackend(r *http.Request) bool {
	// 提取域名（去掉端口）
	hostWithoutPort := r.Host
	if idx := strings.Index(hostWithoutPort, ":"); idx != -1 {
		hostWithoutPort = hostWithoutPort[:idx]
	}

	// 检查域名是否在目标列表中
	if !s.targetDomains[hostWithoutPort] {
		return false
	}

	// 检查路径是否匹配
	path := r.URL.Path
	for _, pattern := range s.pathPatterns {
		if strings.HasPrefix(path, pattern) {
			return true
		}
	}

	return false
}

// forwardToOriginal 直接转发到原始目标服务器
func (s *Server) forwardToOriginal(w http.ResponseWriter, r *http.Request) error {
	// 构建目标URL
	targetURL := *r.URL
	if targetURL.Scheme == "" {
		targetURL.Scheme = "https"
	}

	// 创建新请求
	var body io.Reader
	if r.Body != nil {
		body = r.Body
	}

	req, err := http.NewRequest(r.Method, targetURL.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create original request: %w", err)
	}

	// 复制请求头
	for key, values := range r.Header {
		if shouldForwardHeader(key) {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// 设置Host头
	req.Host = r.Host

	// 发送请求，使用不走系统代理的 Transport，避免请求循环回本 MITM 服务器
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不自动跟随重定向
		},
		Transport: &http.Transport{
			Proxy: nil, // 直连，不使用系统代理（防止请求循环）
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("original request failed: %w", err)
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 设置状态码
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy response body: %w", err)
	}

	logger.Debug("Request forwarded to original target successfully",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Int("status", resp.StatusCode))

	return nil
}

// shouldForwardHeader 判断请求头是否应该转发
func shouldForwardHeader(key string) bool {
	skipHeaders := map[string]bool{
		"host":                true,
		"content-length":      true,
		"connection":          true,
		"proxy-authorization": true,
		"proxy-connection":    true,
		"te":                  true,
		"trailers":            true,
		"transfer-encoding":   true,
		"upgrade":             true,
	}

	return !skipHeaders[strings.ToLower(key)]
}

// GetCertManager 获取证书管理器
func (s *Server) GetCertManager() *cert.CertManager {
	return s.certManager
}

// GetCACertPEM 获取CA证书PEM
func (s *Server) GetCACertPEM() ([]byte, error) {
	return s.certManager.GetCACertPEM()
}
