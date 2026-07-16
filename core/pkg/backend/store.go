package backend

// BackendStore abstracts backend config persistence.
// Implementations may use a YAML file, database, or any other storage.
type BackendStore interface {
	Load() ([]*BackendConfig, error)
	Save(backends []*BackendConfig) error
}
