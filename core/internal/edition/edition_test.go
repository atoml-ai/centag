package edition

import "testing"

func TestParse(t *testing.T) {
	if Parse("personal") != Personal {
		t.Fatal("expected personal")
	}
	if Parse("team") != Team {
		t.Fatal("expected team")
	}
	if Parse("") != Team {
		t.Fatal("empty should default to team")
	}
	if Parse("unknown") != Team {
		t.Fatal("unknown should default to team")
	}
}