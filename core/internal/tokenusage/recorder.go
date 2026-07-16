package tokenusage

// defaultService is wired from server startup for pipeline TokenUsageNode persistence.
var defaultService *Service

// SetDefaultService registers the process-wide token usage service.
func SetDefaultService(s *Service) {
	defaultService = s
}

// DefaultService returns the registered token usage service, if any.
func DefaultService() *Service {
	return defaultService
}