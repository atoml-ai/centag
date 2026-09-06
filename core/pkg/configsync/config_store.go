package configsync

import (
	"database/sql"
	"fmt"
	"time"

	"centag/core/pkg/database"
)

// DBConfigStore stores config data in the database.
type DBConfigStore struct {
	db      *sql.DB
	dialect database.Dialect
}

// NewDBConfigStore creates a database-backed config store.
func NewDBConfigStore() (*DBConfigStore, error) {
	mgr := database.Get()
	if mgr == nil || mgr.GetDB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var dialect database.Dialect = &database.SQLiteDialect{}
	if mgr.DriverName() == "postgresql" {
		dialect = &database.PostgreSQLDialect{}
	}
	return &DBConfigStore{db: mgr.GetDB(), dialect: dialect}, nil
}

// getDialect returns the SQL dialect, defaulting to SQLite for zero-value
// stores constructed directly (e.g. in tests).
func (s *DBConfigStore) getDialect() database.Dialect {
	if s.dialect != nil {
		return s.dialect
	}
	return &database.SQLiteDialect{}
}

// Upsert inserts or updates a config entry.
func (s *DBConfigStore) Upsert(key, value string) error {
	ph := func(n int) string { return s.getDialect().Placeholder(n) }
	query := fmt.Sprintf(`INSERT INTO config_store (config_key, config_value, created_at, updated_at)
		VALUES (%s, %s, %s, %s)
		ON CONFLICT(config_key) DO UPDATE SET
			config_value = EXCLUDED.config_value,
			updated_at = EXCLUDED.updated_at`, ph(1), ph(2), ph(3), ph(4))

	now := time.Now()
	_, err := s.db.Exec(query, key, value, now, now)
	if err != nil {
		return fmt.Errorf("failed to upsert config %s: %w", key, err)
	}
	return nil
}

// Get returns the value for a config key.
func (s *DBConfigStore) Get(key string) (string, error) {
	var value string
	query := fmt.Sprintf("SELECT config_value FROM config_store WHERE config_key = %s", s.getDialect().Placeholder(1))
	err := s.db.QueryRow(query, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// Count returns the number of config entries.
func (s *DBConfigStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM config_store").Scan(&count)
	return count, err
}

// ListAll returns all config entries.
func (s *DBConfigStore) ListAll() (map[string]string, error) {
	rows, err := s.db.Query("SELECT config_key, config_value FROM config_store")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, rows.Err()
}

// Clear removes all config entries.
func (s *DBConfigStore) Clear() error {
	_, err := s.db.Exec("DELETE FROM config_store")
	return err
}
