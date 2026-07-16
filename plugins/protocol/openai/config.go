package openai

// Config OpenAI 协议插件配置
type Config struct {
	Enabled       bool     `json:"enabled" mapstructure:"enabled"`
	MaxMessages   int      `json:"max_messages" mapstructure:"max_messages"`
	MaxTokens     int      `json:"max_tokens" mapstructure:"max_tokens"`
	AllowedModels []string `json:"allowed_models" mapstructure:"allowed_models"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:       true,
		MaxMessages:   20,
		MaxTokens:     4096,
		AllowedModels: []string{"gpt-4", "gpt-4-turbo", "qwen/qwen3-4b-fp8"},
	}
}
