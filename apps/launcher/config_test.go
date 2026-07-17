package main

import "testing"

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Edition != EditionPersonal {
		t.Fatalf("edition=%s", cfg.Edition)
	}
	if cfg.Port != defaultPort {
		t.Fatalf("port=%d", cfg.Port)
	}
}

func TestParseConfigMinimal(t *testing.T) {
	cfg, err := parseConfig([]string{"-edition=minimal", "-port=20111", "-no-open", "-headless"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Edition != EditionMinimal || cfg.Port != 20111 || !cfg.NoOpen || !cfg.Headless {
		t.Fatalf("%+v", cfg)
	}
}

func TestParseConfigRejectsBadEdition(t *testing.T) {
	if _, err := parseConfig([]string{"-edition=team"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSidecarCandidateNames(t *testing.T) {
	min := sidecarCandidateNames(EditionMinimal)
	if min[0] != "centag-minimal" {
		t.Fatalf("%v", min)
	}
	pers := sidecarCandidateNames(EditionPersonal)
	if pers[0] != "centag-gateway" {
		t.Fatalf("%v", pers)
	}
}
