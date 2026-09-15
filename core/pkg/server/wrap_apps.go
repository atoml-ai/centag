package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"centag/core/internal/agent"
	"centag/core/pkg/config"

	"github.com/gin-gonic/gin"
)

// ListWrapApps returns the proxy-launch app catalog (wrap_cli targets).
// Consumed by the desktop shell and the Web UI. Read-only: it only exposes
// catalog metadata; local install detection happens on the client.
func (s *Server) ListWrapApps(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"apps": wrapAppsPayload()})
}

// wrapAppsPayload returns the catalog, ensuring a non-nil slice so the API
// always encodes `"apps": []` rather than `null`.
func wrapAppsPayload() []agent.ProxyApp {
	apps := agent.ProxyApps(agent.NewTemplateRegistry())
	if apps == nil {
		apps = []agent.ProxyApp{}
	}
	return apps
}

func findWrapApp(id string) (agent.ProxyApp, bool) {
	id = strings.TrimSpace(id)
	for _, a := range wrapAppsPayload() {
		if a.ID == id {
			return a, true
		}
	}
	return agent.ProxyApp{}, false
}

// wrapPrepareRequest is optional; an empty body resolves the system default
// pipeline. Client-supplied argv is intentionally not accepted.
type wrapPrepareRequest struct {
	PipelineID  string `json:"pipeline_id"`
	BackendID   string `json:"backend_id"`
	Model       string `json:"model"`
	ViaProxy    bool   `json:"via_proxy"`
	WriteConfig bool   `json:"write_config"`
}

// PrepareWrapApp resolves the Centag model name for an app and, optionally,
// writes the app's local model config. Returns a launch plan; the client (tray
// / Web) performs the actual process launch via `centag wrap run`.
func (s *Server) PrepareWrapApp(c *gin.Context) {
	app, ok := findWrapApp(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown app id: " + c.Param("id")})
		return
	}

	var req wrapPrepareRequest
	// Body is optional (empty → system default); malformed JSON is a client error.
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	model, pipelineID := resolveWrapModel(req)

	warnings := []string{}
	restart := false
	env := map[string]string{}

	if req.WriteConfig {
		warn, needRestart := s.writeAppModelConfig(app, model)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		restart = needRestart
	}

	// Loopback MITM needs no token; LAN MITM auth uses CENTAG_WRAP_TOKEN or the
	// bound egress key (wrapTokenForRun covers both).
	token := s.wrapTokenForRun()

	c.JSON(http.StatusOK, gin.H{
		"ok":               true,
		"app_id":           app.ID,
		"display_name":     app.DisplayName,
		"launch_mode":      app.LaunchMode,
		"argv":             app.Argv,
		"model":            model,
		"pipeline_id":      pipelineID,
		"env":              env,
		"server":           s.localAPIBase(),
		"token":            token,
		"warnings":         warnings,
		"restart_required": restart,
	})
}

// resolveWrapModel maps the optional request target to a Centag model name.
// Defaults to the system default pipeline (transparent).
func resolveWrapModel(req wrapPrepareRequest) (model, pipelineID string) {
	if p := strings.TrimSpace(req.PipelineID); p != "" {
		return "centag/" + p, p
	}
	if b := strings.TrimSpace(req.BackendID); b != "" {
		m := strings.TrimSpace(req.Model)
		if m == "" {
			m = "gpt-4o"
		}
		return b + "/" + m, ""
	}
	p := config.DefaultSystemPipelineID
	return "centag/" + p, p
}

// writeAppModelConfig best-effort writes the app's local config so it uses the
// resolved Centag model. Returns (warning, restartRequired); a non-empty warning
// means the config was NOT written (caller still returns the launch plan).
func (s *Server) writeAppModelConfig(app agent.ProxyApp, model string) (string, bool) {
	if app.ModelConfig.AgentType == "" {
		return "该应用不支持本地写配置，已依赖透明模式映射模型", false
	}

	// system_proxy 模式（桌面 GUI/Chromium 应用）不应写入 loopback base_url：
	// Chromium 的 --proxy-server 指向 MITM (:8081)，若 base_url 写入
	// 127.0.0.1:20060（后端），请求会绕过 MITM 直达后端，导致 401。
	// 让应用保持默认 API 地址（如 https://api.openai.com），流量经 MITM 代理转换。
	if app.LaunchMode == "system_proxy" {
		return "桌面 GUI 应用使用系统代理模式，已跳过写配置（请通过 Web 页面手动配置模型）", false
	}

	tmpl, ok := agent.NewTemplateRegistry().Get(agent.AgentType(app.ModelConfig.AgentType))
	if !ok {
		return "未找到应用模板，已跳过写配置", false
	}
	key := s.wrapTokenForRun()
	if key == "" {
		key = s.systemProxyEgressKey()
	}
	if key == "" {
		return "未取到 Centag 出口 API Key，已跳过写配置（请在 Web 创建/绑定出口 Key）", false
	}
	host, port := "127.0.0.1", 20060
	if s.cfg != nil && s.cfg.Server.Port > 0 {
		port = s.cfg.Server.Port
	}
	info := &agent.BackendInfo{Name: "centag", Model: model, APIKey: key, Host: host, Port: port}
	if err := tmpl.WriteConfig(info); err != nil {
		return "写配置失败：" + err.Error(), false
	}
	meta := tmpl.Meta().Normalize()
	return "", meta.Category == agent.AgentCategoryDesktop
}

