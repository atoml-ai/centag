//go:build backend_azure

package azure

import "centag/core/pkg/plugin"

func init() {
	plugin.RegisterBackend("azure", func(config map[string]interface{}) (interface{}, error) {
		return NewBackend()
	})

	plugin.RegisterBackendMeta(plugin.BackendMeta{
		Type:           "azure",
		Name:           "Azure OpenAI",
		DefaultBaseURL: "https://{resource-name}.openai.azure.com",
		KeyHelp:        "Azure OpenAI API Key (Azure Portal → Azure AI → 密钥管理)",
		Capabilities:   []string{"chat", "streaming"},
		AuthSchemes:    []string{"api-key"},
	})
}
