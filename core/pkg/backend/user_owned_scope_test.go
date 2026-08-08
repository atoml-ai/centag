package backend

import "testing"

func TestUserOwnedScope(t *testing.T) {
	orig := GetManager()
	m := NewManager()
	if err := m.Add(&BackendConfig{ID: "user-backend", Name: "u", TenantID: "user:7"}); err != nil {
		t.Fatalf("add user backend: %v", err)
	}
	if err := m.Add(&BackendConfig{ID: "team-backend", Name: "t"}); err != nil {
		t.Fatalf("add team backend: %v", err)
	}
	SetManagerForTest(m)
	t.Cleanup(func() {
		SetManagerForTest(orig)
	})

	cases := []struct {
		name string
		id   string
		want string
	}{
		{"user-owned backend returns scope", "user-backend", "user:7"},
		{"system/team backend returns empty", "team-backend", ""},
		{"unknown backend returns empty", "nope", ""},
		{"empty id returns empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := UserOwnedScope(tc.id); got != tc.want {
				t.Fatalf("UserOwnedScope(%q) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}
