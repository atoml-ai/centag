package proxy

import (
	"net/http"
	"net/url"
	"testing"

	"centag/core/pkg/config"
)

func TestDetectProxyModeLegacy(t *testing.T) {
	t.Cleanup(func() { config.Set(nil) })
	config.Set(&config.Config{Proxy: config.ProxyConfig{AllowHeaderOverride: true}})

	tests := []struct {
		name           string
		modeHeader     string
		modeParam      string
		expectedMode   ProxyMode
	}{
		{
			name:         "Smart scheduling from header",
			modeHeader:   "smart-scheduling",
			modeParam:    "",
			expectedMode: ModeSmartScheduling,
		},
		{
			name:         "Direct backend from header",
			modeHeader:   "direct-backend",
			modeParam:    "",
			expectedMode: ModeDirectBackend,
		},
		{
			name:         "Transparent proxy from header",
			modeHeader:   "transparent-proxy",
			modeParam:    "",
			expectedMode: ModeTransparentProxy,
		},
		{
			name:         "Transparent fast shortcut #tf",
			modeHeader:   "#tf",
			modeParam:    "",
			expectedMode: ModeTransparentFast,
		},
		{
			name:         "Fixed egress shortcut #j",
			modeHeader:   "#j",
			modeParam:    "",
			expectedMode: ModeFixedEgress,
		},
		{
			name:         "Fixed egress full name",
			modeHeader:   "fixed-egress",
			modeParam:    "",
			expectedMode: ModeFixedEgress,
		},
		{
			name:         "Mode from parameter when header is empty",
			modeHeader:   "",
			modeParam:    "direct-backend",
			expectedMode: ModeDirectBackend,
		},
		{
			name:         "Header takes priority over parameter",
			modeHeader:   "smart-scheduling",
			modeParam:    "direct-backend",
			expectedMode: ModeSmartScheduling,
		},
		{
			name:         "Default mode when not specified",
			modeHeader:   "",
			modeParam:    "",
			expectedMode: ModeDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试请求
			req := &http.Request{
				Header: make(http.Header),
				URL:    &url.URL{},
			}

			if tt.modeHeader != "" {
				req.Header.Set("X-Proxy-Mode", tt.modeHeader)
			}

			if tt.modeParam != "" {
				query := req.URL.Query()
				query.Add("proxy_mode", tt.modeParam)
				req.URL.RawQuery = query.Encode()
			}

			// 检测代理模式
			mode, _ := DetectProxyMode(req)

			// 验证结果
			if mode != tt.expectedMode {
				t.Errorf("DetectProxyMode() = %v, want %v", mode, tt.expectedMode)
			}
		})
	}
}

func TestDetectCacheControl(t *testing.T) {
	tests := []struct {
		name          string
		cacheRead     string
		cacheWrite    string
		qaSplit       string
		expectedRead  bool
		expectedWrite bool
		expectedSplit bool
	}{
		{
			name:          "All enabled with true",
			cacheRead:     "true",
			cacheWrite:    "true",
			qaSplit:       "true",
			expectedRead:  true,
			expectedWrite: true,
			expectedSplit: true,
		},
		{
			name:          "All enabled with enable",
			cacheRead:     "enable",
			cacheWrite:    "enable",
			qaSplit:       "enable",
			expectedRead:  true,
			expectedWrite: true,
			expectedSplit: true,
		},
		{
			name:          "All enabled with 1",
			cacheRead:     "1",
			cacheWrite:    "1",
			qaSplit:       "1",
			expectedRead:  true,
			expectedWrite: true,
			expectedSplit: true,
		},
		{
			name:          "All enabled with yes",
			cacheRead:     "yes",
			cacheWrite:    "yes",
			qaSplit:       "yes",
			expectedRead:  true,
			expectedWrite: true,
			expectedSplit: true,
		},
		{
			name:          "All disabled with false",
			cacheRead:     "false",
			cacheWrite:    "false",
			qaSplit:       "false",
			expectedRead:  false,
			expectedWrite: false,
			expectedSplit: false,
		},
		{
			name:          "All disabled with disable",
			cacheRead:     "disable",
			cacheWrite:    "disable",
			qaSplit:       "disable",
			expectedRead:  false,
			expectedWrite: false,
			expectedSplit: false,
		},
		{
			name:          "Mixed values",
			cacheRead:     "enable",
			cacheWrite:    "disable",
			qaSplit:       "true",
			expectedRead:  true,
			expectedWrite: false,
			expectedSplit: true,
		},
		{
			name:          "Empty headers use defaults",
			cacheRead:     "",
			cacheWrite:    "",
			qaSplit:       "",
			expectedRead:  true,
			expectedWrite: true,
			expectedSplit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试请求
			req := &http.Request{
				Header: make(http.Header),
			}

			if tt.cacheRead != "" {
				req.Header.Set("X-Cache-Read", tt.cacheRead)
			}

			if tt.cacheWrite != "" {
				req.Header.Set("X-Cache-Write", tt.cacheWrite)
			}

			if tt.qaSplit != "" {
				req.Header.Set("X-QA-Split", tt.qaSplit)
			}

			// 检测缓存控制
			cc := DetectCacheControl(req)

			// 验证结果
			if cc.Read != tt.expectedRead {
				t.Errorf("DetectCacheControl().Read = %v, want %v", cc.Read, tt.expectedRead)
			}

			if cc.Write != tt.expectedWrite {
				t.Errorf("DetectCacheControl().Write = %v, want %v", cc.Write, tt.expectedWrite)
			}

			if cc.QASplit != tt.expectedSplit {
				t.Errorf("DetectCacheControl().QASplit = %v, want %v", cc.QASplit, tt.expectedSplit)
			}
		})
	}
}

