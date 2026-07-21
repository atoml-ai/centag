package pac

import (
	"fmt"
	"strings"

	"centag/core/pkg/config"
)

// Config PAC配置
type Config struct {
	ProxyHost    string   // 代理主机地址,如 "127.0.0.1"
	ProxyPort    int      // 代理端口
	Domains      []string // 大模型API域名列表
	PathPatterns []string // 路径模式列表
	CustomRules  []string // 自定义PAC规则
}

// DefaultConfig 默认配置（域名/路径与 system_proxy 默认值同源）
func DefaultConfig() *Config {
	return &Config{
		ProxyHost:    "127.0.0.1",
		ProxyPort:    20060,
		Domains:      append([]string(nil), config.DefaultMITMDomains()...),
		PathPatterns: append([]string(nil), config.DefaultMITMPathPatterns()...),
	}
}

// PACGenerator PAC文件生成器
type PACGenerator struct {
	config *Config
}

// NewPACGenerator 创建PAC生成器
func NewPACGenerator(config *Config) *PACGenerator {
	if config == nil {
		config = DefaultConfig()
	}
	return &PACGenerator{config: config}
}

// Generate 生成PAC文件内容
func (p *PACGenerator) Generate() string {
	var sb strings.Builder

	// 文件头注释
	sb.WriteString("/*\n")
	sb.WriteString(" * Centag PAC Configuration\n")
	sb.WriteString(" * Auto-generated configuration\n")
	sb.WriteString(" */\n\n")

	// 开始函数定义
	sb.WriteString("function FindProxyForURL(url, host) {\n")

	// 域名列表
	sb.WriteString("    // LLM API Domains\n")
	sb.WriteString("    var llm_domains = [\n")
	for i, domain := range p.config.Domains {
		sb.WriteString(fmt.Sprintf("        \"%s\"", domain))
		if i < len(p.config.Domains)-1 {
			sb.WriteString(",\n")
		} else {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("    ];\n\n")

	// 检查域名
	sb.WriteString("    // Check if host matches LLM API domains\n")
	sb.WriteString("    for (var i = 0; i < llm_domains.length; i++) {\n")
	sb.WriteString("        if (dnsDomainIs(host, llm_domains[i])) {\n")
	sb.WriteString(fmt.Sprintf("            return \"PROXY %s:%d\";\n", p.config.ProxyHost, p.config.ProxyPort))
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")

	// 路径模式检查
	sb.WriteString("    // Check URL path patterns\n")
	for _, pattern := range p.config.PathPatterns {
		sb.WriteString(fmt.Sprintf("    if (url.indexOf(\"%s\") !== -1) {\n", pattern))
		sb.WriteString(fmt.Sprintf("        return \"PROXY %s:%d\";\n", p.config.ProxyHost, p.config.ProxyPort))
		sb.WriteString("    }\n")
	}
	sb.WriteString("\n")

	// 自定义规则
	if len(p.config.CustomRules) > 0 {
		sb.WriteString("    // Custom rules\n")
		for _, rule := range p.config.CustomRules {
			sb.WriteString(fmt.Sprintf("    %s\n", rule))
		}
		sb.WriteString("\n")
	}

	// 默认直连
	sb.WriteString("    // Default: Direct connection\n")
	sb.WriteString("    return \"DIRECT\";\n")
	sb.WriteString("}\n")

	return sb.String()
}

// UpdateConfig 更新配置
func (p *PACGenerator) UpdateConfig(config *Config) {
	if config == nil {
		return
	}
	p.config = config
}

// AddDomain 添加域名
func (p *PACGenerator) AddDomain(domain string) {
	for _, d := range p.config.Domains {
		if d == domain {
			return
		}
	}
	p.config.Domains = append(p.config.Domains, domain)
}

// RemoveDomain 移除域名
func (p *PACGenerator) RemoveDomain(domain string) {
	newDomains := make([]string, 0, len(p.config.Domains))
	for _, d := range p.config.Domains {
		if d != domain {
			newDomains = append(newDomains, d)
		}
	}
	p.config.Domains = newDomains
}

// AddPathPattern 添加路径模式
func (p *PACGenerator) AddPathPattern(pattern string) {
	for _, pat := range p.config.PathPatterns {
		if pat == pattern {
			return
		}
	}
	p.config.PathPatterns = append(p.config.PathPatterns, pattern)
}

// RemovePathPattern 移除路径模式
func (p *PACGenerator) RemovePathPattern(pattern string) {
	newPatterns := make([]string, 0, len(p.config.PathPatterns))
	for _, pat := range p.config.PathPatterns {
		if pat != pattern {
			newPatterns = append(newPatterns, pat)
		}
	}
	p.config.PathPatterns = newPatterns
}

// GetDomains 获取域名列表
func (p *PACGenerator) GetDomains() []string {
	return p.config.Domains
}

// GetPathPatterns 获取路径模式列表
func (p *PACGenerator) GetPathPatterns() []string {
	return p.config.PathPatterns
}

// GenerateWithTemplate 使用模板生成PAC
func (p *PACGenerator) GenerateWithTemplate(template string) string {
	if template == "" {
		return p.Generate()
	}

	// 简单模板替换
	result := template
	result = strings.ReplaceAll(result, "{{PROXY_HOST}}", p.config.ProxyHost)
	result = strings.ReplaceAll(result, "{{PROXY_PORT}}", fmt.Sprintf("%d", p.config.ProxyPort))
	result = strings.ReplaceAll(result, "{{DOMAINS}}", p.generateDomainsArray())
	result = strings.ReplaceAll(result, "{{PATH_PATTERNS}}", p.generatePathPatterns())

	return result
}

// generateDomainsArray 生成域名数组JavaScript代码
func (p *PACGenerator) generateDomainsArray() string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, domain := range p.config.Domains {
		sb.WriteString(fmt.Sprintf("\"%s\"", domain))
		if i < len(p.config.Domains)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString("]")
	return sb.String()
}

// generatePathPatterns 生成路径模式检查代码
func (p *PACGenerator) generatePathPatterns() string {
	var sb strings.Builder
	for _, pattern := range p.config.PathPatterns {
		sb.WriteString(fmt.Sprintf("if (url.indexOf(\"%s\") !== -1) {\n", pattern))
		sb.WriteString(fmt.Sprintf("    return \"PROXY %s:%d\";\n", p.config.ProxyHost, p.config.ProxyPort))
		sb.WriteString("}\n")
	}
	return sb.String()
}
