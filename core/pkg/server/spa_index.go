package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"centag/core/internal/edition"

	"github.com/gin-gonic/gin"
)

// serveSPAIndex returns index.html with data-edition injected for first-paint WebUI.
func (s *Server) serveSPAIndex(c *gin.Context, staticDir string) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	indexPath := filepath.Join(staticDir, "index.html")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		c.File(indexPath)
		return
	}

	ed := "team"
	if s != nil {
		switch {
		case s.edition.IsMinimal():
			ed = "minimal"
		case s.edition.IsPersonal():
			ed = "personal"
		}
	}

	html := string(data)
	if !strings.Contains(html, "data-edition=") {
		html = strings.Replace(html, "<html", `<html data-edition="`+ed+`"`, 1)
		html = strings.Replace(html, "<HTML", `<HTML data-edition="`+ed+`"`, 1)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

func conversationStoreKind(ed edition.Edition, driver string) string {
	switch ed {
	case edition.Minimal:
		return "file"
	case edition.Personal:
		return "sqlite"
	default:
		if driver != "" && !isPostgresDriverName(driver) {
			return "sqlite"
		}
		return "postgresql"
	}
}

func isPostgresDriverName(driver string) bool {
	switch driver {
	case "postgresql", "postgres", "pgx":
		return true
	default:
		return false
	}
}
