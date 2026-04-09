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
		toPath, ok, err := s.ResolveAliasToPath(ctx, vaultRoot, target)
		if err != nil || !ok {
			continue
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
	if err := rows.Err(); err != nil {
		return WikiGraph{}, err
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

func wikiGraphLabel(abs string) string {
	base := filepath.Base(abs)
	if ext := filepath.Ext(base); strings.EqualFold(ext, ".md") {
		return strings.TrimSuffix(base, ext)
	}
	return base
}
