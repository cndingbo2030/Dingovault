package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/dingbo/dingovault/internal/domain"
	"github.com/dingbo/dingovault/internal/storage"
)

// QueryBlocks interprets a small DSL:
//   - "key:value" → filter properties_json (e.g. status:todo)
//   - otherwise → FTS prefix search on block content
func (s *Service) QueryBlocks(ctx context.Context, dsl string) ([]domain.Block, error) {
	dsl = strings.TrimSpace(dsl)
	if dsl == "" {
		return nil, fmt.Errorf("empty query")
	}
	if k, v, ok := parsePropertyDSL(dsl); ok {
		return s.store.QueryBlocksByProperty(ctx, k, v)
	}
	match, err := storage.FTSMatchFromUserQuery(dsl)
	if err != nil {
		return nil, err
	}
	ids, err := s.store.BlockIDsFromFTS(ctx, match, 200)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return s.store.GetBlocksByIDs(ctx, ids)
}

func parsePropertyDSL(s string) (key, val string, ok bool) {
	// Ignore leading "property:" disambiguator
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "prop:") {
		s = strings.TrimSpace(s[5:])
	}
	i := strings.IndexByte(s, ':')
	if i <= 0 || i >= len(s)-1 {
		return "", "", false
	}
	key = strings.TrimSpace(s[:i])
	val = strings.TrimSpace(s[i+1:])
	if key == "" || val == "" {
		return "", "", false
	}
	// Heuristic: first segment looks like a property key (no spaces)
	if strings.ContainsAny(key, " \t\n") {
		return "", "", false
	}
	return key, val, true
}
