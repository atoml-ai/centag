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
	"sync"
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
	mu               sync.RWMutex
	targetDomains    map[string]bool // 需要代理的目标域名（热更新）
	pathPatterns     []string        // 需要代理的路径模式（热更新）
	backendAuthToken string          // Centag llmproxy_*；注入到转发请求（热更新）
}

// Config MITM服务器配置
type Config struct {
	Addr             string // 监听地址,如 ":8081"
	BackendAddr      string // 后端LLM Proxy地址,如 "127.0.0.1:20060"
	CACertPath       string
	CAKeyPath        string
	CertDir          string
	CertValidDays    int
	Domains          []string // 需要代理的域名列表
	PathPatterns     []string // 需要代理的路径模式
	BackendAuthToken string   // Centag API key injected when forwarding to BackendAddr
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

	s := &Server{
		addr:        config.Addr,
		certManager: certManager,
		backendAddr: config.BackendAddr,
		tlsConfig:   tlsConfig,
	}
	s.SetRoutingRules(config.Domains, config.PathPatterns)
	s.SetBackendAuthToken(config.BackendAuthToken)
	return s, nil
}

// SetBackendAuthToken hot-updates the Centag key injected on backend forward.
func (s *Server) SetBackendAuthToken(token string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.backendAuthToken = strings.TrimSpace(token)
	s.mu.Unlock()
}

func (s *Server) backendAuthTokenLocked() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backendAuthToken
}

// SetRoutingRules hot-updates domain/path whitelist used by shouldProxyToBackend.
// Called when PAC 规则管理增减域名或路径模式，无需重启 MITM。
func (s *Server) SetRoutingRules(domains, pathPatterns []string) {
	if s == nil {
		return
	}
	targetDomains := make(map[string]bool, len(domains))
	for _, domain := range domains {
		d := strings.ToLower(strings.TrimSpace(domain))
		if d == "" {
			continue
		}
		targetDomains[d] = true
	}
	pats := make([]string, 0, len(pathPatterns))
	for _, p := range pathPatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pats = append(pats, p)
	}

	s.mu.Lock()
	s.targetDomains = targetDomains
	s.pathPatterns = pats
	s.mu.Unlock()
}

func stripHostPort(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return ""
	}
	// host:port（IPv6 带括号时少见，MITM 目标多为域名）
	if i := strings.LastIndex(h, ":"); i >= 0 && !strings.Contains(h, "]") {
		h = h[:i]
	}
	return h
}

// isWhitelistedHost reports whether CONNECT host is in the PAC/MITM domain allowlist.
func (s *Server) isWhitelistedHost(host string) bool {
	if s == nil {
		return false
	}
	h := stripHostPort(host)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.targetDomains[h]
}

// ShouldRouteToBackend reports whether host+path should be forwarded to Centag (:20060).
// Domain must be whitelisted; path matches configured prefixes OR looks like a generic LLM API.
func (s *Server) ShouldRouteToBackend(host, path string) bool {
	if s == nil {
		return false
	}
	hostWithoutPort := stripHostPort(host)

	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.targetDomains[hostWithoutPort] {
		return false
	}
	for _, pattern := range s.pathPatterns {
		if pattern != "" && strings.HasPrefix(path, pattern) {
			return true
		}
	}
	// Domain already classified as LLM provider → accept common API shapes without
	// per-agent path entries (e.g. /zen/v1/responses on opencode.ai).
	return looksLikeLLMAPIPath(path)
}

// Start 启动MITM服务器
func (s *Server) Start() error {
	// 使用自定义Handler来处理CONNECT方法
	// 长 SSE / 流式转发不能设短 WriteTimeout，否则会在 ~30s 切断响应。
	s.httpServer = &http.Server{
		Addr:              s.addr,
		Handler:           s, // 使用Server本身作为Handler
		TLSConfig:         s.tlsConfig,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       300 * time.Second,
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

// handleRequest 处理明文 HTTP 代理请求（较少见；LLM 多为 HTTPS CONNECT）。
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	logger.Info("MITM HTTP request",
		zap.String("method", r.Method),
		zap.String("host", r.Host),
		zap.String("path", r.URL.Path))

	if s.shouldProxyToBackend(r) {
		if err := s.forwardToBackend(w, r, "http"); err != nil {
			logger.Error("Failed to forward HTTP request", zap.Error(err))
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}
		return
	}
	if err := s.forwardToOriginal(w, r); err != nil {
		logger.Error("Failed to forward HTTP to original", zap.Error(err))
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}
}

// handleCONNECT 处理 HTTPS CONNECT。
// 仅对白名单 LLM 域名做 MITM 解密；其它主机走 TCP 隧道，避免影响 Agent 非大模型上网。
func (s *Server) handleCONNECT(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}

	logger.Info("MITM CONNECT request", zap.String("host", host))

	if !s.isWhitelistedHost(host) {
		s.handleCONNECTTunnel(w, r, host)
		return
	}

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
		logger.Debug("Forwarding to Centag backend",
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

// handleCONNECTTunnel 对非白名单主机建立纯 TCP 隧道（不签发证书、不解密）。
// 使进程级 HTTPS_PROXY 指向本 MITM 时，Agent 的 WebFetch/git/npm 等非 LLM 流量仍可正常直达目标。
func (s *Server) handleCONNECTTunnel(w http.ResponseWriter, r *http.Request, host string) {
	dest := host
	if !strings.Contains(dest, ":") {
		dest = dest + ":443"
	}

	targetConn, err := net.DialTimeout("tcp", dest, 30*time.Second)
	if err != nil {
		logger.Error("CONNECT tunnel dial failed", zap.String("host", dest), zap.Error(err))
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		targetConn.Close()
		logger.Error("Failed to hijack connection for tunnel")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		targetConn.Close()
		logger.Error("Failed to hijack connection for tunnel", zap.Error(err))
		return
	}
	defer clientConn.Close()
	defer targetConn.Close()

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		logger.Error("Failed to send tunnel established", zap.Error(err))
		return
	}

	logger.Info("CONNECT tunnel (no MITM)", zap.String("host", dest))

	errCh := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(targetConn, clientConn)
		errCh <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(clientConn, targetConn)
		errCh <- struct{}{}
	}()
	<-errCh
}

