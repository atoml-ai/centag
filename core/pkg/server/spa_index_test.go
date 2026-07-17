package server

import (
	"centag/core/internal/edition"
	"testing"
)

func TestConversationStoreKind(t *testing.T) {
	cases := []struct {
		ed     edition.Edition
		driver string
		want   string
	}{
		{edition.Minimal, "", "file"},
		{edition.Personal, "sqlite", "sqlite"},
		{edition.Personal, "postgresql", "sqlite"}, // personal ignores pg driver
		{edition.Team, "postgresql", "postgresql"},
		{edition.Team, "sqlite", "sqlite"},
	}
	for _, tc := range cases {
		if got := conversationStoreKind(tc.ed, tc.driver); got != tc.want {
			t.Fatalf("edition=%s driver=%q got %q want %q", tc.ed, tc.driver, got, tc.want)
		}
	}
}
