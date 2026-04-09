package server

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/dingbo/dingovault/internal/auth"
	"github.com/dingbo/dingovault/internal/blob"
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

func handleAssetUpload(p blob.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if p == nil {
			http.Error(w, `{"error":"asset storage not configured (set vault path or S3 env)"}`, http.StatusServiceUnavailable)
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

		name := blob.SafePublicFileName(hdr.Filename)
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := allowedAssetExt[ext]; !ok {
			http.Error(w, `{"error":"unsupported file type"}`, http.StatusBadRequest)
			return
		}

		tenant := ""
		if c, ok := auth.ClaimsFromContext(r.Context()); ok && c != nil {
			tenant = strings.TrimSpace(c.Subject)
		}

		ct := hdr.Header.Get("Content-Type")
		res, err := p.Put(r.Context(), blob.PutInput{
			FileName:    name,
			Body:        file,
			Limit:       maxAssetUpload,
			ContentType: ct,
			TenantID:    tenant,
		})
		if err != nil {
			http.Error(w, `{"error":`+jsonString(err.Error())+`}`, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{
			"path":     res.Ref,
			"markdown": res.Markdown,
			"bytes":    res.Bytes,
		})
	}
}
