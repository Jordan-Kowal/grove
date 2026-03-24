package backend

import (
	"os"
	"testing"
)

func TestFixPathSetsNonEmptyPath(t *testing.T) {
	original := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", original) }()

	_ = os.Setenv("PATH", "/fake/path")
	FixPath()

	got := os.Getenv("PATH")
	if got == "/fake/path" {
		t.Error("FixPath did not update PATH")
	}
	if got == "" {
		t.Error("FixPath set PATH to empty string")
	}
}

func TestFixPathContainsUsrBin(t *testing.T) {
	original := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", original) }()

	FixPath()

	got := os.Getenv("PATH")
	if got == "" {
		t.Fatal("FixPath set PATH to empty string")
	}
	// /usr/bin should always be in a resolved macOS PATH
	found := false
	for _, p := range splitPath(got) {
		if p == "/usr/bin" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PATH to contain /usr/bin, got %q", got)
	}
}

func splitPath(p string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == ':' {
			parts = append(parts, p[start:i])
			start = i + 1
		}
	}
	parts = append(parts, p[start:])
	return parts
}
