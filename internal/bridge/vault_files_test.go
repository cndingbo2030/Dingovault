package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListVaultFilesSupportedKinds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile := func(rel string) {
		t.Helper()
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("README.md")
	writeFile("docs/spec.docx")
	writeFile("docs/report.pdf")
	writeFile("images/screen.PNG")
	writeFile("drawings/site.DWG")
	writeFile("skip.txt")
	writeFile(".hidden/secret.md")

	app := NewApp(nil, nil, dir)
	files, err := app.ListVaultFiles()
	if err != nil {
		t.Fatal(err)
	}

	byPath := make(map[string]VaultFileDTO, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}
	want := map[string]string{
		"README.md":         "markdown",
		"docs/spec.docx":    "office",
		"docs/report.pdf":   "pdf",
		"images/screen.PNG": "image",
		"drawings/site.DWG": "cad",
	}
	for path, kind := range want {
		got, ok := byPath[path]
		if !ok {
			t.Fatalf("missing %s in %#v", path, files)
		}
		if got.Kind != kind {
			t.Fatalf("%s kind = %q, want %q", path, got.Kind, kind)
		}
	}
	if _, ok := byPath["skip.txt"]; ok {
		t.Fatal("unsupported .txt file was listed")
	}
	if _, ok := byPath[".hidden/secret.md"]; ok {
		t.Fatal("hidden directory file was listed")
	}
}

func TestOpenVaultFileRejectsEscapedPath(t *testing.T) {
	t.Parallel()
	app := NewApp(nil, nil, t.TempDir())
	if err := app.OpenVaultFile("../outside.pdf"); err == nil {
		t.Fatal("expected escaped path to be rejected")
	}
}
