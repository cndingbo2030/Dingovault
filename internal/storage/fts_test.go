package storage

import "testing"

func TestBuildFTS5MatchQuery(t *testing.T) {
	q, err := buildFTS5MatchQuery("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if q != "hello* AND world*" {
		t.Fatalf("got %q", q)
	}
	_, err = buildFTS5MatchQuery("   ")
	if err == nil {
		t.Fatal("expected error for empty terms")
	}
}
