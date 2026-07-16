package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBuildMetadata_InjectGeoRegionFromHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("X-Geo-Region", "US")
	c.Request = req

	dispatcher := &ModeDispatcher{}
	metadata := dispatcher.buildMetadata(c, ModeDefault, map[string]string{}, map[string]string{})

	if got := metadata["geo_region"]; got != "US" {
		t.Fatalf("geo_region=%v, want US", got)
	}
}

func TestBuildMetadata_InjectSceneFromQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?scene=problem_solving", nil)
	c.Request = req

	dispatcher := &ModeDispatcher{}
	metadata := dispatcher.buildMetadata(c, ModeDefault, map[string]string{}, map[string]string{
		"scene": "problem_solving",
	})

	if got := metadata["scene"]; got != "problem_solving" {
		t.Fatalf("scene=%v, want problem_solving", got)
	}
	if got := metadata["param_scene"]; got != "problem_solving" {
		t.Fatalf("param_scene=%v, want problem_solving", got)
	}
}

func TestBuildMetadata_HeaderSceneOverridesQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?scene=qa", nil)
	req.Header.Set("X-Scene", "essay_review")
	c.Request = req

	dispatcher := &ModeDispatcher{}
	metadata := dispatcher.buildMetadata(c, ModeDefault, map[string]string{}, map[string]string{
		"scene": "qa",
	})

	if got := metadata["scene"]; got != "essay_review" {
		t.Fatalf("scene=%v, want essay_review", got)
	}
}
