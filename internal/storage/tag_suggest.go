package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/ai"
)

const minTagSuggestScore = float32(0.38)

// SuggestTagsByEmbedding returns tag names whose tagged blocks are semantically close to the query vector.
func (s *Store) SuggestTagsByEmbedding(ctx context.Context, query []float32, embeddingModel string, topN int) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if len(query) == 0 || topN <= 0 {
		return nil, nil
	}
	model := strings.TrimSpace(embeddingModel)
	if model == "" {
		return nil, nil
	}
	qdim := len(query)
	uid := storeUserID(ctx)
	const q = `
SELECT t.tag, v.embedding
FROM block_vectors v
INNER JOIN block_tags t ON t.block_id = v.block_id
WHERE v.user_id = ? AND v.model = ? AND v.dim = ?
LIMIT 800`

	rows, err := s.db.QueryContext(ctx, q, uid, model, qdim)
	if err != nil {
		return nil, fmt.Errorf("tag suggest query: %w", err)
	}
	defer rows.Close()

	best, err := scanTagSuggestBestScores(rows, query, qdim)
	if err != nil {
		return nil, err
	}
	return tagNamesSortedTopN(best, topN), nil
}

func scanTagSuggestBestScores(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}, query []float32, qdim int) (map[string]float32, error) {
	best := make(map[string]float32)
	for rows.Next() {
		var tag string
		var blob []byte
		if err := rows.Scan(&tag, &blob); err != nil {
			return nil, fmt.Errorf("tag suggest scan: %w", err)
		}
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		vec := blobToFloat32Vec(blob)
		if len(vec) != qdim {
			continue
		}
		sc := ai.CosineSimilarity(query, vec)
		if sc < minTagSuggestScore {
			continue
		}
		if prev, ok := best[tag]; !ok || sc > prev {
			best[tag] = sc
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return best, nil
}

func tagNamesSortedTopN(best map[string]float32, topN int) []string {
	if len(best) == 0 {
		return nil
	}
	type pair struct {
		tag string
		sc  float32
	}
	pairs := make([]pair, 0, len(best))
	for t, sc := range best {
		pairs = append(pairs, pair{tag: t, sc: sc})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].sc == pairs[j].sc {
			return pairs[i].tag < pairs[j].tag
		}
		return pairs[i].sc > pairs[j].sc
	})
	if len(pairs) > topN {
		pairs = pairs[:topN]
	}
	out := make([]string, len(pairs))
	for i := range pairs {
		out[i] = pairs[i].tag
	}
	return out
}
