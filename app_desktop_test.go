package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cndingbo2030/dingovault/internal/config"
)

func TestResolveDesktopDBPathDefaultsToConfigDir(t *testing.T) {
	dir := t.TempDir()
	config.SetDataDir(dir)
	t.Cleanup(func() { config.SetDataDir("") })

	got, err := resolveDesktopDBPath("dingovault.db")
	if err != nil {
		t.Fatalf("resolve default db path: %v", err)
	}
	want := filepath.Join(dir, "config", "dingovault.db")
	if got != want {
		t.Fatalf("default db path = %q, want %q", got, want)
	}
}

func TestResolveDesktopDBPathPreservesExplicitPath(t *testing.T) {
	got, err := resolveDesktopDBPath("custom.sqlite")
	if err != nil {
		t.Fatalf("resolve explicit db path: %v", err)
	}
	if got != "custom.sqlite" {
		t.Fatalf("explicit db path = %q", got)
	}

	got, err = resolveDesktopDBPath("  ")
	if err != nil {
		t.Fatalf("resolve blank db path: %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("dingovault", "dingovault.db")) {
		t.Fatalf("blank db path should resolve to app config dir, got %q", got)
	}
}
