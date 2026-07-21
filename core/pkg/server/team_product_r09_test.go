package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"centag/core/pkg/extension"

	"github.com/gin-gonic/gin"
)

// R09: open-core must not hard-register Team product routes; only Apply* from Host.
func TestOpenCoreServerSourceHasNoHardcodedTeamProductRoutes(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	srcPath := filepath.Join(filepath.Dir(file), "server.go")
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	forbidden := []string{
		`teamAdmin.GET("/users"`,
		`teamAdmin.POST("/users"`,
		`teamAdmin.GET("/api-keys"`,
		`teamAdmin.GET("/tenants"`,
		`teamAdmin.GET("/token-usage/all"`,
		`teamAdmin.GET("/ab-eval/`,
		`system.POST("/update"`,
		`system.GET("/update/history"`,
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("R09: open-core server.go still hard-registers %q; must use extension.Host only", needle)
		}
	}
	if !strings.Contains(src, "ApplyTeamAdmin") || !strings.Contains(src, "ApplySystemAPI") {
		t.Fatal("expected ApplyTeamAdmin/ApplySystemAPI hooks in server.go")
	}
}

func TestEmptyRuntimeHostExposesNoTeamProductRoutes(t *testing.T) {
	host := extension.NewRuntimeHost("team", extension.Deps{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	host.ApplyTeamAdmin(r.Group("/api/v1/admin"))
	host.ApplyUserAPI(r.Group("/api/v1/user"))
	host.ApplySystemAPI(r.Group("/api/v1/system"))
	if len(r.Routes()) != 0 {
		t.Fatalf("empty Host must not register product routes, got %#v", r.Routes())
	}
}
