package export

import (
	"bytes"
	"testing"
)

func TestMarkdownFileToStandaloneHTML(t *testing.T) {
	b, err := MarkdownFileToStandaloneHTML([]byte("---\ntitle: X\n---\n\n# Hi\n"), "T")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("<h1")) || !bytes.Contains(b, []byte("Hi")) {
		t.Fatalf("%s", b)
	}
}
