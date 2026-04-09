package blob

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Provider stores uploaded binary assets (images, PDFs) outside the Markdown vault tree when using S3/MinIO,
// or under vault/assets for local filesystem mode.
type Provider interface {
	Put(ctx context.Context, in PutInput) (PutResult, error)
}

// PutInput describes one upload (already size-limited by the HTTP handler).
type PutInput struct {
	FileName    string
	Body        io.Reader
	Limit       int64
	ContentType string
	TenantID    string // JWT subject; used as an S3 key prefix segment
}

// PutResult is returned after a successful Put.
type PutResult struct {
	Ref      string // logical ref (filesystem) or same as PublicURL (S3)
	Bytes    int64
	Markdown string // ready to paste into a .md file
}

// MarkdownLink builds an image or file markdown snippet.
func MarkdownLink(fileName, ref string, ext string) string {
	ext = strings.ToLower(ext)
	title := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	if title == "" {
		title = "file"
	}
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return fmt.Sprintf("![%s](%s)", title, ref)
	default:
		return fmt.Sprintf("[%s](%s)", title, ref)
	}
}
