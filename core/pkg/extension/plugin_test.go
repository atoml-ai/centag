package extension_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/pkg/extension"
	"centag/core/pkg/hooks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubPlugin struct {
	name  string
	inits int
}

func (s *stubPlugin) Name() string { return s.name }

func (s *stubPlugin) Init(host extension.Host) error {
	s.inits++
	_ = host.Edition()
	_ = host.Deps()
	return nil
}

func TestRegisterAndInitAll(t *testing.T) {
	extension.ResetForTest()
	defer extension.ResetForTest()

	p := &stubPlugin{name: "demo"}
	extension.Register(p)
	require.Len(t, extension.Plugins(), 1)
	require.NoError(t, extension.InitAll(nil))
	require.Equal(t, 1, p.inits)
}

type billingNop struct{}

func (billingNop) OnUsage(context.Context, *hooks.TokenUsage) error { return nil }
func (billingNop) OnQuotaExceeded(context.Context, int64) error     { return nil }

type routePlugin struct {
	mwRan bool
}

func (routePlugin) Name() string { return "routes" }

func (p *routePlugin) Init(host extension.Host) error {
	host.RegisterTeamAdmin(func(rg *gin.RouterGroup) {
		rg.GET("/ext-ping", func(c *gin.Context) {
			c.String(http.StatusOK, "team-admin")
		})
	})
	host.RegisterUserAPI(func(rg *gin.RouterGroup) {
		rg.GET("/ext-ping", func(c *gin.Context) {
			c.String(http.StatusOK, "user-api")
		})
	})
	host.RegisterSystemAPI(func(rg *gin.RouterGroup) {
		rg.GET("/ext-ping", func(c *gin.Context) {
			c.String(http.StatusOK, "system-api")
		})
	})
	host.RegisterProtectedMiddleware(func(c *gin.Context) {
		p.mwRan = true
		c.Next()
	})
	host.RegisterBillingHook(billingNop{})
	return nil
}

func TestRuntimeHostAppliesRegistrations(t *testing.T) {
	extension.ResetForTest()
	defer extension.ResetForTest()
	gin.SetMode(gin.TestMode)

	p := &routePlugin{}
	extension.Register(p)

	hm := hooks.NewManager()
	host := extension.NewRuntimeHost("team", extension.Deps{HookManager: hm})
	require.NoError(t, extension.InitAll(host))
	require.Equal(t, "team", host.Edition())
	require.NotNil(t, host.Deps().HookManager)

	r := gin.New()
	teamAdmin := r.Group("/api/v1/admin")
	host.ApplyTeamAdmin(teamAdmin)
	userAPI := r.Group("/api/v1/user")
	host.ApplyUserAPI(userAPI)
	systemAPI := r.Group("/api/v1/system")
	host.ApplySystemAPI(systemAPI)

	prot := r.Group("/api/v1")
	for _, mw := range host.ProtectedMiddlewares() {
		prot.Use(mw)
	}
	prot.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	host.FlushBillingHooks(hm)
	_, billing, _, _ := hm.Counts()
	require.Equal(t, 1, billing)

	assertBody := func(path, want string) {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, want, w.Body.String())
	}
	assertBody("/api/v1/admin/ext-ping", "team-admin")
	assertBody("/api/v1/user/ext-ping", "user-api")
	assertBody("/api/v1/system/ext-ping", "system-api")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ok", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, p.mwRan)
}

func TestInitAllNilHostNoRegistrationSideEffects(t *testing.T) {
	extension.ResetForTest()
	defer extension.ResetForTest()

	p := &routePlugin{}
	extension.Register(p)
	require.NoError(t, extension.InitAll(nil))

	// nopHost discarded registrations — a fresh RuntimeHost has nothing queued.
	host := extension.NewRuntimeHost("personal", extension.Deps{})
	r := gin.New()
	host.ApplyTeamAdmin(r.Group("/api/v1/admin"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ext-ping", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}