// knownLLMPathMarkers are vendor-agnostic suffixes that identify LLM API calls.
// Once MITM has decided to send traffic to Centag, paths are normalized via these.
var knownLLMPathMarkers = []struct {
	suffix string
	centag string
}{
	{"/chat/completions", "/v1/chat/completions"},
	{"/v1/messages", "/v1/messages"},
	{"/messages", "/v1/messages"},
	{"/responses", "/v1/responses"},
	{"/embeddings", "/v1/embeddings"},
	{"/completions", "/v1/completions"}, // after /chat/completions
	{"/models", "/v1/models"},
}

// looksLikeLLMAPIPath reports whether path looks like an LLM provider API call.
// Used together with domain whitelist — not for arbitrary internet traffic.
func looksLikeLLMAPIPath(path string) bool {
	if strings.Contains(path, "/v1/") || path == "/v1" {
		return true
	}
	for _, m := range knownLLMPathMarkers {
		if path == m.suffix || strings.HasSuffix(path, m.suffix) || strings.Contains(path, m.suffix+"/") {
			return true
		}
		if strings.Contains(path, m.suffix+"?") { // unlikely in URL.Path but cheap
			return true
		}
		if strings.Contains(path, m.suffix) {
			return true
		}
	}
	return false
}

// convertBackendPath maps any vendor LLM path onto Centag's canonical /v1/* surface.
// Principle: once traffic is classified as LLM proxy, do NOT keep per-agent prefixes
// (/zen, /openai, …); strip to Centag routes so every Agent hits the same gateway.
func convertBackendPath(originalPath string) string {
	if originalPath == "" {
		return originalPath
	}
	// Already on Centag OpenAI-compatible surface.
	if strings.HasPrefix(originalPath, "/v1/") || originalPath == "/v1" {
		return originalPath
	}
	// Generic: …/v1/foo → /v1/foo (covers /zen/v1/…, /openai/v1/…, /gateway/v1/…)
	if idx := strings.LastIndex(originalPath, "/v1/"); idx >= 0 {
		return originalPath[idx:]
	}
	if originalPath == "/v1" || strings.HasSuffix(originalPath, "/v1") {
		return "/v1"
	}
	// Suffix → canonical Centag route (no /v1/ in original path).
	// Longer / more specific markers first (chat/completions before completions).
	for _, m := range knownLLMPathMarkers {
		if originalPath == m.suffix || strings.HasSuffix(originalPath, m.suffix) {
			return m.centag
		}
		if i := strings.Index(originalPath, m.suffix+"/"); i >= 0 {
			// e.g. /xxx/models/gpt-4 → /v1/models/gpt-4
			return m.centag + originalPath[i+len(m.suffix):]
		}
	}
	// Legacy PPInfra / vendor shapes without a /v1/ segment.
	if strings.HasPrefix(originalPath, "/v3/openai/") {
		return "/api/v1/openai/" + strings.TrimPrefix(originalPath, "/v3/openai/")
	}
	if strings.HasPrefix(originalPath, "/openai/") {
		suffix := strings.TrimPrefix(originalPath, "/openai/")
		if suffix == "models" || strings.HasPrefix(suffix, "models/") {
			return "/v1/" + suffix
		}
		return "/api/v1/openai/" + suffix
	}
	// Domain+pattern already said "LLM"; default to chat completions as safest gateway entry.
	// Callers that matched only on domain should still land on a real Centag handler (not SPA).
	return "/v1/chat/completions"
}

// forwardToBackend 转发请求到后端
func (s *Server) forwardToBackend(w http.ResponseWriter, r *http.Request, scheme string) error {
	// 保存原始路径
	originalPath := r.URL.Path
	convertedPath := convertBackendPath(originalPath)

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

	req, err := http.NewRequestWithContext(r.Context(), r.Method, backendURL.String(), body)
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

	// Agent 填的是上游厂商 Token，不知道被 MITM；转发到 Centag 时注入网关 Key。
	applyBackendAuth(req, s.backendAuthTokenLocked())

	// 直连后端；不设短 Client.Timeout（流式响应可能远超 60s），由请求 Context 取消。
	client := &http.Client{
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
	return s.ShouldRouteToBackend(r.Host, r.URL.Path)
}

// applyBackendAuth replaces Agent Authorization with Centag egress key.
// Original Bearer is kept in X-Original-Authorization for diagnostics only.
func applyBackendAuth(req *http.Request, centagToken string) {
	if req == nil || strings.TrimSpace(centagToken) == "" {
		return
	}
	if orig := strings.TrimSpace(req.Header.Get("Authorization")); orig != "" {
		req.Header.Set("X-Original-Authorization", orig)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(centagToken))
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
