package pipeline

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"centag/core/pkg/config"
)

func TestPluginSecurityValidator_ValidateSource(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config.PluginSecurityConfig
		pluginURL   string
		wantErr     bool
		errContains string
	}{
		{
			name: "allowlist disabled - should pass",
			cfg: config.PluginSecurityConfig{
				AllowlistEnabled: false,
			},
			pluginURL: "https://untrusted.com/plugin",
			wantErr:   false,
		},
		{
			name: "allowlist enabled - source in list",
			cfg: config.PluginSecurityConfig{
				AllowlistEnabled: true,
				AllowedSources:   []string{"example.com", "trusted.io"},
			},
			pluginURL: "https://example.com/plugin",
			wantErr:   false,
		},
		{
			name: "allowlist enabled - source not in list",
			cfg: config.PluginSecurityConfig{
				AllowlistEnabled: true,
				AllowedSources:   []string{"example.com", "trusted.io"},
			},
			pluginURL:   "https://untrusted.com/plugin",
			wantErr:     true,
			errContains: "not in allowlist",
		},
		{
			name: "allowlist enabled - subdomain match",
			cfg: config.PluginSecurityConfig{
				AllowlistEnabled: true,
				AllowedSources:   []string{"example.com"},
			},
			pluginURL: "https://sub.example.com/plugin",
			wantErr:   false,
		},
		{
			name: "allowed hosts - IP in CIDR",
			cfg: config.PluginSecurityConfig{
				AllowlistEnabled: true,
				AllowedHosts:     []string{"192.168.1.0/24"},
			},
			pluginURL: "https://192.168.1.100/plugin",
			wantErr:   false,
		},
		{
			name: "allowed hosts - IP not in CIDR",
			cfg: config.PluginSecurityConfig{
				AllowlistEnabled: true,
				AllowedHosts:     []string{"192.168.1.0/24"},
			},
			pluginURL:   "https://192.168.2.100/plugin",
			wantErr:     true,
			errContains: "not in allowlist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewPluginSecurityValidator(tt.cfg)
			err := v.ValidateSource(tt.pluginURL)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestPluginSecurityValidator_ValidateEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config.PluginSecurityConfig
		endpoint    string
		wantErr     bool
		errContains string
	}{
		{
			name: "default deny disabled - should pass",
			cfg: config.PluginSecurityConfig{
				NetworkPolicy: config.PluginNetworkPolicy{
					DefaultDeny: false,
				},
			},
			endpoint: "https://any-site.com/api",
			wantErr:  false,
		},
		{
			name: "default deny enabled - endpoint allowed",
			cfg: config.PluginSecurityConfig{
				NetworkPolicy: config.PluginNetworkPolicy{
					DefaultDeny:      true,
					AllowedEndpoints: []string{"api.openai.com"},
				},
			},
			endpoint: "https://api.openai.com/v1/chat",
			wantErr:  false,
		},
		{
			name: "default deny enabled - endpoint not allowed",
			cfg: config.PluginSecurityConfig{
				NetworkPolicy: config.PluginNetworkPolicy{
					DefaultDeny:      true,
					AllowedEndpoints: []string{"api.openai.com"},
				},
			},
			endpoint:    "https://untrusted.com/api",
			wantErr:     true,
			errContains: "not in allowed list",
		},
		{
			name: "default deny enabled - endpoint blocked",
			cfg: config.PluginSecurityConfig{
				NetworkPolicy: config.PluginNetworkPolicy{
					DefaultDeny:      true,
					BlockedEndpoints: []string{"malicious.com"},
				},
			},
			endpoint:    "https://malicious.com/api",
			wantErr:     true,
			errContains: "is blocked",
		},
		{
			name: "port allowed",
			cfg: config.PluginSecurityConfig{
				NetworkPolicy: config.PluginNetworkPolicy{
					DefaultDeny:  true,
					AllowedPorts: []int{443, 8080},
				},
			},
			endpoint: "https://api.example.com:443/v1",
			wantErr:  false,
		},
		{
			name: "port blocked",
			cfg: config.PluginSecurityConfig{
				NetworkPolicy: config.PluginNetworkPolicy{
					DefaultDeny:  true,
					BlockedPorts: []int{22, 3389},
				},
			},
			endpoint:    "https://api.example.com:22/v1",
			wantErr:     true,
			errContains: "is blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewPluginSecurityValidator(tt.cfg)
			err := v.ValidateEndpoint(tt.endpoint)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestPluginSecurityValidator_IsEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.PluginSecurityConfig
		want bool
	}{
		{
			name: "all disabled",
			cfg: config.PluginSecurityConfig{
				AllowlistEnabled: false,
				RequireSignature: false,
				RequireHashLock:  false,
				NetworkPolicy: config.PluginNetworkPolicy{
					DefaultDeny: false,
				},
			},
			want: false,
		},
		{
			name: "allowlist enabled",
			cfg: config.PluginSecurityConfig{
				AllowlistEnabled: true,
			},
			want: true,
		},
		{
			name: "require signature",
			cfg: config.PluginSecurityConfig{
				RequireSignature: true,
			},
			want: true,
		},
		{
			name: "require hash lock",
			cfg: config.PluginSecurityConfig{
				RequireHashLock: true,
			},
			want: true,
		},
		{
			name: "default deny enabled",
			cfg: config.PluginSecurityConfig{
				NetworkPolicy: config.PluginNetworkPolicy{
					DefaultDeny: true,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewPluginSecurityValidator(tt.cfg)
			got := v.IsEnabled()
			if got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluginSecurityValidator_ValidateHash(t *testing.T) {
	manifest := []byte(`{"implementation":"example.test","kind":"test.node"}`)
	sum := sha256.Sum256(manifest)
	expectedHash := fmt.Sprintf("%x", sum[:])

	tests := []struct {
		name         string
		cfg          config.PluginSecurityConfig
		expectedHash string
		wantErr      bool
		errContains  string
	}{
		{
			name: "hash lock disabled and no hash",
			cfg: config.PluginSecurityConfig{
				RequireHashLock: false,
			},
			wantErr: false,
		},
		{
			name: "hash lock enabled with matching hash",
			cfg: config.PluginSecurityConfig{
				RequireHashLock: true,
			},
			expectedHash: expectedHash,
			wantErr:      false,
		},
		{
			name: "hash lock enabled without hash",
			cfg: config.PluginSecurityConfig{
				RequireHashLock: true,
			},
			wantErr:     true,
			errContains: "hash lock required",
		},
		{
			name: "explicit hash mismatch",
			cfg: config.PluginSecurityConfig{
				RequireHashLock: false,
			},
			expectedHash: "deadbeef",
			wantErr:      true,
			errContains:  "manifest hash mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewPluginSecurityValidator(tt.cfg)
			err := v.ValidateHash(manifest, tt.expectedHash)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPluginSecurityValidator_ValidateSignature(t *testing.T) {
	// 生成测试用的 Ed25519 密钥对
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	publicKeyB64 := base64.StdEncoding.EncodeToString(publicKey)

	manifest := []byte(`{"implementation":"example.test","kind":"test.node","version":"1.0.0"}`)
	signature := ed25519.Sign(privateKey, manifest)
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	tests := []struct {
		name        string
		cfg         config.PluginSecurityConfig
		manifest    []byte
		signature   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "signature not required and empty",
			cfg:       config.PluginSecurityConfig{RequireSignature: false},
			manifest:  manifest,
			signature: "",
			wantErr:   false,
		},
		{
			name:        "signature required but empty",
			cfg:         config.PluginSecurityConfig{RequireSignature: true, TrustedPublicKeys: []string{publicKeyB64}},
			manifest:    manifest,
			signature:   "",
			wantErr:     true,
			errContains: "signature required but not provided",
		},
		{
			name:        "signature required but no trusted keys",
			cfg:         config.PluginSecurityConfig{RequireSignature: true},
			manifest:    manifest,
			signature:   signatureB64,
			wantErr:     true,
			errContains: "no trusted public keys configured",
		},
		{
			name:      "valid signature with trusted key",
			cfg:       config.PluginSecurityConfig{RequireSignature: true, TrustedPublicKeys: []string{publicKeyB64}},
			manifest:  manifest,
			signature: signatureB64,
			wantErr:   false,
		},
		{
			name:        "invalid signature",
			cfg:         config.PluginSecurityConfig{RequireSignature: true, TrustedPublicKeys: []string{publicKeyB64}},
			manifest:    manifest,
			signature:   base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize)),
			wantErr:     true,
			errContains: "signature verification failed",
		},
		{
			name:        "malformed signature encoding",
			cfg:         config.PluginSecurityConfig{RequireSignature: true, TrustedPublicKeys: []string{publicKeyB64}},
			manifest:    manifest,
			signature:   "not-base64!!!",
			wantErr:     true,
			errContains: "invalid signature encoding",
		},
		{
			name:        "wrong public key",
			cfg:         config.PluginSecurityConfig{RequireSignature: true, TrustedPublicKeys: []string{"YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY="}},
			manifest:    manifest,
			signature:   signatureB64,
			wantErr:     true,
			errContains: "signature verification failed",
		},
		{
			name:      "optional signature with valid sig",
			cfg:       config.PluginSecurityConfig{RequireSignature: false, TrustedPublicKeys: []string{publicKeyB64}},
			manifest:  manifest,
			signature: signatureB64,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewPluginSecurityValidator(tt.cfg)
			err := v.ValidateSignature(tt.manifest, tt.signature)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPluginSecurityValidator_ValidateManifestSignature(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	publicKeyB64 := base64.StdEncoding.EncodeToString(publicKey)

	// 构建带签名的 manifest
	descriptor := NodePluginDescriptor{
		Implementation: "example.test",
		Kind:           "test.node",
		Version:        "1.0.0",
	}
	manifestBytes, _ := json.Marshal(descriptor)
	signature := ed25519.Sign(privateKey, manifestBytes)
	descriptor.Signature = base64.StdEncoding.EncodeToString(signature)

	cfg := config.PluginSecurityConfig{
		RequireSignature:  true,
		TrustedPublicKeys: []string{publicKeyB64},
	}
	v := NewPluginSecurityValidator(cfg)

	if err := v.ValidateManifestSignature(descriptor); err != nil {
		t.Fatalf("expected valid manifest signature, got error: %v", err)
	}

	// 测试无签名且非强制的情况
	cfg2 := config.PluginSecurityConfig{RequireSignature: false}
	v2 := NewPluginSecurityValidator(cfg2)
	descriptor2 := NodePluginDescriptor{Implementation: "test"}
	if err := v2.ValidateManifestSignature(descriptor2); err != nil {
		t.Fatalf("expected no error for optional signature, got: %v", err)
	}

	// 测试无签名但强制的情况
	cfg3 := config.PluginSecurityConfig{RequireSignature: true}
	v3 := NewPluginSecurityValidator(cfg3)
	if err := v3.ValidateManifestSignature(descriptor2); err == nil {
		t.Fatalf("expected error for required but missing signature")
	}
}
