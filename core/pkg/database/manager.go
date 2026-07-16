package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"centag/core/pkg/logger"
)

// ErrNotFound is returned by store methods when a requested record does not
// exist.
var ErrNotFound = errors.New("database: record not found")

// PluginFactory is the constructor signature that every database backend must
// register.  config is an opaque map so that each backend can define its own
// set of options without changing the interface.
type PluginFactory func(config map[string]interface{}) (DatabasePlugin, error)

// registry holds all registered database plugin factories, keyed by the
// plugin name (e.g. "postgresql").
var registry = struct {
	sync.RWMutex
	factories map[string]PluginFactory
}{
	factories: make(map[string]PluginFactory),
}

// RegisterPlugin makes a database plugin available for use.  It is designed to
// be called from an init() function inside each plugin package.
func RegisterPlugin(name string, factory PluginFactory) {
	registry.Lock()
	defer registry.Unlock()
	registry.factories[name] = factory
}

// Manager wraps the active DatabasePlugin and exposes store accessors.
// Only one Manager instance should exist per process; use Init / Get.
type Manager struct {
	plugin     DatabasePlugin
	driverName string // e.g. "postgresql" — same key passed to Init
}

var (
	globalManager *Manager
	initOnce      sync.Once
	initErr       error
)

// Init creates and initialises the global Manager using the named plugin.
// It runs all pending migrations before returning.
// Init is idempotent: subsequent calls are no-ops and return the original
// error (if any).
//
// When driverName is "auto", it attempts to connect using the following order:
// 1. postgresql - if LLM_PROXY_DB_DRIVER or PG_HOST/POSTGRES_HOST is set
// 2. sqlite - fallback when PostgreSQL is unavailable
func Init(ctx context.Context, driverName string, config map[string]interface{}) error {
	initOnce.Do(func() {
		drivers := resolveDrivers(driverName)
		logger.Infof("database: initializing with drivers: %v", drivers)

		for _, drv := range drivers {
			registry.RLock()
			factory, ok := registry.factories[drv]
			registry.RUnlock()

			if !ok {
				logger.Warnf("database: driver %s not found in registry", drv)
				continue
			}

			logger.Infof("database: attempting to initialize %s driver", drv)
			plugin, err := factory(config)
			if err != nil {
				// 记录连接失败的错误
				logger.Warnf("database: failed to initialize %s driver: %v", drv, err)
				continue
			}

			logger.Infof("database: attempting migration for %s driver", drv)
			if err := plugin.Migrate(ctx); err != nil {
				logger.Errorf("database: failed to migrate %s database: %v", drv, err)
				plugin.Close()
				continue
			}

			globalManager = &Manager{plugin: plugin, driverName: drv}
			initErr = nil
			logger.Infof("database: successfully initialized %s driver", drv)
			return
		}

		// 所有驱动都失败
		if globalManager == nil {
			initErr = fmt.Errorf("failed to initialize any database driver (tried: %v)", drivers)
			logger.Errorf("database: %v", initErr)
		}
	})

	return initErr
}

// resolveDrivers returns the list of drivers to try in order.
// - "auto" tries ["postgresql", "sqlite"]
// - "postgresql" tries ["postgresql", "sqlite"] (fallback to sqlite on failure)
// - "sqlite" tries ["sqlite"]
func resolveDrivers(driverName string) []string {
	switch driverName {
	case "auto":
		return []string{"postgresql", "sqlite"}
	case "postgresql":
		// 即使显式指定 postgresql，也降级到 sqlite
		return []string{"postgresql", "sqlite"}
	default:
		return []string{driverName}
	}
}

// DriverName returns the database plugin name passed to Init (e.g. "postgresql").
func (m *Manager) DriverName() string { return m.driverName }

// Get returns the global Manager. Returns nil if Init has not been called
// successfully, so callers should always check Init's error first or use IsInitialized().
func Get() *Manager {
	if globalManager == nil {
		logger.Error("database: manager not initialised – call database.Init first")
		return nil
	}
	return globalManager
}

// IsInitialized returns true if the database manager has been initialized.
// Use this to check if database is available before calling Get().
func IsInitialized() bool {
	return globalManager != nil
}

// Plugin exposes the underlying DatabasePlugin for advanced use.
func (m *Manager) Plugin() DatabasePlugin { return m.plugin }

// DBProvider is an optional interface that database plugins can implement
// to expose the underlying sql.DB connection.
type DBProvider interface {
	GetDB() *sql.DB
}

// GetDB returns the underlying sql.DB connection if the plugin implements
// the DBProvider interface. Returns nil otherwise.
// This is needed for services that require direct SQL access.
func (m *Manager) GetDB() *sql.DB {
	if provider, ok := m.plugin.(DBProvider); ok {
		return provider.GetDB()
	}
	return nil
}

// UserStore returns the UserStore from the active plugin.
func (m *Manager) UserStore() UserStore { return m.plugin.UserStore() }

// APIKeyStore returns the APIKeyStore from the active plugin.
func (m *Manager) APIKeyStore() APIKeyStore { return m.plugin.APIKeyStore() }

// RefreshTokenStore returns the RefreshTokenStore from the active plugin.
func (m *Manager) RefreshTokenStore() RefreshTokenStore { return m.plugin.RefreshTokenStore() }

// SystemConfigStore returns the SystemConfigStore from the active plugin.
func (m *Manager) SystemConfigStore() SystemConfigStore { return m.plugin.SystemConfigStore() }

// UserConfigStore returns the UserConfigStore from the active plugin.
func (m *Manager) UserConfigStore() UserConfigStore { return m.plugin.UserConfigStore() }

// ClashRuleStore returns the ClashRuleStore from the active plugin.
func (m *Manager) ClashRuleStore() ClashRuleStore { return m.plugin.ClashRuleStore() }

// TenantStore returns the TenantStore from the active plugin.
// Returns nil if the plugin does not support multi-tenant yet.
func (m *Manager) TenantStore() TenantStore { return m.plugin.TenantStore() }

// HealthCheck delegates to the active plugin.
func (m *Manager) HealthCheck(ctx context.Context) error { return m.plugin.HealthCheck(ctx) }

// Close shuts down the active plugin gracefully.
func (m *Manager) Close() error { return m.plugin.Close() }

// IsFirstRun returns true when no users exist in the database, which signals
// that the system needs to be initialised with default data.
func (m *Manager) IsFirstRun(ctx context.Context) (bool, error) {
	count, err := m.plugin.UserStore().Count(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// ListRegisteredPlugins returns the names of all registered database plugins.
// Useful for diagnostics and startup logging.
func ListRegisteredPlugins() []string {
	registry.RLock()
	defer registry.RUnlock()
	names := make([]string, 0, len(registry.factories))
	for name := range registry.factories {
		names = append(names, name)
	}
	return names
}

// helpers for callers that need zero-value defaults

// DefaultUserConfig returns an empty UserConfig with sensible JSON defaults.
func DefaultUserConfig(userID int64) *UserConfig {
	return &UserConfig{
		UserID:        userID,
		Backends:      "[]",
		ProxySettings: "{}",
		CacheSettings: "{}",
		Embedding:     "{}",
		QASplit:       "{}",
		PresetModes:   "[]",
		Scheduling:    "{}",
		CacheControl:  "{}",
		AuthSettings:  `{"require_api_key":false,"allow_no_auth":true}`,
		UpdatedAt:     time.Now().UTC(),
	}
}
