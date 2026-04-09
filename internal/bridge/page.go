package bridge

import (
	"sort"

	"github.com/cndingbo2030/dingovault/internal/domain"
)

// PageBlock is a block with nested children for the outliner UI.
type PageBlock struct {
	domain.Block
	Children []PageBlock `json:"children"`
}

func buildPageTree(blocks []domain.Block) []PageBlock {
	byParent := make(map[string][]domain.Block)
	for _, b := range blocks {
		p := b.ParentID
		byParent[p] = append(byParent[p], b)
	}
	var roots []domain.Block
	for _, b := range blocks {
		if b.Root() {
			roots = append(roots, b)
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		if roots[i].Metadata.LineStart != roots[j].Metadata.LineStart {
			return roots[i].Metadata.LineStart < roots[j].Metadata.LineStart
		}
		return roots[i].ID < roots[j].ID
	})
	out := make([]PageBlock, 0, len(roots))
	for _, r := range roots {
		out = append(out, buildPageNode(r, byParent))
	}
	return out
}

func buildPageNode(b domain.Block, byParent map[string][]domain.Block) PageBlock {
	kids := byParent[b.ID]
	sort.Slice(kids, func(i, j int) bool {
		if kids[i].Metadata.LineStart != kids[j].Metadata.LineStart {
			return kids[i].Metadata.LineStart < kids[j].Metadata.LineStart
		}
		return kids[i].ID < kids[j].ID
	})
	ch := make([]PageBlock, 0, len(kids))
	for _, k := range kids {
		ch = append(ch, buildPageNode(k, byParent))
	}
	return PageBlock{Block: b, Children: ch}
}
