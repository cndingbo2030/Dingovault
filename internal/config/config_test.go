package config

import "testing"

func TestShouldOpenBundledDemo(t *testing.T) {
	t.Parallel()
	if !ShouldOpenBundledDemo("", Config{}) {
		t.Fatal("expected demo when CLI and vault are empty")
	}
	if ShouldOpenBundledDemo("/tmp/v", Config{}) {
		t.Fatal("cli path set")
	}
	if ShouldOpenBundledDemo("", Config{VaultPath: "/vault"}) {
		t.Fatal("saved vault set")
	}
	if !ShouldOpenBundledDemo("  ", Config{VaultPath: "  "}) {
		t.Fatal("whitespace-only paths should be treated as unset (demo on first run)")
	}
}
