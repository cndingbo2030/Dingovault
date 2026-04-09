package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/dingbo/dingovault/internal/auth"
	"github.com/dingbo/dingovault/internal/blob"
	"github.com/dingbo/dingovault/internal/graph"
	"github.com/dingbo/dingovault/internal/storage"
)

// MountAPI registers SaaS REST routes on mux. Public: /api/v1/health, /api/v1/auth/token.
// All other /api/v1/* routes require Authorization: Bearer <JWT>.
// If graphSvc is non-nil, POST /api/v1/pages/reindex is registered (markdown → index).
// vaultRoot is the indexed notes directory (absolute). When empty, capture and graph return 503.
// assets may use filesystem under vaultRoot or S3/MinIO via blobProv when non-nil.
func MountAPI(mux *http.ServeMux, store storage.Provider, j *auth.JWT, graphSvc *graph.Service, vaultRoot string, assets blob.Provider) {
	mux.HandleFunc("GET /api/v1/health", handleHealth)
	mux.HandleFunc("POST /api/v1/auth/token", handleAuthToken(j))

	protected := func(h http.Handler) http.Handler {
		return auth.BearerAuthMiddleware(j, h)
	}

	mux.Handle("GET /api/v1/blocks/{id}", protected(http.HandlerFunc(handleGetBlock(store))))
	mux.Handle("GET /api/v1/pages", protected(http.HandlerFunc(handleListPageBlocks(store))))
	mux.Handle("POST /api/v1/search", protected(http.HandlerFunc(handleSearch(store))))
	mux.Handle("POST /api/v1/search/fts", protected(http.HandlerFunc(handleSearchFTS(store))))
	mux.Handle("POST /api/v1/query/property", protected(http.HandlerFunc(handleQueryProperty(store))))
	mux.Handle("POST /api/v1/query/fts-ids", protected(http.HandlerFunc(handleFTSIDs(store))))
	mux.Handle("POST /api/v1/blocks/by-ids", protected(http.HandlerFunc(handleBlocksByIDs(store))))
	mux.Handle("GET /api/v1/paths/recent", protected(http.HandlerFunc(handleRecentPaths(store))))
	mux.Handle("POST /api/v1/page-properties/list", protected(http.HandlerFunc(handlePagePropsList(store))))
	mux.Handle("POST /api/v1/alias/resolve", protected(http.HandlerFunc(handleAliasResolve(store))))
	mux.Handle("POST /api/v1/wikilinks/backlinks", protected(http.HandlerFunc(handleBacklinks(store))))
	mux.Handle("DELETE /api/v1/pages", protected(http.HandlerFunc(handleDeletePage(store))))
	if graphSvc != nil {
		mux.Handle("POST /api/v1/pages/reindex", protected(http.HandlerFunc(handleReindexMarkdown(graphSvc))))
		mux.Handle("POST /api/v1/capture", protected(http.HandlerFunc(handleCapture(graphSvc, vaultRoot))))
	}
	if assets != nil {
		mux.Handle("POST /api/v1/assets", protected(http.HandlerFunc(handleAssetUpload(assets))))
	}
	mux.Handle("GET /api/v1/sys/stats", protected(http.HandlerFunc(handleSysStats(store))))
	mux.Handle("GET /api/v1/graph/wiki", protected(http.HandlerFunc(handleWikiGraph(store, vaultRoot))))
}

type reindexBody struct {
	SourcePath string `json:"sourcePath"`
	Markdown   string `json:"markdown"`
}