// wrapCheck is one readiness check in the doctor response.
type wrapCheck struct {
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

// WrapDoctor reports proxy readiness so the desktop shell can show actionable
// messages instead of raw errors.
func (s *Server) WrapDoctor(c *gin.Context) {
	checks := []wrapCheck{}

	// sidecar (this handler proves it is up)
	checks = append(checks, wrapCheck{ID: "sidecar", OK: true, Message: "sidecar 运行中"})

	// CA certificate present
	caPath := ""
	caOK := false
	if s.cfg != nil {
		caPath = config.ResolvePathRelativeToExecutable(s.cfg.SystemProxy.CACertPath)
		if st, err := os.Stat(caPath); err == nil && !st.IsDir() {
			caOK = true
		}
	}
	if caOK {
		checks = append(checks, wrapCheck{ID: "ca", OK: true, Message: "CA 证书存在: " + caPath})
	} else {
		checks = append(checks, wrapCheck{ID: "ca", OK: false, Message: "未找到 CA 证书",
			Action: "先启动 sidecar / 执行一次 wrap enable 生成并信任 CA"})
	}

	// MITM proxy enabled
	if s.cfg != nil && s.cfg.SystemProxy.Enabled {
		checks = append(checks, wrapCheck{ID: "mitm", OK: true, Message: "MITM 代理已启用"})
	} else {
		checks = append(checks, wrapCheck{ID: "mitm", OK: false, Message: "MITM 代理未启用",
			Action: "Web →「本机代理出口」开启 MITM"})
	}

	// egress key (needed for MITM to inject the upstream key)
	if s.systemProxyEgressKey() != "" {
		checks = append(checks, wrapCheck{ID: "egress_key", OK: true, Message: "出口 API Key 已配置"})
	} else {
		checks = append(checks, wrapCheck{ID: "egress_key", OK: false, Message: "出口 API Key 未配置",
			Action: "Web →「本机代理出口」一键绑定/创建出口 Key"})
	}

	// LAN consistency
	if s.cfg != nil && s.cfg.SystemProxy.AllowLANClients {
		if strings.TrimSpace(s.cfg.SystemProxy.AdvertiseHost) != "" {
			checks = append(checks, wrapCheck{ID: "lan", OK: true, Message: "LAN 已开启，advertise_host=" + s.cfg.SystemProxy.AdvertiseHost})
		} else {
			checks = append(checks, wrapCheck{ID: "lan", OK: false, Message: "LAN 已开启但未配置 advertise_host",
				Action: "Web →「本机代理出口」填写 advertise_host"})
		}
	} else {
		checks = append(checks, wrapCheck{ID: "lan", OK: true, Message: "仅本机模式（无需 Token）"})
	}

	// catalog + optional app id
	if id := strings.TrimSpace(c.Query("app_id")); id != "" {
		if _, ok := findWrapApp(id); ok {
			checks = append(checks, wrapCheck{ID: "app", OK: true, Message: "应用存在: " + id})
		} else {
			checks = append(checks, wrapCheck{ID: "app", OK: false, Message: "未知应用: " + id,
				Action: "调用 GET /api/v1/wrap/apps 获取可用应用"})
		}
	}

	// Certificate pinning detection
	if s.mitmServer != nil {
		if domain := strings.TrimSpace(c.Query("domain")); domain != "" {
			if s.mitmServer.CheckCertPinning(domain) {
				checks = append(checks, wrapCheck{
					ID:      "cert_pinning",
					OK:      false,
					Message: "疑似证书固定: " + domain,
					Action:  "该应用可能使用了证书固定，无法通过 MITM 代理。建议使用「写配置」方式配置模型",
				})
			} else {
				tlsFails, totalFails := s.mitmServer.GetDomainFailureStats(domain)
				if totalFails > 0 {
					checks = append(checks, wrapCheck{
						ID:      "cert_pinning",
						OK:      true,
						Message: fmt.Sprintf("域名 %s 有 %d 次失败（TLS: %d）", domain, totalFails, tlsFails),
					})
				}
			}
		}
	}

	ok := true
	for _, ch := range checks {
		if !ch.OK {
			ok = false
			break
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": ok, "checks": checks})
}

// systemProxyEgressKey returns the bound egress key, or "" when unset/nil cfg.
func (s *Server) systemProxyEgressKey() string {
	if s.cfg == nil {
		return ""
	}
	return strings.TrimSpace(config.ResolveSystemProxyEgressAPIKey(&s.cfg.SystemProxy))
}

// newWrapLocalGuard lets loopback clients call local control endpoints without
// auth (desktop shell), and falls back to proxyAuth for anything else.
func newWrapLocalGuard(proxyAuth gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isLoopbackIP(c.ClientIP()) {
			c.Next()
			return
		}
		proxyAuth(c)
	}
}
