package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"centag/core/pkg/bootstrap"
	"centag/core/pkg/config"
)

// PipelineModeConfigChecker 流水线模式配置检查器
type PipelineModeConfigChecker struct {
	cfg *config.Config
}

// NewPipelineModeConfigChecker 创建配置检查器
func NewPipelineModeConfigChecker(cfg *config.Config) *PipelineModeConfigChecker {
	return &PipelineModeConfigChecker{cfg: cfg}
}

// ModeConfigCheckResult 模式配置检查结果
type ModeConfigCheckResult struct {
	Mode           string
	PipelineID     string
	TemplateFile   string
	Enabled        bool
	EnvVar         string
	EnvVarSet      bool
	EnvVarValue    string
	CanUsePipeline bool
	Issues         []string
}

// CheckAllModes 检查所有模式的配置
func (c *PipelineModeConfigChecker) CheckAllModes() []ModeConfigCheckResult {
	results := make([]ModeConfigCheckResult, 0, len(defaultModeMappings))

	for _, mapping := range defaultModeMappings {
		result := c.CheckMode(mapping.Mode)
		results = append(results, result)
	}

	return results
}

// CheckMode 检查单个模式的配置
func (c *PipelineModeConfigChecker) CheckMode(mode ProxyMode) ModeConfigCheckResult {
	result := ModeConfigCheckResult{
		Mode:   string(mode),
		Issues: make([]string, 0),
	}

	// 获取映射信息
	for _, mapping := range defaultModeMappings {
		if mapping.Mode == mode {
			result.PipelineID = mapping.PipelineID
			result.TemplateFile = fmt.Sprintf("%s-v2.yaml", mapping.PipelineID)
			result.EnvVar = c.getEnvVarForMode(mode)
			break
		}
	}

	// 检查环境变量
	result.EnvVarValue = os.Getenv(result.EnvVar)
	result.EnvVarSet = result.EnvVarValue != ""
	result.Enabled = c.isModeEnabled(mode)

	// 检查是否可以使用流水线
	result.CanUsePipeline = result.Enabled && result.PipelineID != ""

	// 检查潜在问题
	if result.Enabled {
		// 检查模板文件是否存在
		tmplPath := filepath.Join(bootstrap.InitdataRoot(), "pipeline-templates", result.TemplateFile)
		if _, err := os.Stat(tmplPath); err != nil {
			// 如果找不到文件，只在详细模式下报告
			// 因为文件可能在其他位置或运行时动态加载
			result.Issues = append(result.Issues, fmt.Sprintf("模板文件未在标准位置找到: %s (将在运行时从数据库加载)", tmplPath))
		}

		// 检查配置是否完整
		if c.cfg == nil {
			result.Issues = append(result.Issues, "配置未加载")
		}
	}

	return result
}

// isModeEnabled 检查模式是否启用
func (c *PipelineModeConfigChecker) isModeEnabled(mode ProxyMode) bool {
	if c.cfg == nil {
		return false
	}

	switch mode {
	case ModeAuditMode:
		return c.cfg.Proxy.ModeATemplateEnabled
	case ModeOptimizeMode:
		return c.cfg.Proxy.ModeOTemplateEnabled
	case ModeDirectBackend:
		return c.cfg.Proxy.ModeDTemplateEnabled
	case ModeTransparentProxy:
		return c.cfg.Proxy.ModeTTemplateEnabled
	case ModeFallback:
		return c.cfg.Proxy.ModeFTemplateEnabled
	case ModeModelMatching:
		return c.cfg.Proxy.ModeMTemplateEnabled
	case ModeIntentClassification:
		return c.cfg.Proxy.ModeCTemplateEnabled
	case ModePipeline:
		return c.cfg.Proxy.ModePTemplateEnabled
	default:
		return false
	}
}

