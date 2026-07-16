package agent

import (
	"sort"
	"sync"
)

// ============================================================================
// Agent 专业化注册表 (Task 2.7)
// ============================================================================

// AgentSpecialization Agent 专业化描述。
// 每个 Agent 类型可注册其类别、能力和元数据，
// 供调度系统按能力匹配选择合适的 Agent。
type AgentSpecialization struct {
	Type         AgentType            `json:"type"`
	Category     AgentCategory        `json:"category"`
	Capabilities []string             `json:"capabilities"`
	Metadata     map[string]string    `json:"metadata,omitempty"`
}

// SpecializedAgentRegistry Agent 专业化注册表。
// 管理所有 Agent 类型的能力声明和接口实例，
// 支持按类别、能力等多维度查询。
type SpecializedAgentRegistry struct {
	mu              sync.RWMutex
	specializations map[AgentType]*AgentSpecialization
	tuiAgents       map[AgentType]TUIAgent
	webAgents       map[AgentType]WebAgent
}

// NewSpecializedAgentRegistry 创建专业化注册表
func NewSpecializedAgentRegistry() *SpecializedAgentRegistry {
	return &SpecializedAgentRegistry{
		specializations: make(map[AgentType]*AgentSpecialization),
		tuiAgents:       make(map[AgentType]TUIAgent),
		webAgents:       make(map[AgentType]WebAgent),
	}
}

// --- 注册 ---

// Register 注册 Agent 专业化声明
func (r *SpecializedAgentRegistry) Register(spec *AgentSpecialization) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.specializations[spec.Type] = spec
}

// RegisterTUIAgent 注册 TUI Agent 实现
func (r *SpecializedAgentRegistry) RegisterTUIAgent(agentType AgentType, agent TUIAgent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tuiAgents[agentType] = agent
}

// RegisterWebAgent 注册 Web Agent 实现
func (r *SpecializedAgentRegistry) RegisterWebAgent(agentType AgentType, agent WebAgent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webAgents[agentType] = agent
}

// --- 查询 ---

// Get 按 AgentType 获取专业化声明
func (r *SpecializedAgentRegistry) Get(agentType AgentType) (*AgentSpecialization, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.specializations[agentType]
	return spec, ok
}

// GetTUIAgent 按 AgentType 获取 TUI Agent 实现
func (r *SpecializedAgentRegistry) GetTUIAgent(agentType AgentType) (TUIAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.tuiAgents[agentType]
	return agent, ok
}

// GetWebAgent 按 AgentType 获取 Web Agent 实现
func (r *SpecializedAgentRegistry) GetWebAgent(agentType AgentType) (WebAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.webAgents[agentType]
	return agent, ok
}

// List 列出所有专业化声明
func (r *SpecializedAgentRegistry) List() []*AgentSpecialization {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*AgentSpecialization, 0, len(r.specializations))
	for _, spec := range r.specializations {
		result = append(result, spec)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Type < result[j].Type
	})
	return result
}

// ListByCategory 按 AgentCategory 过滤列出专业化
func (r *SpecializedAgentRegistry) ListByCategory(category AgentCategory) []*AgentSpecialization {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*AgentSpecialization
	for _, spec := range r.specializations {
		if spec.Category == category {
			result = append(result, spec)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Type < result[j].Type
	})
	return result
}

// DiscoverCapabilities 按能力名称发现所有具备该能力的 Agent
func (r *SpecializedAgentRegistry) DiscoverCapabilities(capability string) []*AgentSpecialization {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*AgentSpecialization
	for _, spec := range r.specializations {
		for _, cap := range spec.Capabilities {
			if cap == capability {
				result = append(result, spec)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Type < result[j].Type
	})
	return result
}

// GetAllTUIAgents 返回所有已注册的 TUI Agent
func (r *SpecializedAgentRegistry) GetAllTUIAgents() map[AgentType]TUIAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[AgentType]TUIAgent, len(r.tuiAgents))
	for k, v := range r.tuiAgents {
		result[k] = v
	}
	return result
}

// GetAllWebAgents 返回所有已注册的 Web Agent
func (r *SpecializedAgentRegistry) GetAllWebAgents() map[AgentType]WebAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[AgentType]WebAgent, len(r.webAgents))
	for k, v := range r.webAgents {
		result[k] = v
	}
	return result
}

// Count 返回注册的 Agent 总数
func (r *SpecializedAgentRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.specializations)
}

// CountByCategory 返回指定类别的 Agent 数量
func (r *SpecializedAgentRegistry) CountByCategory(category AgentCategory) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, spec := range r.specializations {
		if spec.Category == category {
			count++
		}
	}
	return count
}

// SeedDefaults 注册内置 TUI/Web Agent 及对应的专业化声明
func (r *SpecializedAgentRegistry) SeedDefaults() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// --- TUI Agents ---
	codingTUI := NewCodingTUIAgent()
	eduTUI := NewEducationTUIAgent()
	r.tuiAgents[AgentCodingTUI] = codingTUI
	r.tuiAgents[AgentEducationTUI] = eduTUI

	// --- Web Agents ---
	codingWeb := NewCodingWebAgent(nil)
	eduWeb := NewEducationWebAgent(nil)
	r.webAgents[AgentCodingWeb] = codingWeb
	r.webAgents[AgentEducationWeb] = eduWeb

	// --- Specialization Declarations ---
	specs := []*AgentSpecialization{
		{
			Type:     AgentCodingTUI,
			Category: AgentCategoryTUI,
			Capabilities: []string{
				"code_highlight", "diff_view", "progress_tracking",
				"interactive_choice", "notification", "file_browsing",
			},
			Metadata: map[string]string{
				"scene":    "coding",
				"domain":   "programming",
				"requires": "terminal",
			},
		},
		{
			Type:     AgentEducationTUI,
			Category: AgentCategoryTUI,
			Capabilities: []string{
				"progress_tracking", "learning_content", "quiz_interaction",
				"notification", "interactive_choice", "status_display",
			},
			Metadata: map[string]string{
				"scene":    "education",
				"domain":   "learning",
				"requires": "terminal",
			},
		},
		{
			Type:     AgentCodingWeb,
			Category: AgentCategoryWeb,
			Capabilities: []string{
				"browser_automation", "screenshot", "javascript_execution",
				"code_preview", "documentation_browsing", "form_filling",
				"element_interaction", "web_navigation",
			},
			Metadata: map[string]string{
				"scene":    "coding",
				"domain":   "web_programming",
				"requires": "browser_engine",
			},
		},
		{
			Type:     AgentEducationWeb,
			Category: AgentCategoryWeb,
			Capabilities: []string{
				"browser_automation", "online_learning", "quiz_taking",
				"course_navigation", "content_extraction", "screenshot",
				"form_filling", "web_navigation",
			},
			Metadata: map[string]string{
				"scene":    "education",
				"domain":   "online_learning",
				"requires": "browser_engine",
			},
		},
	}

	for _, spec := range specs {
		if _, exists := r.specializations[spec.Type]; !exists {
			r.specializations[spec.Type] = spec
		}
	}
}
