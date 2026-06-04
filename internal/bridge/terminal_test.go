package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

func TestRunBlockCommandRejectsUnconfirmedUnsafeCommand(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
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

	_, err = app.RunBlockCommand(blocks[0].ID, fmt.Sprintf("printf hacked > %s", marker), "", false)
	if err == nil || !strings.Contains(err.Error(), "requires confirmation") {
		t.Fatalf("error = %v, want confirmation requirement", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("marker stat error = %v, want file not created", statErr)
	}
	after, err := store.ListDomainBlocksBySourcePath(ctx, mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("blocks after rejected command = %d, want 1", len(after))
	}
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "- run command\n" {
		t.Fatalf("markdown was modified after rejected command:\n%s", string(raw))
	}
}
