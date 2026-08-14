package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/pipeline"
)

// AgentBackendInfo 后端可用性信息
type AgentBackendInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Enabled   bool     `json:"enabled"`
	BaseURL   string   `json:"base_url"`
	Models    []string `json:"models"`
	HealthOK  bool     `json:"health_ok"`
	ProbeTime string   `json:"probe_time,omitempty"`
	Default   bool     `json:"default,omitempty"`
}

// AgentPipelineInfo 流水线信息
type AgentPipelineInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     bool   `json:"default,omitempty"`
}

// AgentDataProvider 向内置 Agent 提供 centag 运行时数据（避免 handler 依赖具体管理器类型）
type AgentDataProvider interface {
	// ListBackends 返回后端列表（含健康状态与支持模型）
	ListBackends(ctx context.Context) []AgentBackendInfo
	// ListPipelines 返回流水线列表
	ListPipelines(ctx context.Context) []AgentPipelineInfo
	// DefaultPipelineID 返回当前默认流水线
	DefaultPipelineID() string
	// DefaultBackendID 返回当前默认后端
	DefaultBackendID() string
}

// statusCheckReport 生成 status-check 分析报告
func statusCheckReport(p AgentDataProvider) string {
	var b strings.Builder

	b.WriteString("=== 系统状态检查报告 ===\n\n")

	// 后端检查
	b.WriteString("## 后端与模型可用性\n")
	backends := p.ListBackends(context.Background())
	if len(backends) == 0 {
		b.WriteString("- 未配置任何后端\n")
	} else {
		sort.Slice(backends, func(i, j int) bool { return backends[i].Name < backends[j].Name })
		for _, be := range backends {
			status := "❌ 不可用"
			if be.Enabled {
				status = "✅ 已启用"
			}
			if be.HealthOK {
				status += " / 健康"
			} else if be.Enabled {
				status += " / 未探测"
			}
			flag := ""
			if be.Default {
				flag = "（默认）"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s%s 类型=%s 地址=%s\n", status, be.Name, flag, be.Type, be.BaseURL))
			if len(be.Models) > 0 {
				b.WriteString(fmt.Sprintf("  支持模型: %s\n", strings.Join(be.Models, ", ")))
			} else {
				b.WriteString("  支持模型: （未配置/未获取）\n")
			}
			if be.ProbeTime != "" {
				b.WriteString(fmt.Sprintf("  最近探测: %s\n", be.ProbeTime))
			}
		}
	}

	// 流水线说明
	b.WriteString("\n## 当前流水线\n")
	pipelines := p.ListPipelines(context.Background())
	if len(pipelines) == 0 {
		b.WriteString("- 未配置任何流水线（使用系统默认透明模式）\n")
	} else {
		def := p.DefaultPipelineID()
		sort.Slice(pipelines, func(i, j int) bool { return pipelines[i].Name < pipelines[j].Name })
		for _, pl := range pipelines {
			flag := ""
			if pl.ID == def || pl.Default {
				flag = "（默认）"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s%s\n", pl.ID, pl.Name, flag))
			if pl.Description != "" {
				b.WriteString(fmt.Sprintf("  说明: %s\n", pl.Description))
			}
		}
	}

	// 默认配置汇总
	b.WriteString("\n## 默认配置\n")
	b.WriteString(fmt.Sprintf("- 默认后端: %s\n", orDash(p.DefaultBackendID())))
	b.WriteString(fmt.Sprintf("- 默认流水线: %s\n", orDash(p.DefaultPipelineID())))

	b.WriteString("\n---\n报告生成时间: " + time.Now().Format("2006-01-02 15:04:05"))
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// centagAgentDataProvider 基于 backend.Manager + pipeline.PipelineRegistry 实现 AgentDataProvider
type centagAgentDataProvider struct {
	backendMgr *backend.Manager
	pipeline   *pipeline.PipelineRegistry
	cfg        *config.Config
}

// NewAgentDataProvider 创建基于 centag 运行时数据的数据提供器
func NewAgentDataProvider(backendMgr *backend.Manager, pr *pipeline.PipelineRegistry, cfg *config.Config) AgentDataProvider {
	return &centagAgentDataProvider{backendMgr: backendMgr, pipeline: pr, cfg: cfg}
}

// ListBackends 返回后端列表
func (p *centagAgentDataProvider) ListBackends(ctx context.Context) []AgentBackendInfo {
	var out []AgentBackendInfo
	if p.backendMgr == nil {
		return out
	}
	defaultID := p.DefaultBackendID()
	for _, be := range p.backendMgr.GetAll() {
		var models []string
		for _, m := range be.SupportedModels {
			models = append(models, m.ActualModel)
		}
		info := AgentBackendInfo{
			ID:      be.ID,
			Name:    be.Name,
			Type:    be.Type,
			Enabled: be.Enabled,
			BaseURL: be.BaseURL,
			Models:  models,
			Default: be.ID != "" && be.ID == defaultID,
		}
		if be.HealthStatus != nil {
			info.HealthOK = strings.EqualFold(be.HealthStatus.Status, "healthy") || strings.EqualFold(be.HealthStatus.Status, "checking")
			if be.HealthStatus.LastCheckAt != "" {
				info.ProbeTime = be.HealthStatus.LastCheckAt
			}
		}
		out = append(out, info)
	}
	return out
}

// ListPipelines 返回流水线列表
func (p *centagAgentDataProvider) ListPipelines(ctx context.Context) []AgentPipelineInfo {
	var out []AgentPipelineInfo
	if p.pipeline == nil {
		return out
	}
	defaultID := p.DefaultPipelineID()
	for _, pl := range p.pipeline.List() {
		out = append(out, AgentPipelineInfo{
			ID:          pl.ID,
			Name:        pl.Name,
			Description: pl.Description,
			Default:     pl.ID == defaultID,
		})
	}
	return out
}

// DefaultPipelineID 返回默认流水线
func (p *centagAgentDataProvider) DefaultPipelineID() string {
	if p.cfg == nil {
		return ""
	}
	return p.cfg.Proxy.EffectiveDefaultPipeline()
}

// DefaultBackendID 返回默认后端
func (p *centagAgentDataProvider) DefaultBackendID() string {
	if p.cfg == nil {
		return ""
	}
	return p.cfg.Proxy.DefaultBackendID
}
