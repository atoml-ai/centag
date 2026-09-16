package configsync

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"centag/core/pkg/database"
)

// SkillStore is the interface for remote skill persistence.
type SkillStore interface {
	Upsert(row RemoteSkillRow) error
	UpsertBatch(rows []RemoteSkillRow) error
	GetAll() ([]RemoteSkillRow, error)
	Delete(name string) error
	Clear() error
}

// DBSkillStore persists remote skill rows in the database (R13).
type DBSkillStore struct {
	db      *sql.DB
	dialect database.Dialect
}

// NewDBSkillStore creates a database-backed skill store.
func NewDBSkillStore() (*DBSkillStore, error) {
	mgr := database.Get()
	if mgr == nil || mgr.GetDB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var dialect database.Dialect = &database.SQLiteDialect{}
	if mgr.DriverName() == "postgresql" {
		dialect = &database.PostgreSQLDialect{}
	}
	return &DBSkillStore{db: mgr.GetDB(), dialect: dialect}, nil
}

func (s *DBSkillStore) ph(n int) string {
	if s.dialect != nil {
		return s.dialect.Placeholder(n)
	}
	d := database.SQLiteDialect{}
	return d.Placeholder(n)
}

// Upsert inserts or updates a single skill row.
func (s *DBSkillStore) Upsert(row RemoteSkillRow) error {
	toolsJSON, _ := json.Marshal(row.Tools)
	stepsJSON, _ := json.Marshal(row.Steps)

	query := fmt.Sprintf(`INSERT INTO remote_skills
		(name, description, category, tools, steps, system_prompt, version, edition, enabled, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
		ON CONFLICT(name) DO UPDATE SET
			description = EXCLUDED.description,
			category = EXCLUDED.category,
			tools = EXCLUDED.tools,
			steps = EXCLUDED.steps,
			system_prompt = EXCLUDED.system_prompt,
			version = EXCLUDED.version,
			edition = EXCLUDED.edition,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6),
		s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11))

	now := time.Now()
	enabled := 0
	if row.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(query, row.Name, row.Description, row.Category,
		string(toolsJSON), string(stepsJSON), row.SystemPrompt,
		row.Version, row.Edition, enabled, now, now)
	if err != nil {
		return fmt.Errorf("upsert skill %s: %w", row.Name, err)
	}
	return nil
}

// UpsertBatch inserts or updates multiple skill rows in a single transaction.
func (s *DBSkillStore) UpsertBatch(rows []RemoteSkillRow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin skill batch: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(fmt.Sprintf(`INSERT INTO remote_skills
		(name, description, category, tools, steps, system_prompt, version, edition, enabled, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
		ON CONFLICT(name) DO UPDATE SET
			description = EXCLUDED.description,
			category = EXCLUDED.category,
			tools = EXCLUDED.tools,
			steps = EXCLUDED.steps,
			system_prompt = EXCLUDED.system_prompt,
			version = EXCLUDED.version,
			edition = EXCLUDED.edition,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6),
		s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11)))
	if err != nil {
		return fmt.Errorf("prepare skill batch: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, row := range rows {
		toolsJSON, _ := json.Marshal(row.Tools)
		stepsJSON, _ := json.Marshal(row.Steps)
		enabled := 0
		if row.Enabled {
			enabled = 1
		}
		if _, err := stmt.Exec(row.Name, row.Description, row.Category,
			string(toolsJSON), string(stepsJSON), row.SystemPrompt,
			row.Version, row.Edition, enabled, now, now); err != nil {
			return fmt.Errorf("upsert skill %s: %w", row.Name, err)
		}
	}
	return tx.Commit()
}

// GetAll returns all persisted skill rows.
func (s *DBSkillStore) GetAll() ([]RemoteSkillRow, error) {
	rows, err := s.db.Query(`SELECT name, description, category, tools, steps, system_prompt, version, edition, enabled FROM remote_skills`)
	if err != nil {
		return nil, fmt.Errorf("query skills: %w", err)
	}
	defer rows.Close()

	var result []RemoteSkillRow
	for rows.Next() {
		var r RemoteSkillRow
		var toolsStr, stepsStr string
		var enabled int
		if err := rows.Scan(&r.Name, &r.Description, &r.Category,
			&toolsStr, &stepsStr, &r.SystemPrompt,
			&r.Version, &r.Edition, &enabled); err != nil {
			return nil, fmt.Errorf("scan skill: %w", err)
		}
		r.Enabled = enabled != 0
		_ = json.Unmarshal([]byte(toolsStr), &r.Tools)
		_ = json.Unmarshal([]byte(stepsStr), &r.Steps)
		result = append(result, r)
	}
	return result, rows.Err()
}

// Delete removes a skill by name.
func (s *DBSkillStore) Delete(name string) error {
	_, err := s.db.Exec(fmt.Sprintf("DELETE FROM remote_skills WHERE name = %s", s.ph(1)), name)
	return err
}

// Clear removes all skill rows.
func (s *DBSkillStore) Clear() error {
	_, err := s.db.Exec("DELETE FROM remote_skills")
	return err
}
