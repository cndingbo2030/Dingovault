package storage

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/ai"
)

// SemanticSearchHit is one block retrieved by vector similarity (cosine vs query embedding).
type SemanticSearchHit struct {
	BlockID    string  `json:"blockId"`
	SourcePath string  `json:"sourcePath"`
	Content    string  `json:"content"`
	Score      float32 `json:"score"`
}

// WikiGraphSemanticEdge links two pages by embedding similarity (undirected visual edge).
type WikiGraphSemanticEdge struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Score  float32 `json:"score"`
}

func blobToFloat32Vec(blob []byte) []float32 {
	if len(blob) < 4 || len(blob)%4 != 0 {
		return nil
	}
	n := len(blob) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return out
}

// SearchSemantic returns the topK blocks (same tenant + embedding model + dimension as query) by cosine similarity.
func (s *Store) SearchSemantic(ctx context.Context, queryVector []float32, embeddingModel string, topK int) ([]SemanticSearchHit, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if len(queryVector) == 0 || topK <= 0 {
		return nil, nil
	}
	model := strings.TrimSpace(embeddingModel)
	if model == "" {
		return nil, fmt.Errorf("embedding model required")
	}
	qdim := len(queryVector)
	uid := storeUserID(ctx)
	const q = `
SELECT v.block_id, v.embedding, b.content, b.source_path
FROM block_vectors v
INNER JOIN blocks b ON b.id = v.block_id AND b.user_id = v.user_id
WHERE v.user_id = ? AND v.model = ? AND v.dim = ?`

	rows, err := s.db.QueryContext(ctx, q, uid, model, qdim)
	if err != nil {
		return nil, fmt.Errorf("semantic query: %w", err)
	}
	defer rows.Close()

	buf, err := scanSemanticSearchRows(ctx, rows, queryVector, qdim)
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, nil
	}
	sort.Slice(buf, func(i, j int) bool {
		if buf[i].score == buf[j].score {
			return buf[i].hit.SourcePath < buf[j].hit.SourcePath
		}
		return buf[i].score > buf[j].score
	})
	if len(buf) > topK {
		buf = buf[:topK]
	}
	out := make([]SemanticSearchHit, len(buf))
	for i := range buf {
		out[i] = buf[i].hit
	}
	return out, nil
}

const semanticGraphMaxBlocks = 900

// SemanticPageEdges builds page–page edges from block embedding similarity (max across block pairs, different pages).
func (s *Store) SemanticPageEdges(ctx context.Context, embeddingModel string, minCosine float32, maxEdges int) ([]WikiGraphSemanticEdge, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	model := strings.TrimSpace(embeddingModel)
	if model == "" {
		return nil, nil
	}
	if maxEdges <= 0 {
		maxEdges = 64
	}
	uid := storeUserID(ctx)
	data, err := s.loadSemanticGraphRows(ctx, uid, model)
	if err != nil {
		return nil, err
	}
	if len(data) < 2 {
		return nil, nil
	}
	best := accumulateSemanticPagePairScores(data, minCosine)
	return semanticEdgesFromBest(best, maxEdges), nil
}

type semanticScoredHit struct {
	score float32
	hit   SemanticSearchHit
}

func scanSemanticSearchRows(ctx context.Context, rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}, queryVector []float32, qdim int) ([]semanticScoredHit, error) {
	var buf []semanticScoredHit
	for rows.Next() {
		var blockID string
		var blob []byte
		var content, sourcePath string
		if err := rows.Scan(&blockID, &blob, &content, &sourcePath); err != nil {
			return nil, fmt.Errorf("semantic scan: %w", err)
		}
		vec := blobToFloat32Vec(blob)
		if len(vec) != qdim {
			continue
		}
		sim := ai.CosineSimilarity(queryVector, vec)
		buf = append(buf, semanticScoredHit{
			score: sim,
			hit: SemanticSearchHit{
				BlockID:    logicalBlockID(ctx, blockID),
				SourcePath: strings.TrimSpace(sourcePath),
				Content:    content,
				Score:      sim,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return buf, nil
}
