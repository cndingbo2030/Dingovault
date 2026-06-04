package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

func TestRunBlockCommandAppendsQueryableResult(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "p.md")
	if err := os.WriteFile(mdPath, []byte("- run command\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.OpenSQLite(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	svc := graph.NewService(store, parser.NewEngine())
	ctx := tenant.WithUserID(context.Background(), tenant.LocalUserID)
	if err := svc.ReindexFile(ctx, mdPath); err != nil {
		t.Fatal(err)
	}
	blocks, err := store.ListDomainBlocksBySourcePath(ctx, mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(blocks))
	}

	app := NewApp(store, svc, dir)
	result, err := app.RunBlockCommand(blocks[0].ID, "printf dingovault-p1; exit 1", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("exitCode = %d, want 1; output=%q", result.ExitCode, result.Output)
	}

	bySource, err := app.QueryBlocks("source:terminal")
	if err != nil {
		t.Fatal(err)
	}
	byExit, err := app.QueryBlocks("exitCode:1")
	if err != nil {
		t.Fatal(err)
	}
	if len(bySource) != 1 || len(byExit) != 1 {
		t.Fatalf("query counts source=%d exit=%d, want 1/1", len(bySource), len(byExit))
	}
	if bySource[0].Properties["source"] != "terminal" || bySource[0].Properties["exitCode"] != "1" {
		t.Fatalf("properties = %+v", bySource[0].Properties)
	}

	raw, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"source:: terminal", "exitCode:: 1", "durationMs::", "command:: printf dingovault-p1; exit 1", "```text", "dingovault-p1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("written markdown missing %q:\n%s", want, text)
		}
	}
}
