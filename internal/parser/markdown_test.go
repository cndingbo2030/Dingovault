package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cndingbo2030/dingovault/internal/domain"
)

const wikilinksTagsBlocksMD = `# Title here

Intro paragraph with [[Target Page|shown]] and #vault #go-lang.

- First item #itemtag
- Second with [[Bare]]
  - Nested child
`

func parseWikilinksTagsBlocksFixture(t *testing.T) ParseResult {
	t.Helper()
	e := NewEngine()
	res, err := e.ParseSource([]byte(wikilinksTagsBlocksMD), "/tmp/note.md")
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestEngine_WikilinksTagsAndBlocks_Blocks(t *testing.T) {
	res := parseWikilinksTagsBlocksFixture(t)
	if len(res.Blocks) < 4 {
		t.Fatalf("expected several blocks, got %d", len(res.Blocks))
	}
	assertBlockKinds(t, res.Blocks)
	assertNestedParentID(t, res.Blocks)
}

func TestEngine_WikilinksTagsAndBlocks_Wikilinks(t *testing.T) {
	res := parseWikilinksTagsBlocksFixture(t)
	assertWikilinks(t, res)
}

func TestEngine_WikilinksTagsAndBlocks_Tags(t *testing.T) {
	res := parseWikilinksTagsBlocksFixture(t)
	assertTags(t, res)
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

func assertBlockKinds(t *testing.T, blocks []domain.Block) {
	t.Helper()
	var titles, intros, items int
	for _, b := range blocks {
		switch {
		case strings.HasPrefix(b.Content, "Title"):
			titles++
			if b.Metadata.LineStart < 1 {
				t.Errorf("heading line: %+v", b.Metadata)
			}
		case strings.HasPrefix(b.Content, "Intro"):
			intros++
		case strings.HasPrefix(b.Content, "First"), strings.HasPrefix(b.Content, "Second"), strings.HasPrefix(b.Content, "Nested"):
			items++
		}
	}
	if titles != 1 || intros != 1 || items != 3 {
		t.Fatalf("block kind counts titles=%d intros=%d items=%d", titles, intros, items)
	}
}

func assertWikilinks(t *testing.T, res ParseResult) {
	t.Helper()
	var targets []string
	for _, w := range res.Wikilinks {
		targets = append(targets, w.Target)
	}
	if !contains(targets, "Target Page") || !contains(targets, "Bare") {
		t.Fatalf("wikilinks: %+v", res.Wikilinks)
	}
}

func assertTags(t *testing.T, res ParseResult) {
	t.Helper()
	tags := map[string]bool{}
	for _, tg := range res.Tags {
		tags[tg.Tag] = true
	}
	for _, need := range []string{"vault", "go-lang", "itemtag"} {
		if !tags[need] {
			t.Fatalf("missing tag %q in %+v", need, res.Tags)
		}
	}
}

func assertNestedParentID(t *testing.T, blocks []domain.Block) {
	t.Helper()
	var nested *domain.Block
	for i := range blocks {
		if strings.HasPrefix(blocks[i].Content, "Nested") {
			nested = &blocks[i]
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
