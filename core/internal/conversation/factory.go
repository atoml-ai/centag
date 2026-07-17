package conversation

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"centag/core/internal/edition"
)

// Options configures store construction.
type Options struct {
	Edition    edition.Edition
	DB         *sql.DB
	Driver     string // sqlite | postgresql (plugin names may vary)
	FileRoot   string // for minimal; default var/conversations
}

// PrepareEphemeralFileRoot wipes and recreates a directory for process-lifetime file data.
// Used by minimal edition so conversations reset on each service restart.
func PrepareEphemeralFileRoot(root string) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("ephemeral file root is empty")
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("clear ephemeral root: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create ephemeral root: %w", err)
	}
	return nil
}

// NewStore selects the storage backend by edition.
func NewStore(opts Options) (Store, error) {
	switch opts.Edition {
	case edition.Minimal:
		root := opts.FileRoot
		if root == "" {
			root = filepath.Join("var", "conversations")
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			return nil, fmt.Errorf("conversation file root: %w", err)
		}
		return NewFileStore(root), nil
	case edition.Personal:
		if opts.DB == nil {
			return nil, fmt.Errorf("conversation sqlite store requires db")
		}
		return NewSQLStore(opts.DB, dialectSQLite), nil
	default: // team
		if opts.DB == nil {
			return nil, fmt.Errorf("conversation postgres store requires db")
		}
		d := dialectPostgres
		if opts.Driver != "" && !isPostgresDriver(opts.Driver) {
			// allow sqlite in tests for team path
			d = dialectSQLite
		}
		return NewSQLStore(opts.DB, d), nil
	}
}

func isPostgresDriver(driver string) bool {
	switch driver {
	case "postgresql", "postgres", "pgx":
		return true
	default:
		return false
	}
}
