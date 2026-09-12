package configsync

// RemoteSkillRow 远程 agent skill 行（当前来源：飞书 Bitable skill 表）。
// server.BuiltinAgentHandler.ApplyRemoteSkills 消费该结构。
type RemoteSkillRow struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Tools        []string `json:"tools"`
	Steps        []string `json:"steps"`
	SystemPrompt string   `json:"system_prompt"`
	Version      string   `json:"version"`
	Edition      string   `json:"edition"` // 空/all=全版本；personal/team
	Enabled      bool     `json:"enabled"`
}
