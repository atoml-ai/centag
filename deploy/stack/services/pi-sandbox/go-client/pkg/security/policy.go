package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SecurityPolicy 定义安全策略
type SecurityPolicy struct {
	AllowedPaths  []string `json:"allowedPaths"`
	BlockedPaths  []string `json:"blockedPaths"`
	MaxFileSize   int64    `json:"maxFileSize"`
	AllowNetwork  bool     `json:"allowNetwork"`
	AllowedHosts  []string `json:"allowedHosts"`
	BlockedCommands []string `json:"blockedCommands"`
}

// DefaultSecurityPolicy 返回默认安全策略
func DefaultSecurityPolicy() *SecurityPolicy {
	return &SecurityPolicy{
		AllowedPaths: []string{"/workspace", "/tmp"},
		BlockedPaths: []string{"/etc", "/root", "/home", "/var", "/sys", "/proc"},
		MaxFileSize:  10 * 1024 * 1024, // 10MB
		AllowNetwork: true,
		AllowedHosts: []string{"api.anthropic.com", "api.openai.com", "generativelanguage.googleapis.com"},
		BlockedCommands: []string{
			"rm -rf /",
			"sudo",
			"chmod 777",
			"chown",
			"mkfs",
			"dd if=",
			":(){ :|:& };:",  // fork bomb
			"curl | sh",
			"wget | sh",
		},
	}
}

// ValidatePath 验证文件路径
func (sp *SecurityPolicy) ValidatePath(path string) error {
	// 规范化路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	
	// 检查是否在阻止列表中
	for _, blocked := range sp.BlockedPaths {
		if strings.HasPrefix(absPath, blocked) {
			return fmt.Errorf("path blocked: %s (matches blocked prefix: %s)", absPath, blocked)
		}
	}
	
	// 检查是否在允许列表中
	if len(sp.AllowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range sp.AllowedPaths {
			if strings.HasPrefix(absPath, allowedPath) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path not allowed: %s (not in allowed paths)", absPath)
		}
	}
	
	return nil
}

// ValidateCommand 验证命令
func (sp *SecurityPolicy) ValidateCommand(command string) error {
	// 检查是否包含阻止的命令
	lowerCmd := strings.ToLower(command)
	for _, blocked := range sp.BlockedCommands {
		if strings.Contains(lowerCmd, strings.ToLower(blocked)) {
			return fmt.Errorf("command blocked: contains '%s'", blocked)
		}
	}
	
	return nil
}

// ValidateFileSize 验证文件大小
func (sp *SecurityPolicy) ValidateFileSize(size int64) error {
	if size > sp.MaxFileSize {
		return fmt.Errorf("file too large: %d bytes (max: %d bytes)", size, sp.MaxFileSize)
	}
	return nil
}

// ValidateHost 验证网络主机
func (sp *SecurityPolicy) ValidateHost(host string) error {
	if !sp.AllowNetwork {
		return fmt.Errorf("network access disabled")
	}
	
	if len(sp.AllowedHosts) == 0 {
		return nil // 没有配置允许列表，允许所有
	}
	
	for _, allowed := range sp.AllowedHosts {
		if strings.Contains(host, allowed) {
			return nil
		}
	}
	
	return fmt.Errorf("host not allowed: %s", host)
}

// ToolCallValidator 工具调用验证器
type ToolCallValidator struct {
	policy *SecurityPolicy
}

// NewToolCallValidator 创建工具调用验证器
func NewToolCallValidator(policy *SecurityPolicy) *ToolCallValidator {
	return &ToolCallValidator{policy: policy}
}

// ValidateToolCall 验证工具调用
func (v *ToolCallValidator) ValidateToolCall(toolName string, args map[string]interface{}) error {
	switch toolName {
	case "read", "write", "edit":
		path, ok := args["path"].(string)
		if !ok {
			return fmt.Errorf("missing path argument")
		}
		return v.policy.ValidatePath(path)
		
	case "bash":
		cmd, ok := args["command"].(string)
		if !ok {
			return fmt.Errorf("missing command argument")
		}
		return v.policy.ValidateCommand(cmd)
		
	case "find", "grep", "ls":
		// 这些工具通常是安全的
		return nil
		
	default:
		// 未知工具，默认允许（可以根据需要改为拒绝）
		return nil
	}
}

// AuditLogger 审计日志记录器
type AuditLogger struct {
	enabled bool
	logFunc func(level, message string, fields map[string]interface{})
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(enabled bool, logFunc func(level, message string, fields map[string]interface{})) *AuditLogger {
	return &AuditLogger{
		enabled: enabled,
		logFunc: logFunc,
	}
}

// LogToolCall 记录工具调用
func (al *AuditLogger) LogToolCall(toolName string, args map[string]interface{}, allowed bool, reason string) {
	if !al.enabled || al.logFunc == nil {
		return
	}
	
	level := "info"
	if !allowed {
		level = "warn"
	}
	
	al.logFunc(level, "Tool call", map[string]interface{}{
		"tool":    toolName,
		"args":    args,
		"allowed": allowed,
		"reason":  reason,
	})
}

// LogPrompt 记录提示消息
func (al *AuditLogger) LogPrompt(message string) {
	if !al.enabled || al.logFunc == nil {
		return
	}
	
	al.logFunc("info", "User prompt", map[string]interface{}{
		"message": message,
	})
}

// LogResponse 记录响应
func (al *AuditLogger) LogResponse(message string) {
	if !al.enabled || al.logFunc == nil {
		return
	}
	
	al.logFunc("info", "Agent response", map[string]interface{}{
		"message": message,
	})
}

// LogError 记录错误
func (al *AuditLogger) LogError(err error) {
	if !al.enabled || al.logFunc == nil {
		return
	}
	
	al.logFunc("error", "Error occurred", map[string]interface{}{
		"error": err.Error(),
	})
}
