package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"centag/core/pkg/logger"

	"go.uber.org/zap"
)

var (
	// 证书缓存
	certCache = make(map[string]*x509.Certificate)
	certMutex sync.RWMutex
)

// CertManager 证书管理器
type CertManager struct {
	caCert       *x509.Certificate
	caKey        *rsa.PrivateKey
	caCertPath   string
	caKeyPath    string
	certDir      string
	validityDays int
	mu           sync.RWMutex
}

// NewCertManager 创建证书管理器
func NewCertManager(caCertPath, caKeyPath, certDir string, validityDays int) (*CertManager, error) {
	m := &CertManager{
		caCertPath:   caCertPath,
		caKeyPath:    caKeyPath,
		certDir:      certDir,
		validityDays: validityDays,
	}

	// 加载或生成CA证书
	if err := m.loadOrGenerateCA(); err != nil {
		return nil, fmt.Errorf("failed to load or generate CA: %w", err)
	}

	return m, nil
}

// loadOrGenerateCA 加载或生成CA证书
func (m *CertManager) loadOrGenerateCA() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查CA证书文件是否存在
	if _, err := os.Stat(m.caCertPath); os.IsNotExist(err) {
		logger.Info("CA certificate not found, generating new one...")
		return m.generateCA()
	}

	// 加载现有CA证书
	caCert, err := m.loadCert(m.caCertPath)
	if err != nil {
		logger.Warn("Failed to load CA certificate, generating new one", zap.Error(err))
		return m.generateCA()
	}

	// 加载CA私钥
	caKey, err := m.loadKey(m.caKeyPath)
	if err != nil {
		logger.Warn("Failed to load CA private key, generating new one", zap.Error(err))
		return m.generateCA()
	}

	// 验证证书和私钥是否匹配
	if !m.matchCertAndKey(caCert, caKey) {
		logger.Warn("CA certificate and key don't match, generating new one")
		return m.generateCA()
	}

	m.caCert = caCert
	m.caKey = caKey

	logger.Info("CA certificate loaded successfully",
		zap.String("subject", caCert.Subject.CommonName),
		zap.Time("not_before", caCert.NotBefore),
		zap.Time("not_after", caCert.NotAfter))

	return nil
}

// generateCA 生成CA证书
func (m *CertManager) generateCA() error {
	// 生成私钥
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// 准备证书模板
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Centag"},
			CommonName:   "Centag CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            2,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}

	// 自签名
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// 保存CA证书
	if err := m.saveCert(certBytes, m.caCertPath); err != nil {
		return fmt.Errorf("failed to save CA certificate: %w", err)
	}

	// 保存CA私钥
	if err := m.saveKey(priv, m.caKeyPath); err != nil {
		return fmt.Errorf("failed to save CA private key: %w", err)
	}

	// 解析证书
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	m.caCert = cert
	m.caKey = priv

	logger.Info("CA certificate generated and saved successfully",
		zap.String("cert_path", m.caCertPath),
		zap.String("key_path", m.caKeyPath))

	return nil
}

// GenerateCertForDomain 为指定域名生成证书
func (m *CertManager) GenerateCertForDomain(domain string) ([]byte, *rsa.PrivateKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 注意：目前不使用缓存，每次都生成新证书
	// 因为缓存需要同时存储证书和对应的私钥
	// 这里简化实现，每次都生成新的证书和私钥对

	// 生成新的私钥
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// 准备证书模板
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Centag"},
			CommonName:   domain,
		},
		DNSNames:              []string{domain},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, m.validityDays),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// 使用CA签名
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, m.caCert, &priv.PublicKey, m.caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// 解析证书
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// 缓存证书
	certMutex.Lock()
	certCache[domain] = cert
	certMutex.Unlock()

	logger.Debug("Certificate generated for domain",
		zap.String("domain", domain),
		zap.Time("not_after", cert.NotAfter))

	// 可选: 保存到文件
	certPath := filepath.Join(m.certDir, domain+".crt")
	keyPath := filepath.Join(m.certDir, domain+".key")

	go func() {
		if err := m.saveCert(certBytes, certPath); err != nil {
			logger.Warn("Failed to save certificate file",
				zap.String("domain", domain),
				zap.Error(err))
		}
		if err := m.saveKey(priv, keyPath); err != nil {
			logger.Warn("Failed to save private key file",
				zap.String("domain", domain),
				zap.Error(err))
		}
	}()

	return certBytes, priv, nil
}

// GetCACert 获取CA证书
func (m *CertManager) GetCACert() *x509.Certificate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.caCert
}

// GetCAKey 获取CA私钥
func (m *CertManager) GetCAKey() *rsa.PrivateKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.caKey
}

// GetCACertPEM 获取CA证书的PEM格式
func (m *CertManager) GetCACertPEM() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.caCert == nil {
		return nil, errors.New("CA certificate not initialized")
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: m.caCert.Raw,
	}), nil
}

// loadCert 从文件加载证书
func (m *CertManager) loadCert(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	return x509.ParseCertificate(block.Bytes)
}

// loadKey 从文件加载私钥
func (m *CertManager) loadKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// saveCert 保存证书到文件
func (m *CertManager) saveCert(certBytes []byte, path string) error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	return os.WriteFile(path, certPEM, 0644)
}

// saveKey 保存私钥到文件
func (m *CertManager) saveKey(priv *rsa.PrivateKey, path string) error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})

	return os.WriteFile(path, keyPEM, 0600)
}

// matchCertAndKey 验证证书和私钥是否匹配
func (m *CertManager) matchCertAndKey(cert *x509.Certificate, key *rsa.PrivateKey) bool {
	pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return false
	}

	return pubKey.N.Cmp(key.N) == 0 && pubKey.E == key.E
}

// ClearCache 清除证书缓存
func (m *CertManager) ClearCache() {
	certMutex.Lock()
	certCache = make(map[string]*x509.Certificate)
	certMutex.Unlock()
	logger.Info("Certificate cache cleared")
}
