package storage

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/dingbo/dingovault/internal/domain"
)

var safePropKey = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// QueryBlocksByProperty returns blocks whose properties_json has key=value (case-insensitive on value).
func (s *Store) QueryBlocksByProperty(ctx context.Context, key, value string) ([]domain.Block, error) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return nil, fmt.Errorf("empty key or value")
	}
	if !safePropKey.MatchString(key) {
		return nil, fmt.Errorf("property key must be alphanumeric, underscore, or hyphen")
	}

	jsonPath := "$." + key
	q := `
SELECT id, parent_id, content, properties_json, source_path,
	line_start, line_end, outline_level, created_at, updated_at
FROM blocks
WHERE user_id = ?
  AND json_type(properties_json, ?) IS NOT NULL
  AND lower(json_extract(properties_json, ?)) = lower(?)
ORDER BY source_path ASC, line_start ASC`

	uid := storeUserID(ctx)
	rows, err := s.db.QueryContext(ctx, q, uid, jsonPath, jsonPath, value)
	if err != nil {
		return nil, fmt.Errorf("property query: %w", err)
	}
	defer rows.Close()
	blocks, err := s.scanBlockRows(rows)
	if err != nil {
		return nil, err
	}
	for i := range blocks {
		blocks[i] = decodeBlock(ctx, blocks[i])
	}
	return blocks, nil
}

// BlockIDsFromFTS returns block ids matching an FTS query (for QueryBlocks free-text fallback).
func (s *Store) BlockIDsFromFTS(ctx context.Context, ftsMatch string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	q := `
SELECT b.id FROM blocks_fts AS f
JOIN blocks AS b ON b.rowid = f.rowid
WHERE b.user_id = ? AND f MATCH ?
LIMIT ?`
	uid := storeUserID(ctx)
	rows, err := s.db.QueryContext(ctx, q, uid, ftsMatch, limit)
	if err != nil {
		return nil, fmt.Errorf("fts id query: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, logicalBlockID(ctx, id))
	}
	return ids, rows.Err()
}
