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

func TestAppendQuickCapture_NewInbox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "t.db")
	store, err := storage.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	eng := parser.NewEngine()
	svc := NewService(store, eng)
	ctx := tenant.WithUserID(context.Background(), tenant.LocalUserID)

	inbox := filepath.Join(dir, "Inbox.md")
	if err := svc.AppendQuickCapture(ctx, inbox, "hello capture"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(inbox)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "- hello capture") {
		t.Fatalf("expected bullet in file, got %q", string(raw))
	}
}
