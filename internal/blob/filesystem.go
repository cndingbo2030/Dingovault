package blob

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileSystem stores assets under vaultRoot/assets (classic Dingovault layout).
type FileSystem struct {
	vaultRoot string
}

// NewFileSystem returns a Provider that writes under vaultRoot/assets.
func NewFileSystem(vaultRoot string) *FileSystem {
	return &FileSystem{vaultRoot: filepath.Clean(vaultRoot)}
}

// Put implements Provider.
func (f *FileSystem) Put(ctx context.Context, in PutInput) (PutResult, error) {
	_ = ctx
	name := SafePublicFileName(in.FileName)
	destDir := filepath.Join(f.vaultRoot, "assets")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return PutResult{}, fmt.Errorf("mkdir assets: %w", err)
	}
	dest := filepath.Join(destDir, name)
	dst, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return PutResult{}, fmt.Errorf("create asset file: %w", err)
	}
	n, err := io.Copy(dst, io.LimitReader(in.Body, in.Limit))
	_ = dst.Close()
	if err != nil {
		_ = os.Remove(dest)
		return PutResult{}, fmt.Errorf("write asset: %w", err)
	}
	if n == 0 {
		_ = os.Remove(dest)
		return PutResult{}, fmt.Errorf("empty upload")
	}
	rel := "assets/" + filepath.ToSlash(name)
	ext := strings.ToLower(filepath.Ext(name))
	return PutResult{
		Ref:      rel,
		Bytes:    n,
		Markdown: MarkdownLink(name, rel, ext) + "\n",
	}, nil
}

// SafePublicFileName returns a single path segment safe for public asset URLs and local disk.
func SafePublicFileName(name string) string {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, "..", "_")
	base = strings.TrimSpace(base)
	if base == "" || base == "." {
		return "upload.bin"
	}
	return base
}
