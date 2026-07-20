package editionmodule_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/pkg/editionmodule"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeModule struct {
	name string
}

func (f *fakeModule) Name() string { return f.name }

func (f *fakeModule) RegisterAdmin(rg *gin.RouterGroup, _ editionmodule.AdminDeps) error {
	rg.GET("/"+f.name+"/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": f.name})
	})
	return nil
}

func (f *fakeModule) EnrichCapabilities(base map[string]bool) map[string]bool {
	if base == nil {
		base = map[string]bool{}
	}
	out := make(map[string]bool, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out["pro_"+f.name] = true
	return out
}

func TestMountAdminAndEnrich(t *testing.T) {
	editionmodule.ResetForTest()
	defer editionmodule.ResetForTest()

	editionmodule.Register(&fakeModule{name: "demo"})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	admin := r.Group("/api/v1/admin/pro")
	require.NoError(t, editionmodule.MountAdmin(admin, editionmodule.AdminDeps{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/pro/demo/ping", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"module":"demo"`)

	caps := editionmodule.EnrichCapabilities(map[string]bool{"base": true})
	require.True(t, caps["base"])
	require.True(t, caps["pro_demo"])
}

func TestLicenseDefaultAllowAll(t *testing.T) {
	editionmodule.ResetForTest()
	defer editionmodule.ResetForTest()
	require.True(t, editionmodule.License().Enabled("anything"))
}
