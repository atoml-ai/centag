package pipeline

import (
	"database/sql"
	"os"
	"testing"

	"centag/core/pkg/configsync"

	_ "modernc.org/sqlite"
)

func initTestSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func applyPipelineTemplateMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	migrationSQL, err := os.ReadFile("../database/migrations/043_pipeline_templates.sqlite.sql")
	if err != nil {
		t.Fatalf("failed to read migration: %v", err)
	}
	if _, err := db.Exec(string(migrationSQL)); err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}
}

func TestDBPipelineTemplateStore_Upsert(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyPipelineTemplateMigration(t, db)

	store := &DBPipelineTemplateStore{db: db}

	// TC-PTS-001: Insert new template
	tmpl := configsync.PipelineTemplate{
		ID:          "test-pipeline",
		Name:        "Test Pipeline",
		Description: "A test pipeline",
		Nodes:       []configsync.PipelineNodeConfig{{ID: "node1", Type: "llm"}},
	}
	if err := store.Upsert(tmpl); err != nil {
		t.Fatalf("TC-PTS-001: unexpected error: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("TC-PTS-001: failed to count: %v", err)
	}
	if count != 1 {
		t.Fatalf("TC-PTS-001: got count %d, want 1", count)
	}

	// TC-PTS-002: Update existing template
	tmpl.Name = "Updated Pipeline"
	if err := store.Upsert(tmpl); err != nil {
		t.Fatalf("TC-PTS-002: unexpected error: %v", err)
	}

	templates, err := store.ListAll()
	if err != nil {
		t.Fatalf("TC-PTS-002: failed to list: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("TC-PTS-002: got %d templates, want 1", len(templates))
	}
	if templates[0].Name != "Updated Pipeline" {
		t.Fatalf("TC-PTS-002: got name %q, want %q", templates[0].Name, "Updated Pipeline")
	}
}

func TestDBPipelineTemplateStore_Count(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyPipelineTemplateMigration(t, db)

	store := &DBPipelineTemplateStore{db: db}

	// TC-PTS-003: Count empty store
	count, err := store.Count()
	if err != nil {
		t.Fatalf("TC-PTS-003: unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("TC-PTS-003: got %d, want 0", count)
	}

	// TC-PTS-004: Count after inserts
	_ = store.Upsert(configsync.PipelineTemplate{ID: "p1", Name: "P1", Nodes: []configsync.PipelineNodeConfig{}})
	_ = store.Upsert(configsync.PipelineTemplate{ID: "p2", Name: "P2", Nodes: []configsync.PipelineNodeConfig{}})
	count, err = store.Count()
	if err != nil {
		t.Fatalf("TC-PTS-004: unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("TC-PTS-004: got %d, want 2", count)
	}
}

func TestDBPipelineTemplateStore_ListAll(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyPipelineTemplateMigration(t, db)

	store := &DBPipelineTemplateStore{db: db}

	// TC-PTS-005: ListAll empty store
	templates, err := store.ListAll()
	if err != nil {
		t.Fatalf("TC-PTS-005: unexpected error: %v", err)
	}
	if len(templates) != 0 {
		t.Fatalf("TC-PTS-005: got %d templates, want 0", len(templates))
	}

	// TC-PTS-006: ListAll with data
	_ = store.Upsert(configsync.PipelineTemplate{ID: "p1", Name: "P1", Nodes: []configsync.PipelineNodeConfig{{ID: "n1"}}})
	_ = store.Upsert(configsync.PipelineTemplate{ID: "p2", Name: "P2", Nodes: []configsync.PipelineNodeConfig{{ID: "n2"}}})

	templates, err = store.ListAll()
	if err != nil {
		t.Fatalf("TC-PTS-006: unexpected error: %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("TC-PTS-006: got %d templates, want 2", len(templates))
	}
}

func TestDBPipelineTemplateStore_ListByEdition(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyPipelineTemplateMigration(t, db)

	store := &DBPipelineTemplateStore{db: db}

	// TC-PTS-007: ListByEdition with mixed editions
	_ = store.Upsert(configsync.PipelineTemplate{ID: "p1", Name: "P1", Edition: "all", Nodes: []configsync.PipelineNodeConfig{}})
	_ = store.Upsert(configsync.PipelineTemplate{ID: "p2", Name: "P2", Edition: "personal", Nodes: []configsync.PipelineNodeConfig{}})
	_ = store.Upsert(configsync.PipelineTemplate{ID: "p3", Name: "P3", Edition: "team", Nodes: []configsync.PipelineNodeConfig{}})

	// TC-PTS-008: Filter by "personal" should return personal + all
	templates, err := store.ListByEdition("personal")
	if err != nil {
		t.Fatalf("TC-PTS-008: unexpected error: %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("TC-PTS-008: got %d templates, want 2", len(templates))
	}

	// TC-PTS-009: Filter by "team" should return team + all
	templates, err = store.ListByEdition("team")
	if err != nil {
		t.Fatalf("TC-PTS-009: unexpected error: %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("TC-PTS-009: got %d templates, want 2", len(templates))
	}

	// TC-PTS-010: Filter by "all" should return only "all" edition
	templates, err = store.ListByEdition("all")
	if err != nil {
		t.Fatalf("TC-PTS-010: unexpected error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("TC-PTS-010: got %d templates, want 1", len(templates))
	}
}

func TestDBPipelineTemplateStore_GlobalConfig(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyPipelineTemplateMigration(t, db)

	store := &DBPipelineTemplateStore{db: db}

	// TC-PTS-011: Store and retrieve with GlobalConfig
	tmpl := configsync.PipelineTemplate{
		ID:   "p1",
		Name: "P1",
		Nodes: []configsync.PipelineNodeConfig{},
		GlobalConfig: &configsync.GlobalPipelineConfig{
			MaxRetries: 3,
			Timeout:    30,
		},
	}
	if err := store.Upsert(tmpl); err != nil {
		t.Fatalf("TC-PTS-011: unexpected error: %v", err)
	}

	templates, err := store.ListAll()
	if err != nil {
		t.Fatalf("TC-PTS-011: failed to list: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("TC-PTS-011: got %d templates, want 1", len(templates))
	}
	if templates[0].GlobalConfig == nil {
		t.Fatal("TC-PTS-011: GlobalConfig is nil")
	}
	if templates[0].GlobalConfig.MaxRetries != 3 {
		t.Fatalf("TC-PTS-011: got MaxRetries %d, want 3", templates[0].GlobalConfig.MaxRetries)
	}
}

func TestDBPipelineTemplateStore_Metadata(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyPipelineTemplateMigration(t, db)

	store := &DBPipelineTemplateStore{db: db}

	// TC-PTS-012: Store and retrieve with Metadata
	tmpl := configsync.PipelineTemplate{
		ID:   "p1",
		Name: "P1",
		Nodes: []configsync.PipelineNodeConfig{},
		Metadata: map[string]interface{}{
			"author": "test",
			"tags":   []string{"ai", "llm"},
		},
	}
	if err := store.Upsert(tmpl); err != nil {
		t.Fatalf("TC-PTS-012: unexpected error: %v", err)
	}

	templates, err := store.ListAll()
	if err != nil {
		t.Fatalf("TC-PTS-012: failed to list: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("TC-PTS-012: got %d templates, want 1", len(templates))
	}
	if templates[0].Metadata == nil {
		t.Fatal("TC-PTS-012: Metadata is nil")
	}
	if templates[0].Metadata["author"] != "test" {
		t.Fatalf("TC-PTS-012: got author %v, want test", templates[0].Metadata["author"])
	}
}

func TestDBPipelineTemplateStore_Clear(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyPipelineTemplateMigration(t, db)

	store := &DBPipelineTemplateStore{db: db}

	// TC-PTS-013: Clear store
	_ = store.Upsert(configsync.PipelineTemplate{ID: "p1", Name: "P1", Nodes: []configsync.PipelineNodeConfig{}})
	_ = store.Upsert(configsync.PipelineTemplate{ID: "p2", Name: "P2", Nodes: []configsync.PipelineNodeConfig{}})

	if err := store.Clear(); err != nil {
		t.Fatalf("TC-PTS-013: unexpected error: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("TC-PTS-013: unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("TC-PTS-013: got %d, want 0 after clear", count)
	}
}

func TestDBPipelineTemplateStore_DefaultEdition(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyPipelineTemplateMigration(t, db)

	store := &DBPipelineTemplateStore{db: db}

	// TC-PTS-014: Empty edition defaults to "all"
	tmpl := configsync.PipelineTemplate{
		ID:      "p1",
		Name:    "P1",
		Edition: "",
		Nodes:   []configsync.PipelineNodeConfig{},
	}
	if err := store.Upsert(tmpl); err != nil {
		t.Fatalf("TC-PTS-014: unexpected error: %v", err)
	}

	templates, err := store.ListAll()
	if err != nil {
		t.Fatalf("TC-PTS-014: failed to list: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("TC-PTS-014: got %d templates, want 1", len(templates))
	}
	if templates[0].Edition != "all" {
		t.Fatalf("TC-PTS-014: got edition %q, want %q", templates[0].Edition, "all")
	}
}
