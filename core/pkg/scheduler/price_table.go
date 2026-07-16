package scheduler

// PriceInfo 价格信息（元/1M tokens）
type PriceInfo struct {
	InputPrice  float64 `json:"input_price"`   // 输入价格
	OutputPrice float64 `json:"output_price"`  // 输出价格
	Tier        string  `json:"tier"`          // 价格等级：free/low/medium/high
	Currency    string  `json:"currency"`      // 货币单位
}

// ModelPriceTable 模型价格表
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

// initDefaultPrices 初始化默认价格（基于公开信息）
func (pt *ModelPriceTable) initDefaultPrices() {
	// ollama-local: 免费
	pt.SetPrice("ollama-local", "*", &PriceInfo{
		InputPrice:  0,
		OutputPrice: 0,
		Tier:        "free",
		Currency:    "CNY",
	})

	// PPIO 平台价格
	pt.SetPrice("ppinfra", "deepseek-v3.2", &PriceInfo{
		InputPrice:  1.0,
		OutputPrice: 1.0,
		Tier:        "low",
		Currency:    "CNY",
	})
	pt.SetPrice("ppinfra", "qwen3.5-plus", &PriceInfo{
		InputPrice:  1.5,
		OutputPrice: 1.5,
		Tier:        "low",
		Currency:    "CNY",
	})
	pt.SetPrice("ppinfra", "glm-5", &PriceInfo{
		InputPrice:  5.0,
		OutputPrice: 5.0,
		Tier:        "medium",
		Currency:    "CNY",
	})
	pt.SetPrice("ppinfra", "kimi-k2.5", &PriceInfo{
		InputPrice:  8.0,
		OutputPrice: 8.0,
		Tier:        "medium",
		Currency:    "CNY",
	})
	pt.SetPrice("ppinfra", "minimax-m2.5", &PriceInfo{
		InputPrice:  2.0,
		OutputPrice: 2.0,
		Tier:        "low",
		Currency:    "CNY",
	})

	// BigModel (智谱 AI) 价格
	pt.SetPrice("bigmodel", "glm-5", &PriceInfo{
		InputPrice:  20.0,
		OutputPrice: 20.0,
		Tier:        "high",
		Currency:    "CNY",
	})
	pt.SetPrice("bigmodel", "glm-4.7", &PriceInfo{
		InputPrice:  5.0,
		OutputPrice: 5.0,
		Tier:        "medium",
		Currency:    "CNY",
	})
	pt.SetPrice("bigmodel", "glm-4-flash", &PriceInfo{
		InputPrice:  0.5,
		OutputPrice: 0.5,
		Tier:        "low",
		Currency:    "CNY",
	})
	pt.SetPrice("bigmodel", "glm-4-flashx", &PriceInfo{
		InputPrice:  1.0,
		OutputPrice: 1.0,
		Tier:        "low",
		Currency:    "CNY",
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
		InputPrice:  5.0,
		OutputPrice: 5.0,
		Tier:        "medium",
		Currency:    "CNY",
	}
}

// EstimateCost 估算成本
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
