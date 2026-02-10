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
