package ollama

// Config Ollama后端插件配置
type Config struct {
	BaseURL    string `json:"base_url" mapstructure:"base_url"`
	APIKey     string `json:"api_key" mapstructure:"api_key"`
	Timeout    int    `json:"timeout" mapstructure:"timeout"`
	MaxRetries int    `json:"max_retries" mapstructure:"max_retries"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		BaseURL:    "http://localhost:21434",
		APIKey:     "",
		Timeout:    120,
		MaxRetries: 3,
	}
}