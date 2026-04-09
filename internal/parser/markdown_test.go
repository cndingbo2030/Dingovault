package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cndingbo2030/dingovault/internal/domain"
)

func TestEngine_WikilinksTagsAndBlocks(t *testing.T) {
	const md = `# Title here

Intro paragraph with [[Target Page|shown]] and #vault #go-lang.

- First item #itemtag
- Second with [[Bare]]
  - Nested child
`

	e := NewEngine()
	res, err := e.ParseSource([]byte(md), "/tmp/note.md")
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Blocks) < 4 {
		t.Fatalf("expected several blocks, got %d", len(res.Blocks))
	}

	var titles, intros, items int
	for _, b := range res.Blocks {
		switch {
		case strings.HasPrefix(b.Content, "Title"):
			titles++
			if b.Metadata.LineStart < 1 {
				t.Errorf("heading line: %+v", b.Metadata)
			}
		case strings.HasPrefix(b.Content, "Intro"):
			intros++
		case strings.HasPrefix(b.Content, "First"):
			items++
		case strings.HasPrefix(b.Content, "Second"):
			items++
		case strings.HasPrefix(b.Content, "Nested"):
			items++
		}
	}
	if titles != 1 || intros != 1 || items != 3 {
		t.Fatalf("block kind counts titles=%d intros=%d items=%d", titles, intros, items)
	}

	var targets []string
	for _, w := range res.Wikilinks {
		targets = append(targets, w.Target)
	}
	if !contains(targets, "Target Page") || !contains(targets, "Bare") {
		t.Fatalf("wikilinks: %+v", res.Wikilinks)
	}

	tags := map[string]bool{}
	for _, tg := range res.Tags {
		tags[tg.Tag] = true
	}
	for _, need := range []string{"vault", "go-lang", "itemtag"} {
		if !tags[need] {
			t.Fatalf("missing tag %q in %+v", need, res.Tags)
		}
	}

	var nested *domain.Block
	for i := range res.Blocks {
		if strings.HasPrefix(res.Blocks[i].Content, "Nested") {
			nested = &res.Blocks[i]
			break
		}
	}
	if nested == nil {
		t.Fatal("nested block not found")
	}
	if nested.ParentID == "" {
		t.Fatal("nested list item should have ParentID set")
	}
}

func TestEngine_ListItemsHaveDistinctIDs(t *testing.T) {
	var b strings.Builder
	for j := range 80 {
		fmt.Fprintf(&b, "- item %d\n", j)
	}
	e := NewEngine()
	res, err := e.ParseSource([]byte(b.String()), "/tmp/long-list.md")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{}
	for _, bl := range res.Blocks {
		if _, dup := seen[bl.ID]; dup {
			t.Fatalf("duplicate block id %s (line %d-%d)", bl.ID, bl.Metadata.LineStart, bl.Metadata.LineEnd)
		}
		seen[bl.ID] = struct{}{}
	}
	if len(seen) != len(res.Blocks) {
		t.Fatalf("id count mismatch")
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func BenchmarkParse1000SmallFiles(b *testing.B) {
	e := NewEngine()
	src := []byte("# x\n\npara with [[Link]] and #tag\n\n- a\n- b\n")
	path := "/bench/x.md"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			if _, err := e.ParseSource(src, path); err != nil {
				b.Fatal(err)
			}
		}
	}
}
