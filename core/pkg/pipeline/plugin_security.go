package pipeline

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"centag/core/pkg/config"
)

type PluginSecurityValidator struct {
	cfg config.PluginSecurityConfig
}

func NewPluginSecurityValidator(cfg config.PluginSecurityConfig) *PluginSecurityValidator {
	return &PluginSecurityValidator{cfg: cfg}
}

func (v *PluginSecurityValidator) ValidateSource(pluginURL string) error {
	if !v.cfg.AllowlistEnabled {
		return nil
	}

	parsedURL, err := url.Parse(pluginURL)
	if err != nil {
		return fmt.Errorf("invalid plugin URL: %w", err)
	}

	host := parsedURL.Host
	if host == "" {
		return fmt.Errorf("plugin URL has no host")
	}

	if len(v.cfg.AllowedSources) > 0 {
		found := false
		for _, source := range v.cfg.AllowedSources {
			if strings.EqualFold(host, source) || strings.HasSuffix(host, "."+source) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("plugin source %s not in allowlist", host)
		}
	}

	if len(v.cfg.AllowedHosts) > 0 {
		ip := net.ParseIP(host)
		hostAllowed := false
		for _, allowed := range v.cfg.AllowedHosts {
			if ip != nil {
				if _, network, err := net.ParseCIDR(allowed); err == nil {
					if network.Contains(ip) {
						hostAllowed = true
						break
					}
				}
				if allowed == host {
					hostAllowed = true
					break
				}
			} else {
				if strings.EqualFold(host, allowed) || strings.HasSuffix(host, "."+allowed) {
					hostAllowed = true
					break
				}
			}
		}
		if !hostAllowed {
			return fmt.Errorf("plugin host %s not in allowlist", host)
		}
	}

	return nil
}

// ValidateSignature 使用 Ed25519 验证 manifest 签名。
// signature 为 base64 编码的 Ed25519 签名（64 字节）。
// manifestContent 为被签名的原始内容（通常为 manifest JSON）。
// 当 RequireSignature 为 false 且 signature 为空时，返回 nil（跳过验证）。
// 当配置了 TrustedPublicKeys 时，尝试用每个公钥验证，任一通过即成功。
func (v *PluginSecurityValidator) ValidateSignature(manifestContent []byte, signature string) error {
	if !v.cfg.RequireSignature && signature == "" {
		return nil
	}

	if signature == "" {
		return fmt.Errorf("signature required but not provided")
	}

	if len(v.cfg.TrustedPublicKeys) == 0 {
		if v.cfg.RequireSignature {
			return fmt.Errorf("signature required but no trusted public keys configured")
		}
		return nil
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	if len(sigBytes) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length: expected %d, got %d", ed25519.SignatureSize, len(sigBytes))
	}

	for _, pkB64 := range v.cfg.TrustedPublicKeys {
		pkBytes, err := base64.StdEncoding.DecodeString(pkB64)
		if err != nil {
			continue
		}
		if len(pkBytes) != ed25519.PublicKeySize {
			continue
		}
		if ed25519.Verify(pkBytes, manifestContent, sigBytes) {
			return nil
		}
	}

	return fmt.Errorf("signature verification failed: no trusted public key matched")
}

// ValidateManifestSignature 从 NodePluginDescriptor 中提取签名并验证。
// 如果 descriptor 中包含 Signature 字段，则验证该签名覆盖 manifest 的 JSON 序列化内容。
func (v *PluginSecurityValidator) ValidateManifestSignature(descriptor NodePluginDescriptor) error {
	if descriptor.Signature == "" {
		if v.cfg.RequireSignature {
			return fmt.Errorf("signature required but manifest has no signature")
		}
		return nil
	}

	// 序列化 manifest 为规范 JSON（不含 signature 字段，避免循环）
	manifestCopy := descriptor
	manifestCopy.Signature = ""
	manifestBytes, err := json.Marshal(manifestCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest for signature verification: %w", err)
	}

	return v.ValidateSignature(manifestBytes, descriptor.Signature)
}

func (v *PluginSecurityValidator) ValidateHash(manifestContent []byte, expectedHash string) error {
	if !v.cfg.RequireHashLock && expectedHash == "" {
		return nil
	}

	if v.cfg.RequireHashLock && expectedHash == "" {
		return fmt.Errorf("hash lock required but not configured")
	}

	hash := sha256.Sum256(manifestContent)
	actualHash := hex.EncodeToString(hash[:])
	normalizedExpected := strings.TrimSpace(strings.ToLower(expectedHash))
	if normalizedExpected != actualHash {
		return fmt.Errorf("manifest hash mismatch: expected %s, got %s", normalizedExpected, actualHash)
	}

	return nil
}

func (v *PluginSecurityValidator) ValidateEndpoint(endpoint string) error {
	if !v.cfg.NetworkPolicy.DefaultDeny {
		return nil
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	host := parsedURL.Host
	if host == "" {
		return fmt.Errorf("endpoint URL has no host")
	}

	for _, blocked := range v.cfg.NetworkPolicy.BlockedEndpoints {
		if strings.EqualFold(host, blocked) || strings.HasSuffix(host, "."+blocked) {
			return fmt.Errorf("endpoint %s is blocked", endpoint)
		}
	}

	if len(v.cfg.NetworkPolicy.AllowedEndpoints) > 0 {
		allowed := false
		for _, allowedEndpoint := range v.cfg.NetworkPolicy.AllowedEndpoints {
			if strings.EqualFold(host, allowedEndpoint) || strings.HasSuffix(host, "."+allowedEndpoint) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("endpoint %s not in allowed list", endpoint)
		}
	}

	port := parsedURL.Port()
	if port != "" {
		var portNum int
		fmt.Sscanf(port, "%d", &portNum)

		for _, blocked := range v.cfg.NetworkPolicy.BlockedPorts {
			if portNum == blocked {
				return fmt.Errorf("port %d is blocked", portNum)
			}
		}

		if len(v.cfg.NetworkPolicy.AllowedPorts) > 0 {
			portAllowed := false
			for _, allowed := range v.cfg.NetworkPolicy.AllowedPorts {
				if portNum == allowed {
					portAllowed = true
					break
				}
			}
			if !portAllowed {
				return fmt.Errorf("port %d not in allowed list", portNum)
			}
		}
	}

	return nil
}

func (v *PluginSecurityValidator) IsEnabled() bool {
	return v.cfg.AllowlistEnabled || v.cfg.RequireSignature || v.cfg.RequireHashLock || v.cfg.NetworkPolicy.DefaultDeny
}
