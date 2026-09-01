package configsync

import (
	"database/sql"
	"fmt"
	"time"

	"centag/core/pkg/database"
)

// DBConfigStore stores config data in the database.
type DBConfigStore struct {
	db *sql.DB
}

// NewDBConfigStore creates a database-backed config store.
func NewDBConfigStore() (*DBConfigStore, error) {
	db := database.Get().GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return &DBConfigStore{db: db}, nil
}

// Upsert inserts or updates a config entry.
func (s *DBConfigStore) Upsert(key, value string) error {
	query := `INSERT INTO config_store (config_key, config_value, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(config_key) DO UPDATE SET
			config_value = excluded.config_value,
			updated_at = excluded.updated_at`

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
	err := s.db.QueryRow("SELECT config_value FROM config_store WHERE config_key = ?", key).Scan(&value)
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
