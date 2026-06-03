package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestWikiGraphDoesNotBlockResolvingAliases(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wiki.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	from := filepath.Join(dir, "A.md")
	to := filepath.Join(dir, "B.md")
	now := time.Now().UnixNano()
	if _, err := store.db.Exec(
		`INSERT INTO blocks (id, user_id, content, source_path, created_at, updated_at) VALUES (?, 'local', ?, ?, ?, ?)`,
		"a1", "[[B]]", from, now, now,
	); err != nil {
		t.Fatalf("seed source block: %v", err)
	}
	if _, err := store.db.Exec(
		`INSERT INTO blocks (id, user_id, content, source_path, created_at, updated_at) VALUES (?, 'local', ?, ?, ?, ?)`,
		"b1", "target", to, now, now,
	); err != nil {
		t.Fatalf("seed target block: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO block_wikilinks (source_block_id, target) VALUES (?, ?)`, "a1", "B"); err != nil {
		t.Fatalf("seed wikilink: %v", err)
	}
	if _, err := store.db.Exec(
		`INSERT INTO page_aliases (user_id, alias_normalized, source_path) VALUES ('local', ?, ?)`,
		"b", to,
	); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	graph, err := store.WikiGraph(ctx, dir)
	if err != nil {
		t.Fatalf("wiki graph: %v", err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2: %#v", len(graph.Nodes), graph.Nodes)
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("edges = %d, want 1: %#v", len(graph.Edges), graph.Edges)
	}
	if graph.Edges[0].Source != from || graph.Edges[0].Target != to {
		t.Fatalf("edge = %#v, want %q -> %q", graph.Edges[0], from, to)
	}
}

func TestWikiGraphResolvesIndexedMarkdownTargets(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wiki.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	from := filepath.Join(dir, "README.md")
	to := filepath.Join(dir, "Features.md")
	now := time.Now().UnixNano()
	if _, err := store.db.Exec(
		`INSERT INTO blocks (id, user_id, content, source_path, created_at, updated_at) VALUES (?, 'local', ?, ?, ?, ?)`,
		"readme1", "[[Features.md]]", from, now, now,
	); err != nil {
		t.Fatalf("seed source block: %v", err)
	}
	if _, err := store.db.Exec(
		`INSERT INTO blocks (id, user_id, content, source_path, created_at, updated_at) VALUES (?, 'local', ?, ?, ?, ?)`,
		"features1", "target", to, now, now,
	); err != nil {
		t.Fatalf("seed target block: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO block_wikilinks (source_block_id, target) VALUES (?, ?)`, "readme1", "Features.md"); err != nil {
		t.Fatalf("seed wikilink: %v", err)
	}

	graph, err := store.WikiGraph(context.Background(), dir)
	if err != nil {
		t.Fatalf("wiki graph: %v", err)
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("edges = %d, want 1: %#v", len(graph.Edges), graph.Edges)
	}
	if graph.Edges[0].Source != from || graph.Edges[0].Target != to {
		t.Fatalf("edge = %#v, want %q -> %q", graph.Edges[0], from, to)
	}
}
