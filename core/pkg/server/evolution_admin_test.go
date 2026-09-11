package server

import (
	"testing"
)

func TestEvolutionAdminFor(t *testing.T) {
	cases := []struct {
		name       string
		isAdmin    bool
		editionEnv string
		want       bool
	}{
		{"admin always allowed regardless edition", true, "team", true},
		{"admin allowed with no env", true, "", true},
		{"non-admin team blocked", false, "team", false},
		{"non-admin no env blocked", false, "", false},
		{"non-admin personal allowed (single-user owner)", false, "personal", true},
		{"non-admin minimal allowed (single-user owner)", false, "minimal", true},
		{"edition value case-insensitive", false, "Personal", true},
		{"unknown edition blocked", false, "enterprise", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CENTAG_EDITION", tc.editionEnv)
			if got := evolutionAdminFor(tc.isAdmin); got != tc.want {
				t.Errorf("evolutionAdminFor(isAdmin=%v, edition=%q) = %v, want %v", tc.isAdmin, tc.editionEnv, got, tc.want)
			}
		})
	}
}
