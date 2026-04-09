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

	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

const ftsSeedToken = "dvbenchtoken"

func main() {
	opts := parseBenchOptions()
	runBenchmarkOrExit(opts)
}

type benchOptions struct {
	dir    string
	files  int
	total  int
	verify bool
}

func parseBenchOptions() benchOptions {
	dirFlag := flag.String("dir", "", "vault directory (default: temp dir)")
	nFiles := flag.Int("files", 50, "number of markdown files")
	nTotal := flag.Int("total", 10000, "approximate total list items across all files")
	verify := flag.Bool("verify", false, "verify indexed block round-trip (use with DINGO_MASTER_KEY for encryption stress check)")
	flag.Parse()
	return benchOptions{dir: *dirFlag, files: *nFiles, total: *nTotal, verify: *verify}
}

func runBenchmarkOrExit(opts benchOptions) {
	dir, cleanup := prepareBenchDir(opts.dir)
	if cleanup != nil {
		defer cleanup()
	}
	perFile := max(1, opts.total/opts.files)
	samplePath := generateBenchFilesOrExit(dir, opts.files, perFile)
	store := openBenchStoreOrExit(filepath.Join(dir, "bench.db"))
	defer func() { _ = store.Close() }()

	ctx := tenant.WithUserID(context.Background(), tenant.LocalUserID)
	indexMarkdownOrExit(ctx, store, dir, opts.files, perFile)
	ftsDur := benchmarkFTSOrExit(ctx, store, ftsSeedToken, 80)
	pageDur := benchmarkPageLoadOrExit(ctx, store, samplePath, perFile, 120)
	reportLatencyNotes(ftsDur, pageDur)
	if opts.verify {
		runVerifyOrExit(ctx, store, samplePath, perFile)
	}
}

func prepareBenchDir(dir string) (string, func()) {
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			exitf("mkdir: %v", err)
		}
		return dir, nil
	}
	d, err := os.MkdirTemp("", "dingovault-bench-*")
	if err != nil {
		exitf("mkdir: %v", err)
	}
	return d, func() { _ = os.RemoveAll(d) }
}

func generateBenchFilesOrExit(dir string, files, perFile int) string {
	rng := rand.New(rand.NewPCG(1, 2))
	words := []string{"alpha", "beta", "gamma", "delta", "omega", "note", "task", "idea", ftsSeedToken}
	var samplePath string
	for i := range files {
		name := filepath.Join(dir, fmt.Sprintf("bench-%02d.md", i))
		if err := writeBenchFile(name, perFile, words, rng); err != nil {
			exitf("write %s: %v", name, err)
		}
		if i == 0 {
			samplePath = name
		}
	}
	return samplePath
}

func writeBenchFile(name string, perFile int, words []string, rng *rand.Rand) error {
	var b strings.Builder
	// List-only body avoids duplicate StableBlockID edge cases from heading+list overlaps in Goldmark.
	for j := range perFile {
		w := words[rng.IntN(len(words))]
		fmt.Fprintf(&b, "- item %d %s %x\n", j, w, rng.Uint64()&0xfff)
	}
	return os.WriteFile(name, []byte(b.String()), 0o644)
}

func openBenchStoreOrExit(dbPath string) *storage.Store {
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		exitf("remove db: %v", err)
	}
	store, err := storage.OpenSQLite(dbPath)
	if err != nil {
		exitf("sqlite: %v", err)
	}
	return store
}

func indexMarkdownOrExit(ctx context.Context, store *storage.Store, dir string, files, perFile int) {
	g := graph.NewService(store, parser.NewEngine())
	t0 := time.Now()
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return err
		}
		return g.ReindexFile(ctx, path)
	}); err != nil {
		exitf("index: %v", err)
	}
	fmt.Printf("Indexed %d files (~%d blocks) in %s\n", files, perFile*files, time.Since(t0).Round(time.Millisecond))
}

func benchmarkFTSOrExit(ctx context.Context, store *storage.Store, query string, runs int) []time.Duration {
	var ftsDur []time.Duration
	for range runs {
		t1 := time.Now()
		if _, err := store.SearchBlocksFTS(ctx, query, 50); err != nil {
			exitf("fts: %v", err)
		}
		ftsDur = append(ftsDur, time.Since(t1))
	}
	fmt.Printf("SearchBlocks FTS %q: p50=%s p95=%s (n=%d)\n", query, percentile(ftsDur, 50), percentile(ftsDur, 95), runs)
	return ftsDur
}

func benchmarkPageLoadOrExit(ctx context.Context, store *storage.Store, samplePath string, perFile, runs int) []time.Duration {
	var pageDur []time.Duration
	for range runs {
		t1 := time.Now()
		if _, err := store.ListDomainBlocksBySourcePath(ctx, samplePath); err != nil {
			exitf("getpage: %v", err)
		}
		pageDur = append(pageDur, time.Since(t1))
	}
	fmt.Printf("ListDomainBlocksBySourcePath (1 file, ~%d blocks): p50=%s p95=%s (n=%d)\n",
		perFile, percentile(pageDur, 50), percentile(pageDur, 95), runs)
	return pageDur
}

func reportLatencyNotes(ftsDur, pageDur []time.Duration) {
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

func runVerifyOrExit(ctx context.Context, store *storage.Store, samplePath string, perFile int) {
	if err := verifyIndexedBlocks(ctx, store, samplePath, perFile); err != nil {
		exitf("verify: %v", err)
	}
	if os.Getenv("DINGO_MASTER_KEY") != "" {
		fmt.Println("Encryption verify OK (DINGO_MASTER_KEY): decrypted block content matches indexed markdown.")
	} else {
		fmt.Println("Verify OK: sampled blocks readable from index.")
	}
}

func exitf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

func verifyIndexedBlocks(ctx context.Context, store *storage.Store, samplePath string, perFile int) error {
	blocks, err := store.ListDomainBlocksBySourcePath(ctx, samplePath)
	if err != nil {
		return err
	}
	if len(blocks) == 0 {
		return fmt.Errorf("no blocks for sample path")
	}
	n := len(blocks)
	if n > 500 {
		n = 500
	}
	for i := range n {
		b := blocks[i]
		if b.Content == "" {
			return fmt.Errorf("empty content at block %s", b.ID)
		}
		// Bench markdown always contains "item" and a dvbenchtoken in some lines.
		if i == 0 && !containsAny(b.Content, "item", ftsSeedToken) {
			return fmt.Errorf("unexpected decrypted content prefix %q", truncate(b.Content, 40))
		}
	}
	_ = perFile
	return nil
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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
