package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cndingbo2030/dingovault/internal/domain"
	"github.com/cndingbo2030/dingovault/internal/parser"
)

// RemoteStore implements Provider by calling a Dingovault SaaS HTTP API (/api/v1).
// Every request sends Authorization: Bearer <token>. Writes that need file content
// read the markdown from disk via ReadFile (defaults to os.ReadFile).
type RemoteStore struct {
	baseURL string
	token   string
	client  *http.Client
	// ReadFile loads raw markdown for ReplaceIndexedSource (same path as local graph).
	ReadFile func(name string) ([]byte, error)
}

// NewRemoteStore creates a client for baseAPIURL (e.g. https://vault.example.com or http://127.0.0.1:12030).
func NewRemoteStore(baseAPIURL, bearerToken string) (*RemoteStore, error) {
	base := strings.TrimRight(strings.TrimSpace(baseAPIURL), "/")
	if base == "" {
		return nil, fmt.Errorf("empty API base URL")
	}
	if strings.TrimSpace(bearerToken) == "" {
		return nil, fmt.Errorf("empty bearer token")
	}
	return &RemoteStore{
		baseURL:  base,
		token:    strings.TrimSpace(bearerToken),
		client:   &http.Client{Timeout: 60 * time.Second},
		ReadFile: os.ReadFile,
	}, nil
}

// Close is a no-op (HTTP client has no persistent handle to release).
func (r *RemoteStore) Close() error {
	return nil
}

func (r *RemoteStore) doJSON(ctx context.Context, method, path string, reqBody any, respBody any) (int, error) {
	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return 0, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, bodyReader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer closeBody(resp.Body)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("remote %s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(raw)))
	}
	if respBody == nil || len(raw) == 0 {
		return resp.StatusCode, nil
	}
	if err := json.Unmarshal(raw, respBody); err != nil {
		return resp.StatusCode, fmt.Errorf("decode response: %w", err)
	}
	return resp.StatusCode, nil
}

func (r *RemoteStore) GetBlockByID(ctx context.Context, id string) (domain.Block, error) {
	path := "/api/v1/blocks/" + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+path, nil)
	if err != nil {
		return domain.Block{}, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	resp, err := r.client.Do(req)
	if err != nil {
		return domain.Block{}, err
	}
	defer closeBody(resp.Body)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode == http.StatusNotFound {
		return domain.Block{}, fmt.Errorf("block not found: %s", id)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.Block{}, fmt.Errorf("remote GET block: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var b domain.Block
	if err := json.Unmarshal(raw, &b); err != nil {
		return domain.Block{}, err
	}
	return b, nil
}

func (r *RemoteStore) ListDomainBlocksBySourcePath(ctx context.Context, sourcePath string) ([]domain.Block, error) {
	u := r.baseURL + "/api/v1/pages?sourcePath=" + url.QueryEscape(sourcePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer closeBody(resp.Body)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote GET pages: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var blocks []domain.Block
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}
	if blocks == nil {
		blocks = []domain.Block{}
	}
	return blocks, nil
}

func (r *RemoteStore) GetBlocksByIDs(ctx context.Context, ids []string) ([]domain.Block, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []domain.Block
	_, err := r.doJSON(ctx, http.MethodPost, "/api/v1/blocks/by-ids", map[string]any{"ids": ids}, &out)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.Block{}
	}
	return out, nil
}

type pathsWrap struct {
	Paths []string `json:"paths"`
}

func (r *RemoteStore) ListSourcePathsByRecency(ctx context.Context, limit int) ([]string, error) {
	u := fmt.Sprintf("%s/api/v1/paths/recent?limit=%d", r.baseURL, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer closeBody(resp.Body)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote paths/recent: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var w pathsWrap
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, err
	}
	if w.Paths == nil {
		w.Paths = []string{}
	}
	return w.Paths, nil
}

func (r *RemoteStore) QueryBlocksByProperty(ctx context.Context, key, value string) ([]domain.Block, error) {
	var out []domain.Block
	_, err := r.doJSON(ctx, http.MethodPost, "/api/v1/query/property", map[string]string{"key": key, "value": value}, &out)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.Block{}
	}
	return out, nil
}

type ftsIDsResp struct {
	IDs []string `json:"ids"`
}

func (r *RemoteStore) BlockIDsFromFTS(ctx context.Context, ftsMatch string, limit int) ([]string, error) {
	var w ftsIDsResp
	_, err := r.doJSON(ctx, http.MethodPost, "/api/v1/query/fts-ids", map[string]any{"match": ftsMatch, "limit": limit}, &w)
	if err != nil {
		return nil, err
	}
	if w.IDs == nil {
		w.IDs = []string{}
	}
	return w.IDs, nil
}

func (r *RemoteStore) BlocksWithWikilinksToTargets(ctx context.Context, targets []string) ([]domain.Block, error) {
	var out []domain.Block
	_, err := r.doJSON(ctx, http.MethodPost, "/api/v1/wikilinks/backlinks", map[string]any{"targets": targets}, &out)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.Block{}
	}
	return out, nil
}