// getEnvVarForMode 获取模式对应的环境变量名
func (c *PipelineModeConfigChecker) getEnvVarForMode(mode ProxyMode) string {
	switch mode {
	case ModeAuditMode:
		return "LLM_PROXY_MODE_A_TEMPLATE_ENABLED"
	case ModeOptimizeMode:
		return "LLM_PROXY_MODE_O_TEMPLATE_ENABLED"
	case ModeDirectBackend:
		return "LLM_PROXY_MODE_D_TEMPLATE_ENABLED"
	case ModeTransparentProxy:
		return "LLM_PROXY_MODE_T_TEMPLATE_ENABLED"
	case ModeFallback:
		return "LLM_PROXY_MODE_F_TEMPLATE_ENABLED"
	case ModeModelMatching:
		return "LLM_PROXY_MODE_M_TEMPLATE_ENABLED"
	case ModeIntentClassification:
		return "LLM_PROXY_MODE_C_TEMPLATE_ENABLED"
	case ModePipeline:
		return "LLM_PROXY_MODE_P_TEMPLATE_ENABLED"
	default:
		return ""
	}
}

// PrintReport 打印配置检查报告
func (c *PipelineModeConfigChecker) PrintReport() {
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("流水线模式配置检查报告")
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println()

	results := c.CheckAllModes()

	// 统计
	enabledCount := 0
	for _, r := range results {
		if r.Enabled {
			enabledCount++
		}
	}

	fmt.Printf("总模式数: %d\n", len(results))
	fmt.Printf("已启用: %d\n", enabledCount)
	fmt.Printf("未启用: %d\n", len(results)-enabledCount)
	fmt.Println()

	// 详细报告
	fmt.Println("详细配置:")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-8s %-20s %-8s %-8s %-12s %s\n", "模式", "流水线ID", "启用", "环境变量", "模板文件", "状态")
	fmt.Println(strings.Repeat("-", 80))

	for _, r := range results {
		enabledStr := "否"
		if r.Enabled {
			enabledStr = "是"
		}

		status := "OK"
		if r.Enabled && len(r.Issues) > 0 {
			status = "有问题"
		}

		envVarSetStr := "未设置"
		if r.EnvVarSet {
			envVarSetStr = "已设置"
		}

		fmt.Printf("%-8s %-20s %-8s %-8s %-12s %s\n",
			r.Mode,
			r.PipelineID,
			enabledStr,
			envVarSetStr,
			r.TemplateFile,
			status,
		)

		// 打印问题
		for _, issue := range r.Issues {
			fmt.Printf("  ⚠️  %s\n", issue)
		}
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Println()

	// 环境变量示例
	fmt.Println("环境变量配置示例:")
	fmt.Println(strings.Repeat("-", 80))
	for _, r := range results {
		if r.EnvVar == "" {
			continue
		}
		value := "false"
		if r.Enabled {
			value = "true"
		}
		fmt.Printf("export %s=%s\n", r.EnvVar, value)
	}
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println()

	// 使用说明
	if enabledCount == 0 {
		fmt.Println("💡 提示: 所有模式都使用硬编码实现。要启用流水线模板，请设置对应的环境变量。")
	} else if enabledCount == len(results) {
		fmt.Println("✅ 所有模式都已启用流水线模板！")
	} else {
		fmt.Printf("💡 提示: %d 个模式已启用流水线模板，%d 个模式仍使用硬编码实现。\n",
			enabledCount, len(results)-enabledCount)
	}
}

// GetEnabledModes 获取已启用的模式列表
func (c *PipelineModeConfigChecker) GetEnabledModes() []ProxyMode {
	var enabled []ProxyMode
	for _, mapping := range defaultModeMappings {
		if c.isModeEnabled(mapping.Mode) {
			enabled = append(enabled, mapping.Mode)
		}
	}
	return enabled
}

// GetDisabledModes 获取未启用的模式列表
func (c *PipelineModeConfigChecker) GetDisabledModes() []ProxyMode {
	var disabled []ProxyMode
	for _, mapping := range defaultModeMappings {
		if !c.isModeEnabled(mapping.Mode) {
			disabled = append(disabled, mapping.Mode)
		}
	}
	return disabled
}