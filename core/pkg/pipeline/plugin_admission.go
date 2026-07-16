package pipeline

import (
	"fmt"
	"strings"
	"time"

	"centag/core/pkg/config"
)

type AdmissionResult struct {
	Passed      bool
	Score       int
	Issues      []AdmissionIssue
	Warnings    []string
	Timestamp   time.Time
	PluginName  string
	PluginKind  string
}

type AdmissionIssue struct {
	Severity string
	Category string
	Message  string
}

type AdmissionChecker struct {
	cfg config.PluginAdmissionConfig
}

func NewAdmissionChecker(cfg config.PluginAdmissionConfig) *AdmissionChecker {
	return &AdmissionChecker{cfg: cfg}
}

func (c *AdmissionChecker) IsEnabled() bool {
	return c.cfg.Enabled
}

func (c *AdmissionChecker) CheckPermissions(descriptor NodePluginDescriptor) *AdmissionResult {
	result := &AdmissionResult{
		Passed:     true,
		Score:     100,
		Timestamp: time.Now(),
	}

	if !c.cfg.CheckPermissions {
		return result
	}

	result.PluginName = descriptor.Implementation
	result.PluginKind = descriptor.Kind

	permissions := descriptor.Permissions

	criticalPermissions := []string{"*", "sudo", "root", "admin", "shell", "exec"}
	for _, perm := range permissions {
		lowerPerm := strings.ToLower(perm)
		for _, critical := range criticalPermissions {
			if lowerPerm == critical || strings.Contains(lowerPerm, critical) {
				result.Issues = append(result.Issues, AdmissionIssue{
					Severity: "high",
					Category: "permissions",
					Message:  fmt.Sprintf("Permission '%s' is too broad", perm),
				})
				result.Score -= 30
				result.Passed = false
			}
		}
	}

	if len(permissions) > 10 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Plugin requests %d permissions, consider reducing", len(permissions)))
		result.Score -= 10
	}

	if result.Score < 0 {
		result.Score = 0
	}

	return result
}

func (c *AdmissionChecker) CheckTimeout(timeoutSeconds int) *AdmissionResult {
	result := &AdmissionResult{
		Passed:     true,
		Score:     100,
		Timestamp: time.Now(),
	}

	if !c.cfg.CheckTimeout {
		return result
	}

	if timeoutSeconds > c.cfg.MaxTimeoutSeconds {
		result.Issues = append(result.Issues, AdmissionIssue{
			Severity: "high",
			Category: "timeout",
			Message:  fmt.Sprintf("Timeout %ds exceeds maximum %ds", timeoutSeconds, c.cfg.MaxTimeoutSeconds),
		})
		result.Score -= 25
		result.Passed = false
	}

	if timeoutSeconds < c.cfg.MinTimeoutSeconds {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Timeout %ds is below minimum %ds", timeoutSeconds, c.cfg.MinTimeoutSeconds))
		result.Score -= 10
	}

	if result.Score < 0 {
		result.Score = 0
	}

	return result
}

func (c *AdmissionChecker) CheckErrorHandling(descriptor NodePluginDescriptor) *AdmissionResult {
	result := &AdmissionResult{
		Passed:     true,
		Score:     100,
		Timestamp: time.Now(),
	}

	if !c.cfg.CheckErrorHandling {
		return result
	}

	hasRetry := false
	hasFallback := false
	for _, tag := range descriptor.Tags {
		if tag == "retry" {
			hasRetry = true
		}
		if tag == "fallback" {
			hasFallback = true
		}
	}

	if !hasRetry && !hasFallback {
		result.Warnings = append(result.Warnings, "Plugin does not specify retry or fallback strategy")
		result.Score -= 15
	}

	if descriptor.Version == "" || descriptor.Version == "unknown" {
		result.Warnings = append(result.Warnings, "Plugin version is not specified")
		result.Score -= 10
	}

	if descriptor.Implementation == "" {
		result.Issues = append(result.Issues, AdmissionIssue{
			Severity: "high",
			Category: "error_handling",
			Message:  "Plugin implementation identifier is missing",
		})
		result.Score -= 20
		result.Passed = false
	}

	if result.Score < 0 {
		result.Score = 0
	}

	return result
}

