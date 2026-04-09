package blob

import (
	"context"
	"strings"
	"testing"
)

func TestFileSystemPut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fs := NewFileSystem(dir)
	r := strings.NewReader("hello")
	res, err := fs.Put(context.Background(), PutInput{
		FileName: "note.txt",
		Body:     r,
		Limit:    1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Bytes != 5 {
		t.Fatalf("bytes %d", res.Bytes)
	}
	if !strings.Contains(res.Ref, "assets/") {
		t.Fatalf("ref %q", res.Ref)
	}
}
