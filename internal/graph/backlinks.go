package graph

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/domain"
)

// WikilinkLookupKeys returns normalized lookup strings that might appear in block_wikilinks.target
// for links pointing to absPagePath inside notesRoot.
func WikilinkLookupKeys(absPagePath, notesRoot string) ([]string, error) {
	root, err := filepath.Abs(filepath.Clean(notesRoot))
	if err != nil {
		return nil, fmt.Errorf("notes root: %w", err)
	}
	abs, err := filepath.Abs(filepath.Clean(absPagePath))
	if err != nil {
		return nil, fmt.Errorf("page path: %w", err)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return nil, fmt.Errorf("rel path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	base := filepath.Base(rel)
	baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))
	if strings.EqualFold(filepath.Ext(base), ".md") {
		// already markdown file name
	} else {
		baseNoExt = strings.TrimSuffix(base, ".md")
	}

	relNoMd := strings.TrimSuffix(rel, ".md")
	relNoMd = strings.TrimSuffix(relNoMd, ".MD")

	set := make(map[string]struct{})
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		set[s] = struct{}{}
	}

	add(baseNoExt)
	add(base)
	add(relNoMd)
	add(rel)
	add(strings.TrimSuffix(base, ".md"))
	// Common Logseq-style nested title without extension
	if i := strings.LastIndex(relNoMd, "/"); i >= 0 {
		add(relNoMd[i+1:])
	}

	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out, nil
}

// GetBacklinks returns distinct blocks (any page) that contain a wikilink whose target resolves to pagePath.
func (s *Service) GetBacklinks(ctx context.Context, notesRoot, pagePath string) ([]domain.Block, error) {
	abs, err := ResolveVaultPath(notesRoot, pagePath)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(abs), ".md") {
		abs += ".md"
	}
	keys, err := WikilinkLookupKeys(abs, notesRoot)
	if err != nil {
		return nil, err
	}
	return s.store.BlocksWithWikilinksToTargets(ctx, keys)
}
