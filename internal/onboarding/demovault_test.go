package onboarding

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestEnsureDemoVaultFromFSTo_skipsWhenReadmeExists(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{
		"demo-vault/README.md": &fstest.MapFile{Data: []byte("demo"), Mode: 0o644},
	}
	if err := EnsureDemoVaultFromFSTo(dir, fsys, DemoVaultRootName); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "user" {
		t.Fatalf("expected user readme untouched, got %q", string(b))
	}
}

func TestEnsureDemoVaultFromFSTo_materializesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	fsys := fstest.MapFS{
		"demo-vault/README.md": &fstest.MapFile{Data: []byte("# Demo\n"), Mode: 0o644},
	}
	if err := EnsureDemoVaultFromFSTo(dir, fsys, DemoVaultRootName); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "# Demo\n" {
		t.Fatalf("readme: %q", string(b))
	}
}
