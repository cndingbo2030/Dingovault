package graph

import (
	"strings"
	"testing"
)

func TestCollectSubtreeLineIndices_singleListItem(t *testing.T) {
	lines := []string{"- a", "- b", "- c"}
	idx := collectSubtreeLineIndices(lines, 2, 2)
	if len(idx) != 1 || idx[0] != 1 {
		t.Fatalf("got %v", idx)
	}
}

func TestCollectSubtreeLineIndices_withNestedChild(t *testing.T) {
	lines := []string{"- a", "- b", "  - c", "- d"}
	idx := collectSubtreeLineIndices(lines, 2, 2)
	want := []int{1, 2}
	if len(idx) != len(want) {
		t.Fatalf("got %v want %v", idx, want)
	}
	for i := range want {
		if idx[i] != want[i] {
			t.Fatalf("got %v want %v", idx, want)
		}
	}
}

func TestIndentBlock_roundTripFile(t *testing.T) {
	// Integration-style: adjust lines like the service would.
	src := "- a\n- b\n  - c\n- d\n"
	lines, _, _, err := splitFileLines([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	idx := collectSubtreeLineIndices(lines, 2, 2)
	lines2, err := applyIndentShift(lines, idx, 2)
	if err != nil {
		t.Fatal(err)
	}
	out := string(joinFileLines(lines2, "\n", true))
	if !strings.Contains(out, "  - b") || !strings.Contains(out, "    - c") {
		t.Fatalf("indent failed:\n%s", out)
	}
	if !strings.Contains(out, "- a") || !strings.HasPrefix(strings.TrimSpace(strings.Split(out, "\n")[3]), "- d") {
		t.Fatalf("sibling leaked:\n%s", out)
	}
	lines3, err := applyIndentShift(lines2, idx, -2)
	if err != nil {
		t.Fatal(err)
	}
	back := string(joinFileLines(lines3, "\n", true))
	if back != src {
		t.Fatalf("round trip:\nwant %q\ngot %q", src, back)
	}
}

func TestOutdentBlock_rootFails(t *testing.T) {
	lines := []string{"- x"}
	idx := []int{0}
	_, err := applyIndentShift(lines, idx, -2)
	if err == nil {
		t.Fatal("expected error")
	}
}
