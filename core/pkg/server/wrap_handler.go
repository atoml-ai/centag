package server

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

type wrapRunRequest struct {
	PresetID string   `json:"preset_id"`
	Argv     []string `json:"argv"`
	Command  string   `json:"command"`
	// OpenTerminal requests opening a system terminal (best-effort).
	// When false, only builds/returns the command (for copy).
	OpenTerminal *bool `json:"open_terminal"`
}

// ListWrapPresets returns CLI agents that can be launched via wrap run.
func (s *Server) ListWrapPresets(c *gin.Context) {
	if err := s.ensureWrapRunAllowed(c); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"presets": wrapPresets()})
}

// RunWrapAgent builds `centag wrap run -- <argv>` and optionally opens a terminal.
func (s *Server) RunWrapAgent(c *gin.Context) {
	if err := s.ensureWrapRunAllowed(c); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req wrapRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	argv := req.Argv
	if id := strings.TrimSpace(req.PresetID); id != "" {
		p, ok := wrapPresetByID(id)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown preset_id: " + id})
			return
		}
		argv = p.Argv
	}

	parsed, err := parseWrapArgv(argv, req.Command)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	serverURL := s.localAPIBase()
	token := s.wrapTokenForRun()
	userCmd, err := buildWrapRunUserCommand(serverURL, token, parsed)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	openTerm := true
	if req.OpenTerminal != nil {
		openTerm = *req.OpenTerminal
	}

	resp := gin.H{
		"ok":           true,
		"command":      userCmd,
		"user_command": userCmd,
		"argv":         parsed,
		"server":       serverURL,
		"opened":       false,
	}

	if !openTerm {
		c.JSON(http.StatusOK, resp)
		return
	}

	exe, err := resolveWrapExecutable()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":           true,
			"command":      userCmd,
			"user_command": userCmd,
			"argv":         parsed,
			"server":       serverURL,
			"opened":       false,
			"open_error":   err.Error(),
			"hint":         "请复制 command 到本机终端执行",
		})
		return
	}
	execCmd, err := buildWrapRunCommandLine(exe, serverURL, token, parsed)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "command": userCmd})
		return
	}
	resp["exec_command"] = execCmd

	if err := openSystemTerminal(execCmd); err != nil {
		logger.Warnf("wrap run: open terminal failed: %v", err)
		resp["opened"] = false
		resp["open_error"] = err.Error()
		resp["hint"] = "无法自动打开终端，请复制 command 手动执行"
		c.JSON(http.StatusOK, resp)
		return
	}

	resp["opened"] = true
	c.JSON(http.StatusOK, resp)
}

func (s *Server) ensureWrapRunAllowed(c *gin.Context) error {
	if s.edition.IsPersonal() || s.edition.IsMinimal() {
		return nil
	}
	if isLoopbackIP(c.ClientIP()) {
		return nil
	}
	return fmt.Errorf("wrap run is only available on personal/minimal edition or from loopback clients")
}

func (s *Server) localAPIBase() string {
	port := 20060
	if s.cfg != nil && s.cfg.Server.Port > 0 {
		port = s.cfg.Server.Port
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func (s *Server) wrapTokenForRun() string {
	if t := strings.TrimSpace(os.Getenv("CENTAG_WRAP_TOKEN")); t != "" {
		return t
	}
	if s.cfg == nil {
		return ""
	}
	if s.cfg.SystemProxy.AllowLANClients {
		return config.ResolveSystemProxyEgressAPIKey(&s.cfg.SystemProxy)
	}
	return ""
}
