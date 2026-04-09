// Command: go run ./scripts/benchmark.go [-dir DIR] [-files 50] [-total 10000]
// Generates randomized markdown, indexes into a temp SQLite DB, and reports FTS + GetPage latency.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dingbo/dingovault/internal/graph"
	"github.com/dingbo/dingovault/internal/parser"
	"github.com/dingbo/dingovault/internal/storage"
	"github.com/dingbo/dingovault/internal/tenant"
)

const ftsSeedToken = "dvbenchtoken"

func main() {
	dirFlag := flag.String("dir", "", "vault directory (default: temp dir)")
	nFiles := flag.Int("files", 50, "number of markdown files")
	nTotal := flag.Int("total", 10000, "approximate total list items across all files")
	flag.Parse()

	dir := *dirFlag
	if dir == "" {
		d, err := os.MkdirTemp("", "dingovault-bench-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
			os.Exit(1)
		}
		dir = d
		defer func() { _ = os.RemoveAll(dir) }()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	perFile := *nTotal / *nFiles
	if perFile < 1 {
		perFile = 1
	}

	rng := rand.New(rand.NewPCG(1, 2))
	words := []string{"alpha", "beta", "gamma", "delta", "omega", "note", "task", "idea", ftsSeedToken}
	var samplePath string

	for i := range *nFiles {
		name := filepath.Join(dir, fmt.Sprintf("bench-%02d.md", i))
		var b strings.Builder
		// List-only body avoids duplicate StableBlockID edge cases from heading+list overlaps in Goldmark.
		for j := range perFile {
			w := words[rng.IntN(len(words))]
			fmt.Fprintf(&b, "- item %d %s %x\n", j, w, rng.Uint64()&0xfff)
		}
		if err := os.WriteFile(name, []byte(b.String()), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", name, err)
			os.Exit(1)
		}
		if i == 0 {
			samplePath = name
		}
	}

	dbPath := filepath.Join(dir, "bench.db")
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "remove db: %v\n", err)
		os.Exit(1)
	}

	store, err := storage.OpenSQLite(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sqlite: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	g := graph.NewService(store, parser.NewEngine())
	ctx := tenant.WithUserID(context.Background(), tenant.LocalUserID)

	t0 := time.Now()
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return err
		}
		return g.ReindexFile(ctx, path)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "index: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Indexed %d files (~%d blocks) in %s\n", *nFiles, perFile**nFiles, time.Since(t0).Round(time.Millisecond))

	query := ftsSeedToken
	const ftsRuns = 80
	var ftsDur []time.Duration
	for range ftsRuns {
		t1 := time.Now()
		_, err := store.SearchBlocksFTS(ctx, query, 50)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fts: %v\n", err)
			os.Exit(1)
		}
		ftsDur = append(ftsDur, time.Since(t1))
	}
	fmt.Printf("SearchBlocks FTS %q: p50=%s p95=%s (n=%d)\n", query, percentile(ftsDur, 50), percentile(ftsDur, 95), ftsRuns)

	const pageRuns = 120
	var pageDur []time.Duration
	for range pageRuns {
		t1 := time.Now()
		_, err := store.ListDomainBlocksBySourcePath(ctx, samplePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "getpage: %v\n", err)
			os.Exit(1)
		}
		pageDur = append(pageDur, time.Since(t1))
	}
	fmt.Printf("ListDomainBlocksBySourcePath (1 file, ~%d blocks): p50=%s p95=%s (n=%d)\n",
		perFile, percentile(pageDur, 50), percentile(pageDur, 95), pageRuns)

	slow := false
	if p50 := parseDurMs(percentile(ftsDur, 50)); p50 > 50 {
		fmt.Println("NOTE: FTS p50 > 50ms — consider PRAGMA optimize; ensure WAL; warm OS page cache.")
		slow = true
	}
	if p50 := parseDurMs(percentile(pageDur, 50)); p50 > 50 {
		fmt.Println("NOTE: GetPage p50 > 50ms — idx_blocks_source_line helps ORDER BY line_start per file.")
		slow = true
	}
	if !slow {
		fmt.Println("All measured p50 latencies are ≤ 50ms on this machine.")
	}
}

func percentile(durs []time.Duration, p int) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	cp := slices.Clone(durs)
	slices.Sort(cp)
	idx := (len(cp) - 1) * p / 100
	return cp[idx]
}

func parseDurMs(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}
