package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/domain"
)

// GetBlocksByIDs loads blocks in the same order as ids (skips missing ids).
func (s *Store) GetBlocksByIDs(ctx context.Context, ids []string) ([]domain.Block, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	q := fmt.Sprintf(`
SELECT id, parent_id, content, properties_json, source_path,
	line_start, line_end, outline_level, created_at, updated_at
FROM blocks WHERE user_id = ? AND id IN (%s)`, placeholders)

	uid := storeUserID(ctx)
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, uid)
	for _, id := range ids {
		args = append(args, physicalBlockID(ctx, id))
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("blocks by id: %w", err)
	}
	defer rows.Close()

	blocks, err := s.scanBlockRows(rows)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.Block, len(blocks))
	for _, b := range blocks {
		byID[b.ID] = b
	}
	out := make([]domain.Block, 0, len(ids))
	for _, id := range ids {
		pid := physicalBlockID(ctx, id)
		if b, ok := byID[pid]; ok {
			out = append(out, decodeBlock(ctx, b))
		}
	}
	return out, nil
}
