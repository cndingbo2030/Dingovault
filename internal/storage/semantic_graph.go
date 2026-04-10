package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/ai"
)

type semanticGraphRow struct {
	vec  []float32
	path string
}

func (s *Store) loadSemanticGraphRows(ctx context.Context, uid, model string) ([]semanticGraphRow, error) {
	const q = `
SELECT v.embedding, b.source_path
FROM block_vectors v
INNER JOIN blocks b ON b.id = v.block_id AND b.user_id = v.user_id
WHERE v.user_id = ? AND v.model = ?
LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, uid, model, semanticGraphMaxBlocks)
	if err != nil {
		return nil, fmt.Errorf("semantic graph query: %w", err)
	}
	defer rows.Close()
	var data []semanticGraphRow
	for rows.Next() {
		var blob []byte
		var path string
		if err := rows.Scan(&blob, &path); err != nil {
			return nil, fmt.Errorf("semantic graph scan: %w", err)
		}
		vec := blobToFloat32Vec(blob)
		if len(vec) == 0 {
			continue
		}
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		data = append(data, semanticGraphRow{vec: vec, path: path})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func semanticPagePairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

func accumulateSemanticPagePairScores(data []semanticGraphRow, minCosine float32) map[string]float32 {
	best := make(map[string]float32)
	n := len(data)
	for i := 0; i < n; i++ {
		if len(data[i].vec) == 0 {
			continue
		}
		for j := i + 1; j < n; j++ {
			if data[i].path == data[j].path {
				continue
			}
			if len(data[i].vec) != len(data[j].vec) {
				continue
			}
			sim := ai.CosineSimilarity(data[i].vec, data[j].vec)
			if sim < minCosine {
				continue
			}
			k := semanticPagePairKey(data[i].path, data[j].path)
			if prev, ok := best[k]; !ok || sim > prev {
				best[k] = sim
			}
		}
	}
	return best
}

type semanticPageEdge struct {
	a, b  string
	score float32
}

func semanticEdgesFromBest(best map[string]float32, maxEdges int) []WikiGraphSemanticEdge {
	if len(best) == 0 {
		return nil
	}
	edges := make([]semanticPageEdge, 0, len(best))
	for k, sc := range best {
		parts := strings.SplitN(k, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		edges = append(edges, semanticPageEdge{a: parts[0], b: parts[1], score: sc})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].score == edges[j].score {
			return edges[i].a < edges[j].a
		}
		return edges[i].score > edges[j].score
	})
	if len(edges) > maxEdges {
		edges = edges[:maxEdges]
	}
	out := make([]WikiGraphSemanticEdge, len(edges))
	for i := range edges {
		out[i] = WikiGraphSemanticEdge{Source: edges[i].a, Target: edges[i].b, Score: edges[i].score}
	}
	return out
}
