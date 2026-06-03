package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// WikiGraphNode is one page (absolute vault path) in the link graph.
type WikiGraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// WikiGraphEdge is a directed link from one indexed page to another (resolved wikilink target).
type WikiGraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// WikiGraph is page-level nodes and edges derived from block_wikilinks + alias resolution.
type WikiGraph struct {
	Nodes []WikiGraphNode `json:"nodes"`
	Edges []WikiGraphEdge `json:"edges"`
}

// WikiGraph returns distinct pages as nodes and resolved wikilinks as edges (tenant-scoped).
func (s *Store) WikiGraph(ctx context.Context, vaultRoot string) (WikiGraph, error) {
	vaultRoot = strings.TrimSpace(vaultRoot)
	if vaultRoot == "" {
		return WikiGraph{}, fmt.Errorf("vault root required")
	}
	uid := storeUserID(ctx)
	pageSet, baseNameSet, err := s.wikiGraphPageSets(ctx, uid)
	if err != nil {
		return WikiGraph{}, err
	}
	const q = `
SELECT DISTINCT b.source_path, w.target
FROM block_wikilinks w
INNER JOIN blocks b ON b.id = w.source_block_id
WHERE b.user_id = ?`

	rows, err := s.db.QueryContext(ctx, q, uid)
	if err != nil {
		return WikiGraph{}, fmt.Errorf("wiki graph query: %w", err)
	}
	defer rows.Close()

	nodeSet := make(map[string]struct{})
	var edges []WikiGraphEdge
	edgeSeen := make(map[string]struct{})
	var links []WikiGraphEdge

	for rows.Next() {
		var fromPath, target string
		if err := rows.Scan(&fromPath, &target); err != nil {
			return WikiGraph{}, fmt.Errorf("scan: %w", err)
		}
		fromPath = strings.TrimSpace(fromPath)
		target = strings.TrimSpace(target)
		if fromPath == "" || target == "" {
			continue
		}
		links = append(links, WikiGraphEdge{Source: fromPath, Target: target})
	}
	if err := rows.Err(); err != nil {
		return WikiGraph{}, err
	}
	if err := rows.Close(); err != nil {
		return WikiGraph{}, err
	}

	for _, link := range links {
		fromPath := link.Source
		target := link.Target
		toPath, ok, err := s.ResolveAliasToPath(ctx, vaultRoot, target)
		if err != nil || !ok {
			toPath, ok = resolveWikiGraphTarget(vaultRoot, target, pageSet, baseNameSet)
			if !ok {
				continue
			}
		}
		if fromPath == toPath {
			continue
		}
		nodeSet[fromPath] = struct{}{}
		nodeSet[toPath] = struct{}{}
		key := fromPath + "\x00" + toPath
		if _, dup := edgeSeen[key]; dup {
			continue
		}
		edgeSeen[key] = struct{}{}
		edges = append(edges, WikiGraphEdge{Source: fromPath, Target: toPath})
	}

	nodes := make([]WikiGraphNode, 0, len(nodeSet))
	for p := range nodeSet {
		nodes = append(nodes, WikiGraphNode{
			ID:    p,
			Label: wikiGraphLabel(p),
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return WikiGraph{Nodes: nodes, Edges: edges}, nil
}

func (s *Store) wikiGraphPageSets(ctx context.Context, uid string) (map[string]struct{}, map[string]string, error) {
	const q = `SELECT DISTINCT source_path FROM blocks WHERE user_id = ?`
	rows, err := s.db.QueryContext(ctx, q, uid)
	if err != nil {
		return nil, nil, fmt.Errorf("wiki graph pages: %w", err)
	}
	defer rows.Close()

	pageSet := make(map[string]struct{})
	baseCounts := make(map[string]int)
	baseValues := make(map[string]string)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, nil, err
		}
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pageSet[p] = struct{}{}
		key := strings.ToLower(strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)))
		if key != "" {
			baseCounts[key]++
			baseValues[key] = p
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	baseNameSet := make(map[string]string)
	for key, count := range baseCounts {
		if count == 1 {
			baseNameSet[key] = baseValues[key]
		}
	}
	return pageSet, baseNameSet, nil
}

func resolveWikiGraphTarget(vaultRoot, target string, pageSet map[string]struct{}, baseNameSet map[string]string) (string, bool) {
	t := strings.TrimSpace(target)
	if t == "" {
		return "", false
	}
	t = strings.ReplaceAll(t, "\\", "/")
	if strings.Contains(t, "|") {
		t = strings.TrimSpace(strings.SplitN(t, "|", 2)[0])
	}
	candidates := []string{t}
	if !strings.EqualFold(filepath.Ext(t), ".md") {
		candidates = append(candidates, t+".md")
	}
	for _, c := range candidates {
		if abs, ok := resolveWikiGraphVaultPath(vaultRoot, c); ok {
			if _, exists := pageSet[abs]; exists {
				return abs, true
			}
		}
	}
	baseKey := strings.ToLower(strings.TrimSuffix(filepath.Base(t), filepath.Ext(t)))
	if p, ok := baseNameSet[baseKey]; ok {
		return p, true
	}
	return "", false
}

func resolveWikiGraphVaultPath(vaultRoot, rel string) (string, bool) {
	root, err := filepath.Abs(filepath.Clean(vaultRoot))
	if err != nil {
		return "", false
	}
	if strings.TrimSpace(rel) == "" {
		return "", false
	}
	var p string
	if filepath.IsAbs(rel) {
		p = filepath.Clean(rel)
	} else {
		p = filepath.Join(root, filepath.FromSlash(rel))
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", false
	}
	rootWithSep := root + string(filepath.Separator)
	if abs != root && !strings.HasPrefix(abs+string(filepath.Separator), rootWithSep) {
		return "", false
	}
	return abs, true
}

func wikiGraphLabel(abs string) string {
	base := filepath.Base(abs)
	if ext := filepath.Ext(base); strings.EqualFold(ext, ".md") {
		return strings.TrimSuffix(base, ext)
	}
	return base
}