func (r *RemoteStore) SearchBlocksFTS(ctx context.Context, query string, limit int) ([]BlockSearchHit, error) {
	var out []BlockSearchHit
	_, err := r.doJSON(ctx, http.MethodPost, "/api/v1/search/fts", map[string]any{"query": query, "limit": limit}, &out)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []BlockSearchHit{}
	}
	return out, nil
}

func (r *RemoteStore) SearchBlocksFTSWithAliases(ctx context.Context, query string, limit int) ([]BlockSearchHit, error) {
	var out []BlockSearchHit
	_, err := r.doJSON(ctx, http.MethodPost, "/api/v1/search", map[string]any{"query": query, "limit": limit}, &out)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []BlockSearchHit{}
	}
	return out, nil
}

type aliasResolveResp struct {
	Path string `json:"path"`
	OK   bool   `json:"ok"`
}

func (r *RemoteStore) ResolveAliasToPath(ctx context.Context, notesRoot, target string) (string, bool, error) {
	var ar aliasResolveResp
	_, err := r.doJSON(ctx, http.MethodPost, "/api/v1/alias/resolve", map[string]string{
		"notesRoot": notesRoot,
		"target":    target,
	}, &ar)
	if err != nil {
		return "", false, err
	}
	return ar.Path, ar.OK, nil
}

func (r *RemoteStore) ListSourcePathsByPageProperty(ctx context.Context, key, value string) ([]string, error) {
	var w pathsWrap
	_, err := r.doJSON(ctx, http.MethodPost, "/api/v1/page-properties/list", map[string]string{"key": key, "value": value}, &w)
	if err != nil {
		return nil, err
	}
	if w.Paths == nil {
		w.Paths = []string{}
	}
	return w.Paths, nil
}

func (r *RemoteStore) ReplaceIndexedSource(ctx context.Context, absSourcePath string, _ parser.ParseResult, _ map[string]string, _ []string) error {
	read := r.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	raw, err := read(absSourcePath)
	if err != nil {
		return fmt.Errorf("read markdown for remote reindex: %w", err)
	}
	_, err = r.doJSON(ctx, http.MethodPost, "/api/v1/pages/reindex", map[string]string{
		"sourcePath": absSourcePath,
		"markdown":   string(raw),
	}, nil)
	return err
}

func (r *RemoteStore) WikiGraph(ctx context.Context, vaultRoot string) (WikiGraph, error) {
	_ = vaultRoot
	var g WikiGraph
	_, err := r.doJSON(ctx, http.MethodGet, "/api/v1/graph/wiki", nil, &g)
	if err != nil {
		return WikiGraph{}, err
	}
	if g.Nodes == nil {
		g.Nodes = []WikiGraphNode{}
	}
	if g.Edges == nil {
		g.Edges = []WikiGraphEdge{}
	}
	return g, nil
}

func (r *RemoteStore) IndexStats(ctx context.Context) (IndexStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/sys/stats", nil)
	if err != nil {
		return IndexStats{}, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	resp, err := r.client.Do(req)
	if err != nil {
		return IndexStats{}, err
	}
	defer closeBody(resp.Body)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return IndexStats{}, fmt.Errorf("remote sys/stats: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var st IndexStats
	if err := json.Unmarshal(raw, &st); err != nil {
		return IndexStats{}, err
	}
	return st, nil
}

// UpsertBlockEmbedding is a no-op for the remote provider (vectors stay local-only for now).
func (r *RemoteStore) UpsertBlockEmbedding(ctx context.Context, userID, blockID, model string, vec []float32) error {
	_ = ctx
	_ = userID
	_ = blockID
	_ = model
	_ = vec
	return nil
}

// SearchSemantic is unsupported for the remote provider.
func (r *RemoteStore) SearchSemantic(ctx context.Context, queryVector []float32, embeddingModel string, topK int) ([]SemanticSearchHit, error) {
	_ = r
	_ = ctx
	_ = queryVector
	_ = embeddingModel
	_ = topK
	return nil, nil
}

// SemanticPageEdges is unsupported for the remote provider.
func (r *RemoteStore) SemanticPageEdges(ctx context.Context, embeddingModel string, minCosine float32, maxEdges int) ([]WikiGraphSemanticEdge, error) {
	_ = r
	_ = ctx
	_ = embeddingModel
	_ = minCosine
	_ = maxEdges
	return nil, nil
}

// SuggestTagsByEmbedding is unsupported for the remote provider.
func (r *RemoteStore) SuggestTagsByEmbedding(ctx context.Context, query []float32, embeddingModel string, topN int) ([]string, error) {
	_ = r
	_ = ctx
	_ = query
	_ = embeddingModel
	_ = topN
	return nil, nil
}

func (r *RemoteStore) DeleteIndexedSource(ctx context.Context, absSourcePath string) error {
	u := r.baseURL + "/api/v1/pages?sourcePath=" + url.QueryEscape(absSourcePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer closeBody(resp.Body)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("remote DELETE pages: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

func closeBody(c io.Closer) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		log.Printf("remote response close: %v", err)
	}
}
