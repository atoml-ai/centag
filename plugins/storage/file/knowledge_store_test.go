package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"centag/core/pkg/storage"
)

func TestFileKnowledgeStore_Store_Retrieve_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileKnowledgeStore(tmpDir, "default")
	if err != nil {
		t.Fatalf("NewFileKnowledgeStore failed: %v", err)
	}

	ctx := context.Background()

	t.Run("store and retrieve txt", func(t *testing.T) {
		doc := &storage.DataDocument{
			ID:         "doc-1",
			Content:    "Hello world from txt document",
			Collection: "default",
			DataType:   storage.DataTypeKnowledge,
		}
		if err := store.Store(ctx, doc); err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		results, err := store.Retrieve(ctx, "hello", 10, nil)
		if err != nil {
			t.Fatalf("Retrieve failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected at least one result")
		}
		if results[0].Document.ID != "doc-1" {
			t.Fatalf("expected doc-1, got %s", results[0].Document.ID)
		}
	})

	t.Run("store and retrieve json", func(t *testing.T) {
		doc := &storage.DataDocument{
			ID:         "doc-2",
			Content:    "{\"key\": \"value\", \"description\": \"JSON knowledge document\"}",
			Collection: "default",
			DataType:   storage.DataTypeKnowledge,
			Metadata: map[string]interface{}{
				"format": "json",
				"topic":  "testing",
			},
		}
		if err := store.Store(ctx, doc); err != nil {
			t.Fatalf("Store JSON failed: %v", err)
		}

		results, err := store.Retrieve(ctx, "knowledge", 10, nil)
		if err != nil {
			t.Fatalf("Retrieve failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected at least one result for JSON doc")
		}
	})

	t.Run("store and retrieve md", func(t *testing.T) {
		doc := &storage.DataDocument{
			ID:         "doc-3",
			Content:    "# Markdown Document\n\nThis is a test markdown file.",
			Collection: "default",
			DataType:   storage.DataTypeKnowledge,
			Metadata:   map[string]interface{}{"format": "md"},
		}
		if err := store.Store(ctx, doc); err != nil {
			t.Fatalf("Store MD failed: %v", err)
		}

		results, err := store.Retrieve(ctx, "markdown", 10, nil)
		if err != nil {
			t.Fatalf("Retrieve failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected at least one result for MD doc")
		}
	})

	t.Run("retrieve with filter", func(t *testing.T) {
		filter := map[string]interface{}{"topic": "testing"}
		results, err := store.Retrieve(ctx, "knowledge", 10, filter)
		if err != nil {
			t.Fatalf("Retrieve with filter failed: %v", err)
		}
		for _, r := range results {
			if r.Document.Metadata == nil || r.Document.Metadata["topic"] != "testing" {
				t.Fatalf("expected topic=testing in metadata, got %v", r.Document.Metadata)
			}
		}
	})

	t.Run("delete knowledge", func(t *testing.T) {
		if err := store.DeleteKnowledge(ctx, "doc-1"); err != nil {
			t.Fatalf("DeleteKnowledge failed: %v", err)
		}
		results, err := store.Retrieve(ctx, "hello", 10, nil)
		if err != nil {
			t.Fatalf("Retrieve after delete failed: %v", err)
		}
		for _, r := range results {
			if r.Document.ID == "doc-1" {
				t.Fatal("doc-1 should have been deleted")
			}
		}
	})
}

func TestFileKnowledgeStore_ListCollections(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileKnowledgeStore(tmpDir, "default")
	if err != nil {
		t.Fatalf("NewFileKnowledgeStore failed: %v", err)
	}

	ctx := context.Background()

	if err := os.MkdirAll(filepath.Join(tmpDir, "test-coll"), 0755); err != nil {
		t.Fatalf("failed to create test collection dir: %v", err)
	}

	cols, err := store.ListCollections(ctx)
	if err != nil {
		t.Fatalf("ListCollections failed: %v", err)
	}

	foundDefault := false
	foundTest := false
	for _, c := range cols {
		if c == "default" {
			foundDefault = true
		}
		if c == "test-coll" {
			foundTest = true
		}
	}
	if !foundDefault {
		t.Fatal("expected 'default' collection")
	}
	if !foundTest {
		t.Fatal("expected 'test-coll' collection")
	}
}

func TestFileKnowledgeStore_HealthCheck(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileKnowledgeStore(tmpDir, "default")
	if err != nil {
		t.Fatalf("NewFileKnowledgeStore failed: %v", err)
	}

	ctx := context.Background()
	if err := store.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}

func TestFileKnowledgeStore_Close(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileKnowledgeStore(tmpDir, "default")
	if err != nil {
		t.Fatalf("NewFileKnowledgeStore failed: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestFileKnowledgeStore_GetStoreInfo(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileKnowledgeStore(tmpDir, "default")
	if err != nil {
		t.Fatalf("NewFileKnowledgeStore failed: %v", err)
	}

	info := store.GetStoreInfo()
	if info.Type != "file" {
		t.Fatalf("expected type 'file', got %s", info.Type)
	}
}

func TestFileKnowledgeStore_GetDataType(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileKnowledgeStore(tmpDir, "default")
	if err != nil {
		t.Fatalf("NewFileKnowledgeStore failed: %v", err)
	}

	dt := store.GetDataType()
	if dt != storage.DataTypeKnowledge {
		t.Fatalf("expected DataTypeKnowledge, got %s", dt)
	}
}

func TestFileKnowledgeStore_JSONRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileKnowledgeStore(tmpDir, "default")
	if err != nil {
		t.Fatalf("NewFileKnowledgeStore failed: %v", err)
	}

	ctx := context.Background()

	doc := &storage.DataDocument{
		ID:         "roundtrip-1",
		Content:    "{\"roundtrip\": true}",
		Collection: "default",
		DataType:   storage.DataTypeKnowledge,
		Metadata: map[string]interface{}{
			"format": "json",
			"source": "test",
		},
	}

	if err := store.Store(ctx, doc); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	filePath := filepath.Join(tmpDir, "default", "roundtrip-1.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loadedDoc storage.DataDocument
	if err := json.Unmarshal(data, &loadedDoc); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if loadedDoc.ID != "roundtrip-1" {
		t.Fatalf("expected ID roundtrip-1, got %s", loadedDoc.ID)
	}
	if loadedDoc.Metadata == nil || loadedDoc.Metadata["source"] != "test" {
		t.Fatalf("metadata not preserved")
	}
}
