package bridge

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVaultCommandUsesVaultRoot(t *testing.T) {
	dir := t.TempDir()
	app := NewApp(nil, nil, dir)

	got, err := app.RunVaultCommand("pwd")
	if err != nil {
		t.Fatal(err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("exit code = %d, output = %q", got.ExitCode, got.Output)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got.Output) != want {
		t.Fatalf("pwd = %q, want %q", strings.TrimSpace(got.Output), want)
	}
}

func TestRunVaultCommandKeepsNonZeroOutput(t *testing.T) {
	app := NewApp(nil, nil, t.TempDir())

	got, err := app.RunVaultCommand("printf 'bad'; exit 7")
	if err != nil {
		t.Fatal(err)
	}
	if got.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", got.ExitCode)
	}
	if got.Output != "bad" {
		t.Fatalf("output = %q, want bad", got.Output)
	}
}
