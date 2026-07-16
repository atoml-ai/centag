package agent

import (
	"strings"
	"testing"
)

func TestWebAgentTemplate_NewWithConfig(t *testing.T) {
	// nil config → default
	w := NewWebAgentTemplate(AgentCodingWeb, "Test", "desc", nil)
	if w.config == nil || !w.config.Headless {
		t.Error("nil config should use defaults")
	}

	// custom config
	cfg := &BrowserConfig{Headless: false, ViewportWidth: 1920, Timeout: 60}
	w2 := NewWebAgentTemplate(AgentCodingWeb, "Test", "desc", cfg)
	if w2.config.ViewportWidth != 1920 {
		t.Errorf("width = %d", w2.config.ViewportWidth)
	}
}

func TestWebAgentTemplate_BrowserConfig(t *testing.T) {
	cfg := &BrowserConfig{Headless: false}
	w := NewWebAgentTemplate(AgentCodingWeb, "Test", "", cfg)
	if w.BrowserConfig().Headless {
		t.Error("should return custom config")
	}
}

func TestWebAgentTemplate_AgentTemplateMethods(t *testing.T) {
	w := NewWebAgentTemplate(AgentEducationWeb, "Edu Web", "desc", nil)
	if w.AgentType() != AgentEducationWeb {
		t.Error("AgentType mismatch")
	}
	if w.DisplayName() != "Edu Web" {
		t.Error("DisplayName mismatch")
	}

	files, err := w.ConfigFiles(nil)
	if err != nil || len(files) != 0 {
		t.Error("ConfigFiles should return nil,nil")
	}
	if w.SetupCommand(nil) != "" {
		t.Error("SetupCommand should be empty")
	}
	if w.VerifyCommand(nil) != "" {
		t.Error("VerifyCommand should be empty")
	}
	if s := w.Steps(nil); len(s) != 0 {
		t.Error("Steps should be nil")
	}
	if err := w.WriteConfig(nil); err != nil {
		t.Error("WriteConfig should return nil")
	}
}

func TestWebAgentTemplate_StubMethods(t *testing.T) {
	w := NewWebAgentTemplate(AgentCodingWeb, "Test", "", nil)

	// ClickElement stub
	if err := w.ClickElement("#btn"); err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("ClickElement stub: got %v", err)
	}
	// FillFormField stub
	if err := w.FillFormField("#input", "val"); err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("FillFormField stub: got %v", err)
	}
	// WaitForElement stub
	if err := w.WaitForElement("#el", 1000); err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("WaitForElement stub: got %v", err)
	}

	// OpenBrowser
	err := w.OpenBrowser("http://example.com")
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Error("OpenBrowser stub should fail")
	}

	// TakeScreenshot
	data, err := w.TakeScreenshot()
	if err == nil || data != nil {
		t.Error("TakeScreenshot stub should fail with nil data")
	}

	// ExecuteJavaScript
	js, err := w.ExecuteJavaScript("1+1")
	if err == nil || js != "" {
		t.Error("ExecuteJavaScript stub should fail with empty result")
	}

	// GetPageContent
	content, err := w.GetPageContent()
	if err == nil || content != "" {
		t.Error("GetPageContent stub should fail with empty result")
	}
}

// ============================================================================
// CodingWebAgent Tests
// ============================================================================

func TestCodingWebAgent_New(t *testing.T) {
	a := NewCodingWebAgent(nil)
	if a.AgentType() != AgentCodingWeb {
		t.Errorf("AgentType = %q", a.AgentType())
	}
	if a.config == nil {
		t.Fatal("config should not be nil")
	}
}

func TestCodingWebAgent_ExecuteJavaScript(t *testing.T) {
	a := NewCodingWebAgent(nil)
	// Stub mode: 内部调用父类的 ExecuteJavaScript 会返回 error
	_, err := a.ExecuteJavaScript("console.log('test')")
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("should propagate browser engine error, got %v", err)
	}
}

func TestCodingWebAgent_WaitForElement_ZeroTimeout(t *testing.T) {
	a := NewCodingWebAgent(&BrowserConfig{Timeout: 10})
	// WaitForElement(selector, 0) → uses config.Timeout * 1000
	err := a.WaitForElement("#el", 0)
	if err == nil {
		t.Error("stub should return error even with default timeout")
	}
	if !strings.Contains(err.Error(), "10000ms") {
		t.Errorf("should use default timeout, got %q", err)
	}
}

// ============================================================================
// EducationWebAgent Tests
// ============================================================================

func TestEducationWebAgent_New(t *testing.T) {
	a := NewEducationWebAgent(nil)
	if a.AgentType() != AgentEducationWeb {
		t.Errorf("AgentType = %q", a.AgentType())
	}
}

func TestEducationWebAgent_OpenBrowser(t *testing.T) {
	a := NewEducationWebAgent(nil)
	err := a.OpenBrowser("http://learning.example.com")
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("should return browser engine error, got %v", err)
	}
}

func TestEducationWebAgent_GetPageContent(t *testing.T) {
	a := NewEducationWebAgent(nil)
	content, err := a.GetPageContent()
	if err == nil || content != "" {
		t.Errorf("stub should fail, got content=%q err=%v", content, err)
	}
}

// ============================================================================
// BrowserConfig Tests
// ============================================================================

func TestBrowserConfig_Defaults(t *testing.T) {
	cfg := DefaultBrowserConfig()
	if !cfg.Headless {
		t.Error("Headless should be true")
	}
	if cfg.ViewportWidth != 1280 {
		t.Errorf("ViewportWidth = %d", cfg.ViewportWidth)
	}
	if cfg.ViewportHeight != 720 {
		t.Errorf("ViewportHeight = %d", cfg.ViewportHeight)
	}
	if cfg.Timeout != 30 {
		t.Errorf("Timeout = %d", cfg.Timeout)
	}
}

func TestBrowserConfig_Custom(t *testing.T) {
	cfg := &BrowserConfig{
		Headless:       false,
		ViewportWidth:  800,
		ViewportHeight: 600,
		Timeout:        5,
		UserAgent:      "Mozilla/5.0",
	}
	if cfg.Headless {
		t.Error("headless should be false")
	}
	if cfg.Timeout != 5 {
		t.Error("timeout mismatch")
	}
	if cfg.UserAgent != "Mozilla/5.0" {
		t.Error("UserAgent mismatch")
	}
}

// ============================================================================
// WebAgent Interface Compliance
// ============================================================================

func TestWebAgentInterfaceCompliance(t *testing.T) {
	var _ WebAgent = (*WebAgentTemplate)(nil)
	var _ WebAgent = (*CodingWebAgent)(nil)
	var _ WebAgent = (*EducationWebAgent)(nil)
	var _ AgentTemplate = (*webConfigTemplate)(nil)
	t.Log("WebAgent interface compliance OK")
}
