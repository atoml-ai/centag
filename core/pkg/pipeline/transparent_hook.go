package pipeline

// BackendEndpoint describes a resolved upstream API endpoint.
type BackendEndpoint struct {
	BaseURL string
	APIKey  string
}

// ResolveBackendEndpoint resolves backend_id to base URL and API key (wired in server).
var ResolveBackendEndpoint func(backendID string) (*BackendEndpoint, error)