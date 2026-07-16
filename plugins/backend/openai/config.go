package openai

// Config OpenAI 后端插件配置
type Config struct {
	APIKey     string `json:"api_key" mapstructure:"api_key"`
	BaseURL    string `json:"base_url" mapstructure:"base_url"`
	Timeout    int    `json:"timeout" mapstructure:"timeout"`
	MaxRetries int    `json:"max_retries" mapstructure:"max_retries"`
	RetryDelay int    `json:"retry_delay" mapstructure:"retry_delay"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		BaseURL:    "https://api.openai.com/v1",
		Timeout:    60,
		MaxRetries: 3,
		RetryDelay: 1,
	}
}
