package strategy

import "time"

// BuiltinStrategy 内置策略描述（含实际权重）
type BuiltinStrategy struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Weights     WeightBreakdown `json:"weights"`
	IsBuiltin   bool           `json:"is_builtin"`
}

// WeightBreakdown 权重构成说明
type WeightBreakdown struct {
	NameSimilarity float64 `json:"name_similarity"` // 名称相似度权重
	CapacityMatch  float64 `json:"capacity_match"`  // 参数量匹配权重
	FamilyMatch    float64 `json:"family_match"`    // 家族匹配权重
}

// CustomStrategy 用户自定义策略
type CustomStrategy struct {
	ID          string          `json:"id"`           // 唯一标识（基于名称生成）
	Name        string          `json:"name"`         // 用户指定名称
	Description string          `json:"description"`  // 描述
	Weights     WeightBreakdown `json:"weights"`      // 权重配比
	Strictness  int             `json:"strictness"`   // 严格度 0-100
	Tolerance   float64         `json:"tolerance"`    // 参数量容忍度
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// StrategyListItem 策略列表项（内置+自定义统一结构）
type StrategyListItem struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Weights     WeightBreakdown `json:"weights"`
	IsBuiltin   bool            `json:"is_builtin"`
	Strictness  int             `json:"strictness,omitempty"`
	Tolerance   float64         `json:"tolerance,omitempty"`
	CreatedAt   *time.Time      `json:"created_at,omitempty"`
}

// BuiltinStrategies 内置策略定义（反映实际代码中的权重）
var BuiltinStrategies = []BuiltinStrategy{
	{
		ID:   "exact",
		Name: "精确匹配",
		Description: "仅匹配完全相同的模型名称。匹配最严格，无法匹配时直接拒绝请求。" +
			"适用于对模型版本有严格要求的场景。",
		Weights: WeightBreakdown{
			NameSimilarity: 1.0,
			CapacityMatch:  0.0,
			FamilyMatch:    0.0,
		},
		IsBuiltin: true,
	},
	{
		ID:   "family",
		Name: "家族匹配",
		Description: "优先匹配同一家族的模型（如 gpt-4 系列、qwen 系列）。" +
			"家族权重 70%，名称相似度 30%。允许在同系列不同版本间替代。",
		Weights: WeightBreakdown{
			NameSimilarity: 0.30,
			CapacityMatch:  0.00,
			FamilyMatch:    0.70,
		},
		IsBuiltin: true,
	},
	{
		ID:   "capacity",
		Name: "容量匹配",
		Description: "根据模型参数量（如 7B、13B、70B）进行匹配，优先选择参数量接近的模型。" +
			"容量权重 60%，名称相似度 30%，家族 10%。适用于注重性能规格的场景。",
		Weights: WeightBreakdown{
			NameSimilarity: 0.30,
			CapacityMatch:  0.60,
			FamilyMatch:    0.10,
		},
		IsBuiltin: true,
	},
	{
		ID:   "hybrid",
		Name: "混合匹配",
		Description: "综合名称相似度、参数量和家族三个维度进行评分。" +
			"默认权重：名称 50%，容量 30%，家族 20%。可通过自定义策略调整权重比例。",
		Weights: WeightBreakdown{
			NameSimilarity: 0.50,
			CapacityMatch:  0.30,
			FamilyMatch:    0.20,
		},
		IsBuiltin: true,
	},
}

// GetBuiltin 按 ID 查找内置策略
func GetBuiltin(id string) (*BuiltinStrategy, bool) {
	for i := range BuiltinStrategies {
		if BuiltinStrategies[i].ID == id {
			return &BuiltinStrategies[i], true
		}
	}
	return nil, false
}