func handleReindexMarkdown(g *graph.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body reindexBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(body.SourcePath)
		if path == "" {
			http.Error(w, `{"error":"sourcePath required"}`, http.StatusBadRequest)
			return
		}
		if err := g.ReindexMarkdownBytes(r.Context(), path, []byte(body.Markdown)); err != nil {
			http.Error(w, `{"error":`+strconv.Quote(err.Error())+`}`, http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleSearchFTS(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body searchBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		limit := body.Limit
		if limit <= 0 {
			limit = 50
		}
		hits, err := store.SearchBlocksFTS(r.Context(), body.Query, limit)
		if err != nil {
			http.Error(w, `{"error":`+strconv.Quote(err.Error())+`}`, http.StatusBadRequest)
			return
		}
		writeJSON(w, hits)
	}
}

func handleSysStats(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := store.IndexStats(r.Context())
		if err != nil {
			http.Error(w, `{"error":"stats unavailable"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, st)
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

type tokenRequest struct {
	UserID string `json:"userId"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func handleAuthToken(j *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req tokenRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		uid := strings.TrimSpace(req.UserID)
		if uid == "" {
			http.Error(w, `{"error":"userId required"}`, http.StatusBadRequest)
			return
		}
		tok, err := j.MintAccessToken(uid)
		if err != nil {
			http.Error(w, `{"error":"token issue failed"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResponse{AccessToken: tok, TokenType: "Bearer"})
	}
}

func handleGetBlock(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
			return
		}
		b, err := store.GetBlockByID(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		writeJSON(w, b)
	}
}

func handleListPageBlocks(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(r.URL.Query().Get("sourcePath"))
		if path == "" {
			http.Error(w, `{"error":"sourcePath query required"}`, http.StatusBadRequest)
			return
		}
		blocks, err := store.ListDomainBlocksBySourcePath(r.Context(), path)
		if err != nil {
			http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, blocks)
	}
}

type searchBody struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func handleSearch(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body searchBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		limit := body.Limit
		if limit <= 0 {
			limit = 50
		}
		hits, err := store.SearchBlocksFTSWithAliases(r.Context(), body.Query, limit)
		if err != nil {
			http.Error(w, `{"error":`+strconv.Quote(err.Error())+`}`, http.StatusBadRequest)
			return
		}
		writeJSON(w, hits)
	}
}

type propBody struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func handleQueryProperty(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body propBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		blocks, err := store.QueryBlocksByProperty(r.Context(), body.Key, body.Value)
		if err != nil {
			http.Error(w, `{"error":`+strconv.Quote(err.Error())+`}`, http.StatusBadRequest)
			return
		}
		writeJSON(w, blocks)
	}
}

type ftsIDsBody struct {
	Match string `json:"match"`
	Limit int    `json:"limit"`
}

func handleFTSIDs(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body ftsIDsBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		ids, err := store.BlockIDsFromFTS(r.Context(), body.Match, body.Limit)
		if err != nil {
			http.Error(w, `{"error":`+strconv.Quote(err.Error())+`}`, http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ids": ids})
	}
}

type idsBody struct {
	IDs []string `json:"ids"`
}

func handleBlocksByIDs(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body idsBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		blocks, err := store.GetBlocksByIDs(r.Context(), body.IDs)
		if err != nil {
			http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, blocks)
	}
}

func handleRecentPaths(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 3000
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		paths, err := store.ListSourcePathsByRecency(r.Context(), limit)
		if err != nil {
			http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
			return
		}
		if paths == nil {
			paths = []string{}
		}
		writeJSON(w, map[string]any{"paths": paths})
	}
}

func handlePagePropsList(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body propBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		paths, err := store.ListSourcePathsByPageProperty(r.Context(), body.Key, body.Value)
		if err != nil {
			http.Error(w, `{"error":`+strconv.Quote(err.Error())+`}`, http.StatusBadRequest)
			return
		}
		if paths == nil {
			paths = []string{}
		}
		writeJSON(w, map[string]any{"paths": paths})
	}
}

type aliasBody struct {
	NotesRoot string `json:"notesRoot"`
	Target    string `json:"target"`
}

func handleAliasResolve(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body aliasBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		abs, ok, err := store.ResolveAliasToPath(r.Context(), body.NotesRoot, body.Target)
		if err != nil {
			http.Error(w, `{"error":`+strconv.Quote(err.Error())+`}`, http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"path": abs, "ok": ok})
	}
}

type backlinksBody struct {
	Targets []string `json:"targets"`
}

func handleBacklinks(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body backlinksBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		blocks, err := store.BlocksWithWikilinksToTargets(r.Context(), body.Targets)
		if err != nil {
			http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, blocks)
	}
}

func handleDeletePage(store storage.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(r.URL.Query().Get("sourcePath"))
		if path == "" {
			http.Error(w, `{"error":"sourcePath query required"}`, http.StatusBadRequest)
			return
		}
		if err := store.DeleteIndexedSource(r.Context(), path); err != nil {
			http.Error(w, `{"error":"delete failed"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}
