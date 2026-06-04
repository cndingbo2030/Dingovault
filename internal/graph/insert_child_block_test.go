package graph

import (
	"os"
	"testing"
)

func TestInsertChildBlock_AppendsAfterExistingSubtree(t *testing.T) {
	svc, mdPath, ids, ctx := moveBlockFixture(t, "- a\n  - b\n- c\n")

	if err := svc.InsertChildBlock(ctx, ids["a"], "new"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "- a\n  - b\n  - new\n- c\n"
	if string(got) != want {
		t.Fatalf("file = %q, want %q", string(got), want)
	}
}

func TestInsertChildBlock_MultilineContent(t *testing.T) {
	svc, mdPath, ids, ctx := moveBlockFixture(t, "- a\n- c\n")

	if err := svc.InsertChildBlock(ctx, ids["a"], "terminal result\nsource: terminal\nexitCode: 0"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "- a\n  - terminal result\n    source: terminal\n    exitCode: 0\n- c\n"
	if string(got) != want {
		t.Fatalf("file = %q, want %q", string(got), want)
	}
}
