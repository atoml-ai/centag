//go:build backend_ollama

package ollama

import "centag/core/pkg/plugin"

func init() {
	plugin.RegisterBackend("ollama", func(config map[string]interface{}) (interface{}, error) {
		return NewBackend()
	})

	plugin.RegisterBackendMeta(plugin.BackendMeta{
		Type:           "ollama",
		Name:           "Ollama",
		DefaultBaseURL: "http://localhost:11434",
		KeyHelp:        "No API key required for local Ollama",
		Capabilities:   []string{"chat", "streaming", "embeddings"},
		AuthSchemes:    []string{"none"},
	})
}
