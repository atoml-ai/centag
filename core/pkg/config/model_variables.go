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
		Name:        "system.default_backend",
		Value:       proxy.DefaultBackendID,
		Description: "默认后端：全局默认的 LLM 后端，未显式指定后端时作为兜底，一般用于主对话/生成等通用推理场景。",
		Category:    "system",
	})
	items = append(items, ModelVariableItem{
		Name:        "system.default_model",
		Value:       proxy.DefaultModel,
		Description: "默认模型：全局默认模型，通常与 system.default_backend 搭配使用，用于主对话/生成等通用推理场景。",
		Category:    "system",
	})
	items = append(items, ModelVariableItem{
		Name:        "system.fallback_backend",
		Value:       proxy.FallbackBackendID,
		Description: "备用后端：默认后端不可用（鉴权失败/限流/超时/报错）时自动切换的降级后端，用于提升服务可用性。",
		Category:    "system",
	})
	items = append(items, ModelVariableItem{
		Name:        "system.fallback_model",
		Value:       proxy.FallbackModel,
		Description: "备用模型：默认模型不可用时的降级模型，配合 system.fallback_backend 实现自动容错切换。",
		Category:    "system",
	})

	// 快速分类变量（用于 router 节点 LLM 意图分类），同样存储在 model_variables。
	classifyBackend := ""
	classifyModel := ""
	if v, ok := mv["system.classify_backend"]; ok {
		classifyBackend = v
	}
	if v, ok := mv["system.classify_model"]; ok {
		classifyModel = v
	}
	items = append(items, ModelVariableItem{
		Name:        "system.classify_backend",
		Value:       classifyBackend,
		Description: "快速分类后端：router 节点 LLM 意图分类（llm_classify / keyword_then_intent）专用的低成本/快速后端，以降低分类开销；未配置时回落默认后端。",
		Category:    "system",
	})
	items = append(items, ModelVariableItem{
		Name:        "system.classify_model",
		Value:       classifyModel,
		Description: "快速分类模型：router 节点 LLM 意图分类专用的小体积/低延迟模型，通常与 system.classify_backend 搭配；未配置时回落默认模型。",
		Category:    "system",
	})

	items = append(items, ModelVariableItem{
		Name:        "system.embedding_backend",
		Value:       embedding.BackendID,
		Description: "向量化后端：文本向量化（Embedding）调用使用的后端，用于语义检索、向量库写入与查询等场景。",
		Category:    "system",
	})
	items = append(items, ModelVariableItem{
		Name:        "system.embedding_model",
		Value:       embedding.Model,
		Description: "向量化模型：文本向量化模型，维度需与向量库索引一致，配合 system.embedding_backend 使用。",
		Category:    "system",
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
		Name:        "system.rerank_backend",
		Value:       rerankBackend,
		Description: "重排序后端：RAG 检索结果重排序（Rerank）使用的后端，对召回结果按相关性精排，提升命中质量。",
		Category:    "system",
	})
	items = append(items, ModelVariableItem{
		Name:        "system.rerank_model",
		Value:       rerankModel,
		Description: "重排序模型：重排序模型（通常为 cross-encoder 类），用于对向量召回结果打分精排，配合 system.rerank_backend 使用。",
		Category:    "system",
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
