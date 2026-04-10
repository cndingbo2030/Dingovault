package bridge

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/ai"
	"github.com/cndingbo2030/dingovault/internal/config"
	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/locale"
	"github.com/cndingbo2030/dingovault/internal/storage"
)

// SemanticRelatedDTO is one semantically similar block on another page.
type SemanticRelatedDTO struct {
	BlockID    string  `json:"blockId"`
	SourcePath string  `json:"sourcePath"`
	RelPath    string  `json:"relPath"`
	Preview    string  `json:"preview"`
	Score      float32 `json:"score"`
}

// GetSemanticRelatedForPage finds blocks on other pages similar to the page's Markdown (vector search).
func (a *App) GetSemanticRelatedForPage(pagePath string, limit int) ([]SemanticRelatedDTO, error) {
	if a.store == nil {
		return nil, fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
	}
	if a.notesRoot == "" {
		return nil, fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	if limit <= 0 {
		limit = 12
	}
	if limit > 24 {
		limit = 24
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	abs, err := a.resolveVaultMarkdownAbs(ctx, pagePath)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", a.t(locale.ErrReadPage), err)
	}
	pageSample := truncateRunes(string(raw), 8000)

	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	c.AI = config.NormalizeAISettings(c.AI)
	p, err := ai.NewProvider(c.AI)
	if err != nil {
		return nil, err
	}
	qvec, err := p.Embed(ctx, pageSample)
	if err != nil || len(qvec) == 0 {
		return nil, nil
	}
	hits, err := a.store.SearchSemantic(ctx, qvec, c.AI.EmbeddingsModel, limit*3)
	if err != nil {
		return nil, err
	}
	return semanticRelatedDTOsFromHits(a.notesRoot, abs, hits, limit), nil
}

func semanticRelatedDTOsFromHits(notesRoot, pageAbs string, hits []storage.SemanticSearchHit, limit int) []SemanticRelatedDTO {
	var out []SemanticRelatedDTO
	seen := make(map[string]struct{})
	for _, h := range hits {
		sp := strings.TrimSpace(h.SourcePath)
		if sp == "" || strings.EqualFold(sp, pageAbs) {
			continue
		}
		if _, dup := seen[h.BlockID]; dup {
			continue
		}
		seen[h.BlockID] = struct{}{}
		rel, _ := graph.VaultRelativePath(notesRoot, sp)
		prev := strings.TrimSpace(strings.ReplaceAll(h.Content, "\n", " "))
		if len(prev) > 200 {
			prev = prev[:197] + "…"
		}
		out = append(out, SemanticRelatedDTO{
			BlockID:    h.BlockID,
			SourcePath: sp,
			RelPath:    rel,
			Preview:    prev,
			Score:      h.Score,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// SuggestTagsForBlock suggests existing vault #tags that are semantically similar to the block text.
func (a *App) SuggestTagsForBlock(blockID string) ([]string, error) {
	blockID = strings.TrimSpace(blockID)
	if blockID == "" {
		return nil, nil
	}
	if a.store == nil {
		return nil, fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	b, err := a.store.GetBlockByID(ctx, blockID)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(b.Content)
	if text == "" {
		return nil, nil
	}
	if len(text) > 6000 {
		text = text[:6000]
	}
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	c.AI = config.NormalizeAISettings(c.AI)
	p, err := ai.NewProvider(c.AI)
	if err != nil {
		return nil, err
	}
	vec, err := p.Embed(ctx, text)
	if err != nil || len(vec) == 0 {
		return nil, nil
	}
	return a.store.SuggestTagsByEmbedding(ctx, vec, c.AI.EmbeddingsModel, 5)
}
