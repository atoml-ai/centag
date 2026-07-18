package scheduler

// PriceInfo 价格信息（USD/1M tokens）
type PriceInfo struct {
	InputPrice  float64 `json:"input_price"`  // 输入价格
	OutputPrice float64 `json:"output_price"` // 输出价格
	Tier        string  `json:"tier"`         // 价格等级：free/low/medium/high
	Currency    string  `json:"currency"`     // 货币单位（USD）
}

// ModelPriceTable 模型价格表。
//
// Deprecated: prefer centag/core/internal/billing.PricingService with YAML/DB rules.
// This table remains as a fallback when no pricing rule matches or PricingService is unset.
type ModelPriceTable struct {
	// 后端 ID -> 模型 ID -> 价格信息
	prices map[string]map[string]*PriceInfo
}

// NewModelPriceTable 创建价格表
func NewModelPriceTable() *ModelPriceTable {
	pt := &ModelPriceTable{
		prices: make(map[string]map[string]*PriceInfo),
	}
	pt.initDefaultPrices()
	return pt
}

// initDefaultPrices 初始化默认价格（USD / 1M tokens，与 config/pricing/default.yaml 对齐）
func (pt *ModelPriceTable) initDefaultPrices() {
	// ollama-local: 免费
	pt.SetPrice("ollama-local", "*", &PriceInfo{
		InputPrice:  0,
		OutputPrice: 0,
		Tier:        "free",
		Currency:    "USD",
	})

	// PPIO 平台价格
	pt.SetPrice("ppinfra", "deepseek-v3.2", &PriceInfo{
		InputPrice:  0.1389,
		OutputPrice: 0.1389,
		Tier:        "low",
		Currency:    "USD",
	})
	pt.SetPrice("ppinfra", "qwen3.5-plus", &PriceInfo{
		InputPrice:  0.2083,
		OutputPrice: 0.2083,
		Tier:        "low",
		Currency:    "USD",
	})
	pt.SetPrice("ppinfra", "glm-5", &PriceInfo{
		InputPrice:  0.6944,
		OutputPrice: 0.6944,
		Tier:        "medium",
		Currency:    "USD",
	})
	pt.SetPrice("ppinfra", "kimi-k2.5", &PriceInfo{
		InputPrice:  1.1111,
		OutputPrice: 1.1111,
		Tier:        "medium",
		Currency:    "USD",
	})
	pt.SetPrice("ppinfra", "minimax-m2.5", &PriceInfo{
		InputPrice:  0.2778,
		OutputPrice: 0.2778,
		Tier:        "low",
		Currency:    "USD",
	})

	// BigModel (智谱 AI) 价格
	pt.SetPrice("bigmodel", "glm-5", &PriceInfo{
		InputPrice:  2.7778,
		OutputPrice: 2.7778,
		Tier:        "high",
		Currency:    "USD",
	})
	pt.SetPrice("bigmodel", "glm-4.7", &PriceInfo{
		InputPrice:  0.6944,
		OutputPrice: 0.6944,
		Tier:        "medium",
		Currency:    "USD",
	})
	pt.SetPrice("bigmodel", "glm-4-flash", &PriceInfo{
		InputPrice:  0.0694,
		OutputPrice: 0.0694,
		Tier:        "low",
		Currency:    "USD",
	})
	pt.SetPrice("bigmodel", "glm-4-flashx", &PriceInfo{
		InputPrice:  0.1389,
		OutputPrice: 0.1389,
		Tier:        "low",
		Currency:    "USD",
	})
}

// SetPrice 设置价格
func (pt *ModelPriceTable) SetPrice(backendID, model string, price *PriceInfo) {
	if pt.prices[backendID] == nil {
		pt.prices[backendID] = make(map[string]*PriceInfo)
	}
	pt.prices[backendID][model] = price
}

// GetPrice 获取价格
func (pt *ModelPriceTable) GetPrice(backendID, model string) *PriceInfo {
	if backendPrices, ok := pt.prices[backendID]; ok {
		if price, ok := backendPrices[model]; ok {
			return price
		}
		// 尝试通配符匹配
		if price, ok := backendPrices["*"]; ok {
			return price
		}
	}
	// 返回默认价格（中等）
	return &PriceInfo{
		InputPrice:  0.7,
		OutputPrice: 0.7,
		Tier:        "medium",
		Currency:    "USD",
	}
}

// EstimateCost 估算成本（USD）
func (pt *ModelPriceTable) EstimateCost(backendID, model string, inputTokens, outputTokens int) float64 {
	price := pt.GetPrice(backendID, model)
	inputCost := float64(inputTokens) / 1_000_000 * price.InputPrice
	outputCost := float64(outputTokens) / 1_000_000 * price.OutputPrice
	return inputCost + outputCost
}

// GetPriceTier 获取价格等级
func (pt *ModelPriceTable) GetPriceTier(backendID, model string) string {
	price := pt.GetPrice(backendID, model)
	return price.Tier
}

// GetAllPrices 获取所有价格信息
func (pt *ModelPriceTable) GetAllPrices() map[string]map[string]*PriceInfo {
	return pt.prices
}
