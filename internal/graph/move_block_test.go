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

func TestMoveBlockUnder_ListItems(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		movingText string
		parentText string
		want       string
		wantErr    string
	}{
		{
			name:       "root sibling becomes last child",
			body:       "- a\n  - b\n- c\n",
			movingText: "c",
			parentText: "a",
			want:       "- a\n  - b\n  - c\n",
		},
		{
			name:       "nested child moves under another root",
			body:       "- a\n  - b\n- c\n",
			movingText: "b",
			parentText: "c",
			want:       "- a\n- c\n  - b\n",
		},
		{
			name:       "moving under descendant is rejected",
			body:       "- a\n  - b\n- c\n",
			movingText: "a",
			parentText: "b",
			wantErr:    "own descendant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, mdPath, ids, ctx := moveBlockFixture(t, tt.body)

			err := svc.MoveBlockUnder(ctx, ids[tt.movingText], ids[tt.parentText])
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(mdPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("file = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func moveBlockFixture(t *testing.T, body string) (*Service, string, map[string]string, context.Context) {
	t.Helper()
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "p.md")
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

	blocks, err := store.ListDomainBlocksBySourcePath(ctx, mdPath)
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]string, len(blocks))
	for _, b := range blocks {
		ids[strings.TrimSpace(b.Content)] = b.ID
	}
	return svc, mdPath, ids, ctx
}
