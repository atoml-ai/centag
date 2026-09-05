package pipeline

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"centag/core/pkg/configsync"
	"centag/core/pkg/database"
)

// DBPipelineTemplateStore stores pipeline templates in the database.
type DBPipelineTemplateStore struct {
	db      *sql.DB
	dialect database.Dialect
}

// NewDBPipelineTemplateStore creates a database-backed pipeline template store.
func NewDBPipelineTemplateStore() (*DBPipelineTemplateStore, error) {
	mgr := database.Get()
	if mgr == nil || mgr.GetDB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var dialect database.Dialect = &database.SQLiteDialect{}
	if mgr.DriverName() == "postgresql" {
		dialect = &database.PostgreSQLDialect{}
	}
	return &DBPipelineTemplateStore{db: mgr.GetDB(), dialect: dialect}, nil
}

// getDialect returns the SQL dialect, defaulting to SQLite for zero-value
// stores constructed directly (e.g. in tests).
func (s *DBPipelineTemplateStore) getDialect() database.Dialect {
	if s.dialect != nil {
		return s.dialect
	}
	return &database.SQLiteDialect{}
}

// Upsert inserts or updates a pipeline template.
func (s *DBPipelineTemplateStore) Upsert(tmpl configsync.PipelineTemplate) error {
	nodesJSON, err := json.Marshal(tmpl.Nodes)
	if err != nil {
		return fmt.Errorf("failed to marshal nodes: %w", err)
	}

	var globalConfigJSON []byte
	if tmpl.GlobalConfig != nil {
		globalConfigJSON, err = json.Marshal(tmpl.GlobalConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal global_config: %w", err)
		}
	}

	var metadataJSON []byte
	if tmpl.Metadata != nil {
		metadataJSON, err = json.Marshal(tmpl.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// Default edition to "all" if empty
	edition := tmpl.Edition
	if edition == "" {
		edition = "all"
	}

	ph := make([]string, 12)
	for i := range ph {
		ph[i] = s.getDialect().Placeholder(i + 1)
	}
	query := fmt.Sprintf(`INSERT INTO pipeline_templates (id, name, description, shortcut_code, schema_version, version, edition, nodes, global_config, metadata, created_at, updated_at)
		VALUES (%s)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			shortcut_code = excluded.shortcut_code,
			schema_version = excluded.schema_version,
			version = excluded.version,
			edition = excluded.edition,
			nodes = excluded.nodes,
			global_config = excluded.global_config,
			metadata = excluded.metadata,
			updated_at = excluded.updated_at`, strings.Join(ph, ", "))

	now := time.Now()
	_, err = s.db.Exec(query, tmpl.ID, tmpl.Name, tmpl.Description, tmpl.ShortcutCode, tmpl.SchemaVersion, tmpl.Version, edition, nodesJSON, globalConfigJSON, metadataJSON, now, now)
	if err != nil {
		return fmt.Errorf("failed to upsert pipeline template %s: %w", tmpl.ID, err)
	}
	return nil
}

// Count returns the number of pipeline templates in the database.
func (s *DBPipelineTemplateStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM pipeline_templates").Scan(&count)
	return count, err
}

// ListAll returns all pipeline templates from the database.
func (s *DBPipelineTemplateStore) ListAll() ([]configsync.PipelineTemplate, error) {
	rows, err := s.db.Query("SELECT id, name, description, shortcut_code, schema_version, version, edition, nodes, global_config, metadata FROM pipeline_templates")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []configsync.PipelineTemplate
	for rows.Next() {
		var tmpl configsync.PipelineTemplate
		var nodesJSON []byte
		var globalConfigJSON, metadataJSON []byte

		err := rows.Scan(&tmpl.ID, &tmpl.Name, &tmpl.Description, &tmpl.ShortcutCode, &tmpl.SchemaVersion, &tmpl.Version, &tmpl.Edition, &nodesJSON, &globalConfigJSON, &metadataJSON)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(nodesJSON, &tmpl.Nodes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal nodes for %s: %w", tmpl.ID, err)
		}

		if globalConfigJSON != nil {
			tmpl.GlobalConfig = &configsync.GlobalPipelineConfig{}
			if err := json.Unmarshal(globalConfigJSON, tmpl.GlobalConfig); err != nil {
				return nil, fmt.Errorf("failed to unmarshal global_config for %s: %w", tmpl.ID, err)
			}
		}

		if metadataJSON != nil {
			tmpl.Metadata = make(map[string]interface{})
			if err := json.Unmarshal(metadataJSON, &tmpl.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata for %s: %w", tmpl.ID, err)
			}
		}

		templates = append(templates, tmpl)
	}

	return templates, rows.Err()
}

// ListByEdition returns pipeline templates filtered by edition.
func (s *DBPipelineTemplateStore) ListByEdition(edition string) ([]configsync.PipelineTemplate, error) {
	query := fmt.Sprintf("SELECT id, name, description, shortcut_code, schema_version, version, edition, nodes, global_config, metadata FROM pipeline_templates WHERE edition = %s OR edition = 'all'", s.getDialect().Placeholder(1))
	rows, err := s.db.Query(query, edition)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []configsync.PipelineTemplate
	for rows.Next() {
		var tmpl configsync.PipelineTemplate
		var nodesJSON []byte
		var globalConfigJSON, metadataJSON []byte

		err := rows.Scan(&tmpl.ID, &tmpl.Name, &tmpl.Description, &tmpl.ShortcutCode, &tmpl.SchemaVersion, &tmpl.Version, &tmpl.Edition, &nodesJSON, &globalConfigJSON, &metadataJSON)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(nodesJSON, &tmpl.Nodes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal nodes for %s: %w", tmpl.ID, err)
		}

		if globalConfigJSON != nil {
			tmpl.GlobalConfig = &configsync.GlobalPipelineConfig{}
			if err := json.Unmarshal(globalConfigJSON, tmpl.GlobalConfig); err != nil {
				return nil, fmt.Errorf("failed to unmarshal global_config for %s: %w", tmpl.ID, err)
			}
		}

		if metadataJSON != nil {
			tmpl.Metadata = make(map[string]interface{})
			if err := json.Unmarshal(metadataJSON, &tmpl.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata for %s: %w", tmpl.ID, err)
			}
		}

		templates = append(templates, tmpl)
	}

	return templates, rows.Err()
}

// Clear removes all pipeline templates from the database.
func (s *DBPipelineTemplateStore) Clear() error {
	_, err := s.db.Exec("DELETE FROM pipeline_templates")
	return err
}
