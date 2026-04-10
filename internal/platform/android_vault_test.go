package platform

import (
	"path/filepath"
	"testing"
)

func TestAndroidScopedVaultPath(t *testing.T) {
	base := "/storage/emulated/0/Android/data/com.example/files"
	got := AndroidScopedVaultPath(base)
	want := filepath.Join(base, "Dingovault")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if AndroidScopedVaultPath("") != "" {
		t.Fatal("empty input should yield empty path")
	}
}
