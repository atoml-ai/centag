package ota

import "testing"

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"v0.2.8", "0.2.8"},
		{"0.2.8", "0.2.8"},
		{" V1.0.0 ", "1.0.0"},
		{"dev", "dev"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeVersion(tc.in); got != tc.want {
			t.Fatalf("NormalizeVersion(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.2.7", "0.2.8", -1},
		{"v0.2.8", "0.2.8", 0},
		{"0.3.0", "0.2.9", 1},
		{"dev", "0.1.0", -1},
		{"0.1.0", "dev", 1},
		{"1.0.0-rc1", "1.0.0", -1},
		{"1.0.0", "1.0.0-rc1", 1},
	}
	for _, tc := range cases {
		if got := CompareVersions(tc.a, tc.b); got != tc.want {
			t.Fatalf("CompareVersions(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	got := AssetName("team", "v0.2.8", "linux", "amd64")
	want := "update-package-centag-team-0.2.8-linux-amd64.tar.gz"
	if got != want {
		t.Fatalf("AssetName=%q want %q", got, want)
	}
}

func TestParseChecksum(t *testing.T) {
	sum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	content := sum + "  update-package-centag-team-0.2.8-linux-amd64.tar.gz\n"
	got := ParseChecksum(content, "update-package-centag-team-0.2.8-linux-amd64.tar.gz")
	if got != sum {
		t.Fatalf("ParseChecksum=%q want %q", got, sum)
	}
	if got := ParseChecksum(sum+"\n", "anything.tar.gz"); got != sum {
		t.Fatalf("bare digest ParseChecksum=%q", got)
	}
}
