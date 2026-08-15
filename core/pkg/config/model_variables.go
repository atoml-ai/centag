package config

// ModelVariables holds system-level and user-defined template variables
// that can be referenced via {{system.*}} and {{user.*}} in pipeline configs.
type ModelVariables struct {
	SystemVariables map[string]string `json:"system_variables"`
	UserVariables   map[string]string `json:"user_variables"`
}

// ModelVariableItem is the API representation of a single variable.
type ModelVariableItem struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Category    string `json:"category"` // "system" | "user"
}

// DefaultModelVariables returns the initial model variables config.
// Only rerank-related variables have non-empty defaults; the rest are
// derived from existing Proxy/Embedding config fields at resolve time.
func DefaultModelVariables() ModelVariables {
	return ModelVariables{
		SystemVariables: map[string]string{},
		UserVariables:   map[string]string{},
	}
}

// ListSystemVariables returns the full list of system variable definitions
// with descriptions, pulling current values from the relevant config fields.
func ListSystemVariables(cfg *Config) []ModelVariableItem {
	if cfg == nil {
		return nil
	}
	proxy := cfg.Proxy
	embedding := cfg.Embedding
	mv := cfg.ModelVariables.SystemVariables

	var items []ModelVariableItem

	items = append(items, ModelVariableItem{
		Name: "system.default_backend", Value: proxy.DefaultBackendID,
		Description: "默认后端", Category: "system",
	})
	items = append(items, ModelVariableItem{
		Name: "system.default_model", Value: proxy.DefaultModel,
		Description: "默认模型", Category: "system",
	})
	items = append(items, ModelVariableItem{
		Name: "system.fallback_backend", Value: proxy.FallbackBackendID,
		Description: "备用后端", Category: "system",
	})
	items = append(items, ModelVariableItem{
		Name: "system.fallback_model", Value: proxy.FallbackModel,
		Description: "备用模型", Category: "system",
	})
	items = append(items, ModelVariableItem{
		Name: "system.embedding_backend", Value: embedding.BackendID,
		Description: "向量化后端", Category: "system",
	})
	items = append(items, ModelVariableItem{
		Name: "system.embedding_model", Value: embedding.Model,
		Description: "向量化模型", Category: "system",
	})

	// Rerank variables are stored in model_variables (no dedicated config struct yet).
	rerankBackend := ""
	rerankModel := ""
	if v, ok := mv["system.rerank_backend"]; ok {
		rerankBackend = v
	}
	if v, ok := mv["system.rerank_model"]; ok {
		rerankModel = v
	}
	items = append(items, ModelVariableItem{
		Name: "system.rerank_backend", Value: rerankBackend,
		Description: "重排序后端", Category: "system",
	})
	items = append(items, ModelVariableItem{
		Name: "system.rerank_model", Value: rerankModel,
		Description: "重排序模型", Category: "system",
	})

	return items
}

// ListUserVariables returns user-defined variables as ModelVariableItems.
func ListUserVariables(cfg *Config) []ModelVariableItem {
	if cfg == nil {
		return nil
	}
	var items []ModelVariableItem
	for k, v := range cfg.ModelVariables.UserVariables {
		items = append(items, ModelVariableItem{
			Name: k, Value: v, Category: "user",
		})
	}
	return items
}
