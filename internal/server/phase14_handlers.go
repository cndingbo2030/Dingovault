package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dingbo/dingovault/internal/graph"
	"github.com/dingbo/dingovault/internal/storage"
)

type captureBody struct {
	Text       string `json:"text"`
	SourcePath string `json:"sourcePath"`
}

func handleCapture(g *graph.Service, vaultRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if vaultRoot == "" {
			http.Error(w, `{"error":"capture requires vault path (set -notes or vaultPath)"}`, http.StatusServiceUnavailable)
			return
		}
		var body captureBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		text := strings.TrimSpace(body.Text)
		if text == "" {
			http.Error(w, `{"error":"text required"}`, http.StatusBadRequest)
			return
		}
		rel := strings.TrimSpace(body.SourcePath)
		if rel == "" {
			rel = "Inbox.md"
		}
		abs, err := graph.ResolveVaultPath(vaultRoot, rel)
		if err != nil {
			http.Error(w, `{"error":`+jsonString(err.Error())+`}`, http.StatusBadRequest)
			return
		}
		if err := g.AppendQuickCapture(r.Context(), abs, text); err != nil {
			http.Error(w, `{"error":`+jsonString(err.Error())+`}`, http.StatusBadRequest)
			return
		}
		markdown := "- " + strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", " ")
		writeJSON(w, map[string]any{
			"sourcePath": rel,
			"absPath":    abs,
			"markdown":   markdown + "\n",
		})
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func handleWikiGraph(store storage.Provider, vaultRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if vaultRoot == "" {
			http.Error(w, `{"error":"graph requires vault path"}`, http.StatusServiceUnavailable)
			return
		}
		g, err := store.WikiGraph(r.Context(), vaultRoot)
		if err != nil {
			http.Error(w, `{"error":`+jsonString(err.Error())+`}`, http.StatusInternalServerError)
			return
		}
		if g.Nodes == nil {
			g.Nodes = []storage.WikiGraphNode{}
		}
		if g.Edges == nil {
			g.Edges = []storage.WikiGraphEdge{}
		}
		writeJSON(w, g)
	}
}

const maxAssetUpload = 32 << 20

var allowedAssetExt = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}, ".svg": {}, ".pdf": {},
}

func handleAssetUpload(vaultRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if vaultRoot == "" {
			http.Error(w, `{"error":"upload requires vault path"}`, http.StatusServiceUnavailable)
			return
		}
		if err := r.ParseMultipartForm(maxAssetUpload); err != nil {
			http.Error(w, `{"error":"invalid multipart form"}`, http.StatusBadRequest)
			return
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"missing file field"}`, http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()

		name := safeAssetFilename(hdr.Filename)
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := allowedAssetExt[ext]; !ok {
			http.Error(w, `{"error":"unsupported file type"}`, http.StatusBadRequest)
			return
		}

		destDir := filepath.Join(vaultRoot, "assets")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			http.Error(w, `{"error":"cannot create assets dir"}`, http.StatusInternalServerError)
			return
		}
		dest := filepath.Join(destDir, name)
		dst, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			http.Error(w, `{"error":"cannot write file (exists or permission)"}`, http.StatusInternalServerError)
			return
		}
		n, err := io.Copy(dst, io.LimitReader(file, maxAssetUpload))
		_ = dst.Close()
		if err != nil {
			_ = os.Remove(dest)
			http.Error(w, `{"error":"save failed"}`, http.StatusInternalServerError)
			return
		}
		if n == 0 {
			_ = os.Remove(dest)
			http.Error(w, `{"error":"empty file"}`, http.StatusBadRequest)
			return
		}

		rel := "assets/" + filepath.ToSlash(name)
		md := assetMarkdownLink(rel, name, ext)
		writeJSON(w, map[string]any{
			"path":     rel,
			"markdown": md,
			"bytes":    n,
		})
	}
}

func safeAssetFilename(name string) string {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, "..", "_")
	base = strings.TrimSpace(base)
	if base == "" || base == "." {
		return "upload.bin"
	}
	return base
}

func assetMarkdownLink(rel, filename, ext string) string {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return fmt.Sprintf("![%s](%s)", linkTitle(filename), rel)
	default:
		return fmt.Sprintf("[%s](%s)", linkTitle(filename), rel)
	}
}

func linkTitle(filename string) string {
	t := strings.TrimSuffix(filename, filepath.Ext(filename))
	if t == "" {
		return "file"
	}
	return t
}