func TestDetectCacheControl_ReadWriteFromCacheNotFromCacheControl(t *testing.T) {
	t.Cleanup(func() { config.Set(nil) })

	cfg := &config.Config{
		Cache: config.CacheConfig{
			EnableCacheRead:  true,
			EnableCacheWrite: true,
			SaveOnlyMode:     false,
		},
		CacheControl: config.CacheControlConfig{
			Enabled:     true,
			DefaultRead: false,
			DefaultWrite: false,
		},
	}
	config.Set(cfg)

	req := &http.Request{Header: make(http.Header)}
	cc := DetectCacheControl(req)
	if !cc.Read || !cc.Write {
		t.Fatalf("Read=%v Write=%v, want both true (Cache master switches win over CacheControl defaults)", cc.Read, cc.Write)
	}
}

func TestDetectCacheControl_EnableCacheReadFalse(t *testing.T) {
	t.Cleanup(func() { config.Set(nil) })

	cfg := &config.Config{
		Cache: config.CacheConfig{
			EnableCacheRead:  false,
			EnableCacheWrite: true,
		},
		CacheControl: config.CacheControlConfig{
			Enabled:     true,
			DefaultRead: true,
		},
	}
	config.Set(cfg)

	req := &http.Request{Header: make(http.Header)}
	cc := DetectCacheControl(req)
	if cc.Read {
		t.Fatalf("Read=true, want false when EnableCacheRead is false")
	}
}

func TestIsValidProxyMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     ProxyMode
		expected bool
	}{
		{
			name:     "Valid smart scheduling mode",
			mode:     ModeSmartScheduling,
			expected: true,
		},
		{
			name:     "Valid direct backend mode",
			mode:     ModeDirectBackend,
			expected: true,
		},
		{
			name:     "Valid transparent proxy mode",
			mode:     ModeTransparentProxy,
			expected: true,
		},
		{
			name:     "Valid fixed egress mode",
			mode:     ModeFixedEgress,
			expected: true,
		},
		{
			name:     "Custom user pipeline id",
			mode:     ProxyMode("user-analytics"),
			expected: true,
		},
		{
			name:     "Empty mode",
			mode:     ProxyMode(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidProxyMode(tt.mode)
			if result != tt.expected {
				t.Errorf("IsValidProxyMode(%v) = %v, want %v", tt.mode, result, tt.expected)
			}
		})
	}
}

func TestParseBoolHeader(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue bool
		expected     bool
	}{
		{
			name:         "True value",
			value:        "true",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "Enable value",
			value:        "enable",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "Yes value",
			value:        "yes",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "One value",
			value:        "1",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "False value",
			value:        "false",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "Disable value",
			value:        "disable",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "Case insensitive true",
			value:        "TRUE",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "Case insensitive enable",
			value:        "ENABLE",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "Empty value returns default",
			value:        "",
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "Empty value returns default false",
			value:        "",
			defaultValue: false,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试请求
			req := &http.Request{
				Header: make(http.Header),
			}

			if tt.value != "" {
				req.Header.Set("Test-Header", tt.value)
			}

			// 测试解析
			result := parseBoolHeader(req, "Test-Header", tt.defaultValue)

			if result != tt.expected {
				t.Errorf("parseBoolHeader() = %v, want %v", result, tt.expected)
			}
		})
	}
}
