package pipeline

import (
	"context"
	"testing"
)

func TestCategoryKeywordResolver_Table(t *testing.T) {
	r := CategoryKeywordResolver{}
	cases := []struct {
		name       string
		content    string
		categories []string
		wantCat    string
		wantEmpty  bool
	}{
		{"empty content", "", []string{"code"}, "", true},
		{"no categories", "hello code", nil, "", true},
		{"longest match", "please translate this code snippet", []string{"code", "translate"}, "translate", false},
		{"case insensitive", "Write CODE please", []string{"code"}, "code", false},
		{"no match", "hello world", []string{"code", "translate"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cat, conf, err := r.ResolveCategory(context.Background(), tc.content, tc.categories)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantEmpty {
				if cat != "" {
					t.Fatalf("want empty, got %q conf=%v", cat, conf)
				}
				return
			}
			if cat != tc.wantCat {
				t.Fatalf("cat=%q want %q", cat, tc.wantCat)
			}
			if conf < 0.35 {
				t.Fatalf("confidence too low: %v", conf)
			}
		})
	}
}

func TestSetIntentResolver_NilResetsDefault(t *testing.T) {
	SetIntentResolver(stubIntentResolver{cat: "x", conf: 1})
	SetIntentResolver(nil)
	got := GetIntentResolver()
	if _, ok := got.(CategoryKeywordResolver); !ok {
		t.Fatalf("expected CategoryKeywordResolver, got %T", got)
	}
}
