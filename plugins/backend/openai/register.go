//go:build backend_openai

package openai

import "centag/core/pkg/plugin"

func init() {
	plugin.RegisterBackend("openai", func(config map[string]interface{}) (interface{}, error) {
		return NewBackend()
	})

	plugin.RegisterBackendMeta(plugin.BackendMeta{
		Type:           "openai",
		Name:           "OpenAI",
		DefaultBaseURL: "https://api.openai.com/v1",
		KeyHelp:        "OpenAI API Key (sk-...)",
		Capabilities:   []string{"chat", "embeddings", "streaming", "function_calling"},
		AuthSchemes:    []string{"bearer"},
	})
}
