package skills

// SkillRegistry Skill注册表
type SkillRegistry struct {
	skills map[string]*Skill
}

// NewSkillRegistry 创建Skill注册表
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills: make(map[string]*Skill),
	}
}

// RegisterSkill 注册Skill
func (r *SkillRegistry) RegisterSkill(skill *Skill) {
	r.skills[skill.Name] = skill
}

// GetSkill 获取Skill
func (r *SkillRegistry) GetSkill(name string) (*Skill, bool) {
	skill, ok := r.skills[name]
	return skill, ok
}

// ListSkills 列出所有Skill
func (r *SkillRegistry) ListSkills() []*Skill {
	var result []*Skill
	for _, skill := range r.skills {
		if skill.Enabled {
			result = append(result, skill)
		}
	}
	return result
}

// IsSkillAllowed 检查Skill是否允许使用
func (r *SkillRegistry) IsSkillAllowed(name string, internalOnly bool) bool {
	skill, ok := r.skills[name]
	if !ok {
		return false
	}
	
	if internalOnly && !skill.Internal {
		return false
	}
	
	return skill.Enabled
}

// LoadBuiltinSkills 加载内置Skill
func LoadBuiltinSkills(registry *SkillRegistry) {
	// 注册内置Skill
	registry.RegisterSkill(NewStatusCheckSkill())
	registry.RegisterSkill(NewConfigAnalysisSkill())
	registry.RegisterSkill(NewErrorDiagnosisSkill())
	registry.RegisterSkill(NewLogAnalysisSkill())
	registry.RegisterSkill(NewStrategyRecommendSkill())
	registry.RegisterSkill(NewBillingAuditSkill())
	registry.RegisterSkill(NewCostAnalysisSkill())
}