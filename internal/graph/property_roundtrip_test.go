package graph

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

func TestReindexDoesNotRewriteOrdinaryDoubleColonBlocks(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "p.md")
	body := "- note:: ordinary prose\n- https://example.test/a::b stays prose\n- ratio 3::1 stays prose\n"
	if err := os.WriteFile(mdPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.OpenSQLite(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	svc := NewService(store, parser.NewEngine())
	ctx := tenant.WithUserID(context.Background(), tenant.LocalUserID)
	if err := svc.ReindexFile(ctx, mdPath); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Fatalf("reindex rewrote markdown:\n%s", string(raw))
	}
	blocks, err := store.ListDomainBlocksBySourcePath(ctx, mdPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range blocks {
		if len(b.Properties) != 0 {
			t.Fatalf("ordinary block %q parsed as properties: %+v", b.Content, b.Properties)
		}
	}
}