func (c *AdmissionChecker) CheckObservability(descriptor NodePluginDescriptor) *AdmissionResult {
	result := &AdmissionResult{
		Passed:     true,
		Score:     100,
		Timestamp: time.Now(),
	}

	if !c.cfg.CheckObservability {
		return result
	}

	hasMetrics := false
	hasLogging := false
	for _, tag := range descriptor.Tags {
		if tag == "metrics" {
			hasMetrics = true
		}
		if tag == "logging" {
			hasLogging = true
		}
	}

	if !hasMetrics {
		result.Warnings = append(result.Warnings, "Plugin does not expose metrics")
		result.Score -= 10
	}

	if !hasLogging {
		result.Warnings = append(result.Warnings, "Plugin does not expose logging configuration")
		result.Score -= 5
	}

	if result.Score < 0 {
		result.Score = 0
	}

	return result
}

func (c *AdmissionChecker) CheckSignature(descriptor NodePluginDescriptor, validator *PluginSecurityValidator) *AdmissionResult {
	result := &AdmissionResult{
		Passed:    true,
		Score:     100,
		Timestamp: time.Now(),
	}

	if validator == nil || !validator.IsEnabled() {
		return result
	}

	if err := validator.ValidateManifestSignature(descriptor); err != nil {
		result.Issues = append(result.Issues, AdmissionIssue{
			Severity: "high",
			Category: "signature",
			Message:  fmt.Sprintf("Signature validation failed: %s", err.Error()),
		})
		result.Score -= 40
		result.Passed = false
	}

	if result.Score < 0 {
		result.Score = 0
	}

	return result
}

func (c *AdmissionChecker) CheckAll(descriptor NodePluginDescriptor, timeoutSeconds int, validator *PluginSecurityValidator) *AdmissionResult {
	result := &AdmissionResult{
		Passed:      true,
		Score:       100,
		Issues:      []AdmissionIssue{},
		Warnings:    []string{},
		Timestamp:   time.Now(),
		PluginName:  descriptor.Implementation,
		PluginKind:  descriptor.Kind,
	}

	if !c.cfg.Enabled {
		result.Warnings = append(result.Warnings, "Admission check is disabled")
		return result
	}

	permResult := c.CheckPermissions(descriptor)
	result.merge(permResult)

	timeoutResult := c.CheckTimeout(timeoutSeconds)
	result.merge(timeoutResult)

	errResult := c.CheckErrorHandling(descriptor)
	result.merge(errResult)

	obsResult := c.CheckObservability(descriptor)
	result.merge(obsResult)

	sigResult := c.CheckSignature(descriptor, validator)
	result.merge(sigResult)

	if result.Score < 70 {
		result.Passed = false
	}

	return result
}

func (r *AdmissionResult) merge(other *AdmissionResult) {
	if !other.Passed {
		r.Passed = false
	}
	r.Score += other.Score
	if r.Score > 100 {
		r.Score = 100
	}
	r.Issues = append(r.Issues, other.Issues...)
	r.Warnings = append(r.Warnings, other.Warnings...)
}

func (r *AdmissionResult) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Admission Result for %s:\n", r.PluginName))
	sb.WriteString(fmt.Sprintf("  Passed: %v\n", r.Passed))
	sb.WriteString(fmt.Sprintf("  Score: %d/100\n", r.Score))
	if len(r.Issues) > 0 {
		sb.WriteString("  Issues:\n")
		for _, issue := range r.Issues {
			sb.WriteString(fmt.Sprintf("    - [%s] %s: %s\n", issue.Severity, issue.Category, issue.Message))
		}
	}
	if len(r.Warnings) > 0 {
		sb.WriteString("  Warnings:\n")
		for _, warn := range r.Warnings {
			sb.WriteString(fmt.Sprintf("    - %s\n", warn))
		}
	}
	return sb.String()
}