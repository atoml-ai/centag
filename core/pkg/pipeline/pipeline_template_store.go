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
	if !database.IsInitialized() {
		return nil, fmt.Errorf("database not initialized")
	}
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
// This preserves the is_user_modified flag (used for remote sync).
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

// UpsertFromRemote syncs a template from remote (Feishu) and respects is_user_modified.
// If the template has is_user_modified = TRUE, it will NOT be overwritten.
// If the template doesn't exist or is_user_modified = FALSE, it will be created/updated.
func (s *DBPipelineTemplateStore) UpsertFromRemote(tmpl configsync.PipelineTemplate) (bool, error) {
	// First check if the template exists and is user-modified
	var isUserModified bool
	err := s.db.QueryRow("SELECT is_user_modified FROM pipeline_templates WHERE id = ?", tmpl.ID).Scan(&isUserModified)
	if err == nil && isUserModified {
		// Template exists and is user-modified, skip update
		return false, nil
	}

	// Template doesn't exist or is not modified, proceed with upsert
	nodesJSON, err := json.Marshal(tmpl.Nodes)
	if err != nil {
		return false, fmt.Errorf("failed to marshal nodes: %w", err)
	}

	var globalConfigJSON []byte
	if tmpl.GlobalConfig != nil {
		globalConfigJSON, err = json.Marshal(tmpl.GlobalConfig)
		if err != nil {
			return false, fmt.Errorf("failed to marshal global_config: %w", err)
		}
	}

	var metadataJSON []byte
	if tmpl.Metadata != nil {
		metadataJSON, err = json.Marshal(tmpl.Metadata)
		if err != nil {
			return false, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	edition := tmpl.Edition
	if edition == "" {
		edition = "all"
	}

	ph := make([]string, 12)
	for i := range ph {
		ph[i] = s.getDialect().Placeholder(i + 1)
	}
	query := fmt.Sprintf(`INSERT INTO pipeline_templates (id, name, description, shortcut_code, schema_version, version, edition, nodes, global_config, metadata, is_user_modified, created_at, updated_at)
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
			is_user_modified = FALSE,
			updated_at = excluded.updated_at`, strings.Join(ph, ", "))

	now := time.Now()
	_, err = s.db.Exec(query, tmpl.ID, tmpl.Name, tmpl.Description, tmpl.ShortcutCode, tmpl.SchemaVersion, tmpl.Version, edition, nodesJSON, globalConfigJSON, metadataJSON, now, now)
	if err != nil {
		return false, fmt.Errorf("failed to upsert pipeline template %s: %w", tmpl.ID, err)
	}
	return true, nil
}

// UpsertAsAdmin saves a template as Admin edit, setting is_user_modified = TRUE.
func (s *DBPipelineTemplateStore) UpsertAsAdmin(tmpl configsync.PipelineTemplate) error {
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

	edition := tmpl.Edition
	if edition == "" {
		edition = "all"
	}

	ph := make([]string, 12)
	for i := range ph {
		ph[i] = s.getDialect().Placeholder(i + 1)
	}
	query := fmt.Sprintf(`INSERT INTO pipeline_templates (id, name, description, shortcut_code, schema_version, version, edition, nodes, global_config, metadata, is_user_modified, created_at, updated_at)
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
			is_user_modified = TRUE,
			updated_at = excluded.updated_at`, strings.Join(ph, ", "))

	now := time.Now()
	_, err = s.db.Exec(query, tmpl.ID, tmpl.Name, tmpl.Description, tmpl.ShortcutCode, tmpl.SchemaVersion, tmpl.Version, edition, nodesJSON, globalConfigJSON, metadataJSON, now, now)
	if err != nil {
		return fmt.Errorf("failed to upsert pipeline template as admin %s: %w", tmpl.ID, err)
	}
	return nil
}

// ResetAllModified clears all is_user_modified flags and resets templates for fresh sync.
func (s *DBPipelineTemplateStore) ResetAllModified() error {
	_, err := s.db.Exec("UPDATE pipeline_templates SET is_user_modified = FALSE")
	return err
}

// ListModified returns all templates that have been modified by Admin.
func (s *DBPipelineTemplateStore) ListModified() ([]configsync.PipelineTemplate, error) {
	rows, err := s.db.Query("SELECT id, name, description, shortcut_code, schema_version, version, edition, nodes, global_config, metadata FROM pipeline_templates WHERE is_user_modified = TRUE")
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

// GetModifiedStatus returns the is_user_modified status for a template.
func (s *DBPipelineTemplateStore) GetModifiedStatus(id string) (bool, error) {
	var isUserModified bool
	err := s.db.QueryRow("SELECT is_user_modified FROM pipeline_templates WHERE id = ?", id).Scan(&isUserModified)
	if err != nil {
		return false, err
	}
	return isUserModified, nil
}

// MarkAsModified marks a single template as user-modified.
func (s *DBPipelineTemplateStore) MarkAsModified(id string) error {
	_, err := s.db.Exec("UPDATE pipeline_templates SET is_user_modified = TRUE WHERE id = ?", id)
	return err
}

// ResetSingleTemplate resets a single template to default (clears is_user_modified flag).
func (s *DBPipelineTemplateStore) ResetSingleTemplate(id string) error {
	_, err := s.db.Exec("UPDATE pipeline_templates SET is_user_modified = FALSE WHERE id = ?", id)
	return err
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
