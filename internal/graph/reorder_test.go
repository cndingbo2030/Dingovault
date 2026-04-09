package graph

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

func TestReorderSiblingBefore_ListItems(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	md := filepath.Join(dir, "p.md")
	body := "- a\n- b\n- c\n"
	if err := os.WriteFile(md, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(dir, "t.db")
	store, err := storage.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	eng := parser.NewEngine()
	svc := NewService(store, eng)
	ctx := tenant.WithUserID(context.Background(), tenant.LocalUserID)
	if err := svc.ReindexFile(ctx, md); err != nil {
		t.Fatal(err)
	}

	blocks, err := store.ListDomainBlocksBySourcePath(ctx, md)
	if err != nil {
		t.Fatal(err)
	}
	var idA, idC string
	for _, b := range blocks {
		switch strings.TrimSpace(b.Content) {
		case "a":
			idA = b.ID
		case "c":
			idC = b.ID
		}
	}
	if idA == "" || idC == "" {
		t.Fatalf("blocks not found: %+v", blocks)
	}

	// Move "c" before "a" → c, a, b
	if err := svc.ReorderSiblingBefore(ctx, idC, idA); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(md)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("unexpected file: %q", string(out))
	}
	if !strings.Contains(lines[0], "c") || !strings.Contains(lines[1], "a") {
		t.Fatalf("expected c then a, got %q", lines)
	}
}
