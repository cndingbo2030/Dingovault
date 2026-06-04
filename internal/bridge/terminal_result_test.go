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
	app, store, mdPath, blockID, ctx := terminalResultFixture(t)
	result, err := app.RunBlockCommand(blockID, "printf dingovault-p1; exit 1", "", true)
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
	if bySource[0].Properties["source"] != "terminal" || bySource[0].Properties["exitCode"] != "1" || bySource[0].Properties["runId"] == "" {
		t.Fatalf("properties = %+v", bySource[0].Properties)
	}

	raw, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"properties:", "runId::", "source:: terminal", "exitCode:: 1", "durationMs::", "command:: printf dingovault-p1; exit 1", "```text", "dingovault-p1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("written markdown missing %q:\n%s", want, text)
		}
	}

	blocks, err := store.ListDomainBlocksBySourcePath(ctx, mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("blocks after append = %d, want parent + result", len(blocks))
	}
}

func TestRunBlockCommandAppendsDistinctHistoryRecords(t *testing.T) {
	app, _, mdPath, blockID, _ := terminalResultFixture(t)
	for range 2 {
		if _, err := app.RunBlockCommand(blockID, "printf same-output", "", true); err != nil {
			t.Fatal(err)
		}
	}

	results, err := app.QueryBlocks("source:terminal")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("terminal result count = %d, want 2", len(results))
	}
	seen := map[string]bool{}
	for _, result := range results {
		if result.Properties["runId"] == "" {
			t.Fatalf("missing runId in %+v", result.Properties)
		}
		if seen[result.Properties["runId"]] {
			t.Fatalf("duplicate runId %q", result.Properties["runId"])
		}
		seen[result.Properties["runId"]] = true
	}
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Count(text, "Terminal result") != 2 || strings.Count(text, "runId::") != 2 {
		t.Fatalf("duplicate command should append two explicit history blocks:\n%s", text)
	}
}

func terminalResultFixture(t *testing.T) (*App, *storage.Store, string, string, context.Context) {
	t.Helper()
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
	return app, store, mdPath, blocks[0].ID, ctx
}
