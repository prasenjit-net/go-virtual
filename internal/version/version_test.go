package version

import "testing"

func TestGet(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	origBuildDate := BuildDate

	Version = "1.2.3"
	Commit = "abc123"
	BuildDate = "2024-01-01T00:00:00Z"

	info := Get()

	if info.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", info.Version)
	}
	if info.Commit != "abc123" {
		t.Fatalf("expected commit abc123, got %q", info.Commit)
	}
	if info.BuildDate != "2024-01-01T00:00:00Z" {
		t.Fatalf("expected build date to match")
	}
	if info.GoVersion == "" {
		t.Fatalf("expected Go version to be set")
	}

	Version = origVersion
	Commit = origCommit
	BuildDate = origBuildDate
}

func TestGet_StripLeadingV(t *testing.T) {
	origVersion := Version
	Version = "v1.2.3"
	defer func() { Version = origVersion }()

	info := Get()
	if info.Version != "1.2.3" {
		t.Fatalf("expected v-prefix stripped, got %q", info.Version)
	}
}

func TestStripV(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"dev", "dev"},
		{"", ""},
	}
	for _, c := range cases {
		if got := stripV(c.in); got != c.want {
			t.Errorf("stripV(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
