package proxymode

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}
	
	modes := mgr.ListModes()
	if len(modes) == 0 {
		t.Error("Expected default modes to be initialized")
	}
}

func TestManager_ListModes(t *testing.T) {
	mgr := NewManager()
	modes := mgr.ListModes()
	
	expectedKeys := []string{"#d", "#s", "#m", "#c", "#t", "#f"}
	for _, key := range expectedKeys {
		found := false
		for _, mode := range modes {
			if mode.Key == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected mode key %s to exist", key)
		}
	}
}

func TestManager_GetMode(t *testing.T) {
	mgr := NewManager()
	
	mode, exists := mgr.GetMode("#d")
	if !exists {
		t.Fatal("Expected #d mode to exist")
	}
	if mode.Key != "#d" {
		t.Errorf("Expected key #d, got %s", mode.Key)
	}
	if mode.Type != "direct" {
		t.Errorf("Expected type direct, got %s", mode.Type)
	}
	
	// Test non-existent mode
	_, exists = mgr.GetMode("#invalid")
	if exists {
		t.Error("Expected #invalid mode to not exist")
	}
}

func TestManager_AddMode(t *testing.T) {
	mgr := NewManager()
	
	newMode := ModeConfig{
		Key:         "#x",
		Name:        "测试模式",
		Type:        "custom",
		Description: "自定义测试模式",
		Enabled:     true,
		Config:      map[string]interface{}{"test": "value"},
	}
	
	err := mgr.AddMode(newMode)
	if err != nil {
		t.Fatalf("AddMode() error: %v", err)
	}
	
	mode, exists := mgr.GetMode("#x")
	if !exists {
		t.Fatal("Expected #x mode to exist after adding")
	}
	if mode.Name != "测试模式" {
		t.Errorf("Expected name 测试模式，got %s", mode.Name)
	}
}

func TestManager_AddMode_DuplicateKey(t *testing.T) {
	mgr := NewManager()
	
	newMode := ModeConfig{
		Key:         "#d",
		Name:        "重复模式",
		Type:        "direct",
		Description: "重复的键",
		Enabled:     true,
	}
	
	err := mgr.AddMode(newMode)
	if err == nil {
		t.Error("Expected error when adding duplicate key")
	}
}

func TestManager_UpdateMode(t *testing.T) {
	mgr := NewManager()
	
	updated := ModeConfig{
		Key:         "#d",
		Name:        "更新后的指定后端",
		Type:        "direct",
		Description: "已更新描述",
		Enabled:     false,
		Config:      map[string]interface{}{"new": "config"},
	}
	
	err := mgr.UpdateMode(updated)
	if err != nil {
		t.Fatalf("UpdateMode() error: %v", err)
	}
	
	mode, exists := mgr.GetMode("#d")
	if !exists {
		t.Fatal("Expected #d mode to exist after update")
	}
	if mode.Name != "更新后的指定后端" {
		t.Errorf("Expected updated name, got %s", mode.Name)
	}
	if mode.Enabled {
		t.Error("Expected mode to be disabled after update")
	}
}

func TestManager_UpdateMode_NonExistent(t *testing.T) {
	mgr := NewManager()
	
	updated := ModeConfig{
		Key:         "#nonexistent",
		Name:        "不存在的模式",
		Type:        "custom",
		Description: "测试",
		Enabled:     true,
	}
	
	err := mgr.UpdateMode(updated)
	if err == nil {
		t.Error("Expected error when updating non-existent mode")
	}
}

func TestManager_DeleteMode(t *testing.T) {
	mgr := NewManager()
	
	// First add a custom mode
	customMode := ModeConfig{
		Key:         "#z",
		Name:        "待删除模式",
		Type:        "custom",
		Description: "测试删除",
		Enabled:     true,
	}
	err := mgr.AddMode(customMode)
	if err != nil {
		t.Fatalf("AddMode() error: %v", err)
	}
	
	// Delete it
	err = mgr.DeleteMode("#z")
	if err != nil {
		t.Fatalf("DeleteMode() error: %v", err)
	}
	
	// Verify it's gone
	_, exists := mgr.GetMode("#z")
	if exists {
		t.Error("Expected #z mode to be deleted")
	}
}

func TestManager_DeleteMode_Protected(t *testing.T) {
	mgr := NewManager()
	
	// Try to delete a default mode
	err := mgr.DeleteMode("#d")
	if err == nil {
		t.Error("Expected error when deleting protected default mode")
	}
}

func TestManager_EnableDisableMode(t *testing.T) {
	mgr := NewManager()
	
	// Disable
	err := mgr.EnableMode("#s", false)
	if err != nil {
		t.Fatalf("EnableMode() error: %v", err)
	}
	
	mode, exists := mgr.GetMode("#s")
	if !exists {
		t.Fatal("Expected #s mode to exist")
	}
	if mode.Enabled {
		t.Error("Expected #s mode to be disabled")
	}
	
	// Re-enable
	err = mgr.EnableMode("#s", true)
	if err != nil {
		t.Fatalf("EnableMode() error: %v", err)
	}
	
	mode, exists = mgr.GetMode("#s")
	if !exists {
		t.Fatal("Expected #s mode to exist")
	}
	if !mode.Enabled {
		t.Error("Expected #s mode to be enabled")
	}
}

func TestManager_ValidateModeKey(t *testing.T) {
	tests := []struct {
		key     string
		wantErr bool
	}{
		{"#d", false},
		{"#x", false},
		{"#1", false},
		{"#ag", false},   // multi-char keys supported (e.g. #ag, #mem0, #custom)
		{"d", true},      // missing #
		{"##d", true},    // double #
		{"", true},       // empty
		{"# ", true},     // space
		{"#@", true},     // special char
	}
	
	for _, tt := range tests {
		err := validateModeKey(tt.key)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateModeKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
		}
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	mgr := NewManager()
	done := make(chan bool, 10)
	
	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				mgr.ListModes()
				mgr.GetMode("#d")
			}
			done <- true
		}()
	}
	
	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				key := "#t" + string(rune('0'+id))
				mode := ModeConfig{
					Key:     key,
					Name:    "并发测试",
					Type:    "custom",
					Enabled: true,
				}
				mgr.AddMode(mode)
				mgr.DeleteMode(key)
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent test timeout")
		}
	}
}

func TestModeConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mode    ModeConfig
		wantErr bool
	}{
		{
			name: "valid",
			mode: ModeConfig{Key: "#x", Name: "测试", Type: "custom", Enabled: true},
			wantErr: false,
		},
		{
			name: "empty key",
			mode: ModeConfig{Key: "", Name: "测试", Type: "custom", Enabled: true},
			wantErr: true,
		},
		{
			name: "empty name",
			mode: ModeConfig{Key: "#x", Name: "", Type: "custom", Enabled: true},
			wantErr: true,
		},
		{
			name: "empty type",
			mode: ModeConfig{Key: "#x", Name: "测试", Type: "", Enabled: true},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mode.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ModeConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
