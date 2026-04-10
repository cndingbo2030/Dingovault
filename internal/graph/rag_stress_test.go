package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cndingbo2030/dingovault/internal/domain"
	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

// TestRAGStress_SearchWhileWritingEmbeddings runs concurrent semantic search while many embedding upserts
// are in flight (simulates chat/RAG during background indexing). Skipped under -short.
func TestRAGStress_SearchWhileWritingEmbeddings(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}
	t.Parallel()

	dir := t.TempDir()
	md := filepath.Join(dir, "stress.md")
	var body strings.Builder
	for i := range 220 {
		fmt.Fprintf(&body, "- item %d\n", i)
	}
	if err := os.WriteFile(md, []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(dir, "stress.db")
	store, err := storage.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := tenant.WithUserID(context.Background(), tenant.LocalUserID)
	eng := parser.NewEngine()
	svc := NewService(store, eng)
	if err := svc.ReindexFile(ctx, md); err != nil {
		t.Fatal(err)
	}

	blocks, err := store.ListDomainBlocksBySourcePath(ctx, md)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) < 100 {
		t.Fatalf("expected many blocks, got %d", len(blocks))
	}

	const model = "stress-model"
	q := make([]float32, 16)
	for i := range q {
		q[i] = float32(i+1) * 0.01
	}
	vec := make([]float32, 16)
	for i := range vec {
		vec[i] = 0.02
	}

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	stressSpawnSemanticReaders(&wg, errs, store, ctx, q, model, 24, 40)
	stressSpawnEmbeddingWriters(&wg, errs, store, ctx, blocks, model, vec, 16, 80)
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}

func stressSpawnSemanticReaders(wg *sync.WaitGroup, errs chan error, store *storage.Store, ctx context.Context, q []float32, model string, workers, iters int) {
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				if _, e := store.SearchSemantic(ctx, q, model, 12); e != nil {
					errs <- e
					return
				}
			}
		}()
	}
}

func stressSpawnEmbeddingWriters(wg *sync.WaitGroup, errs chan error, store *storage.Store, ctx context.Context, blocks []domain.Block, model string, vec []float32, workers, perWorker int) {
	for i := 0; i < workers; i++ {
		off := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range perWorker {
				idx := off*perWorker + j
				if idx >= len(blocks) {
					return
				}
				if e := store.UpsertBlockEmbedding(ctx, tenant.LocalUserID, blocks[idx].ID, model, vec); e != nil {
					errs <- e
					return
				}
			}
		}()
	}
}
