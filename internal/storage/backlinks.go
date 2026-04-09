package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dingbo/dingovault/internal/domain"
)

// BlocksWithWikilinksToTargets returns distinct source blocks that link to any of the given targets
// (case-insensitive match on block_wikilinks.target). Uses idx_block_wikilinks_target_lower when present.
func (s *Store) BlocksWithWikilinksToTargets(ctx context.Context, targets []string) ([]domain.Block, error) {
	seen := make(map[string]struct{})
	var lowered []string
	for _, t := range targets {
		lt := strings.ToLower(strings.TrimSpace(t))
		if lt == "" {
			continue
		}
		if _, ok := seen[lt]; ok {
			continue
		}
		seen[lt] = struct{}{}
		lowered = append(lowered, lt)
	}
	if len(lowered) == 0 {
		return nil, nil
	}

	placeholders := strings.Repeat("?,", len(lowered)-1) + "?"
	q := fmt.Sprintf(`
SELECT DISTINCT b.id, b.parent_id, b.content, b.properties_json, b.source_path,
	b.line_start, b.line_end, b.outline_level, b.created_at, b.updated_at
FROM block_wikilinks AS l
JOIN blocks AS b ON b.id = l.source_block_id
WHERE b.user_id = ? AND lower(l.target) IN (%s)
ORDER BY b.source_path ASC, b.line_start ASC, b.id ASC`, placeholders)

	uid := storeUserID(ctx)
	args := make([]interface{}, 0, len(lowered)+1)
	args = append(args, uid)
	for _, v := range lowered {
		args = append(args, v)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("backlinks query: %w", err)
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

func (s *Store) scanBlockRows(rows *sql.Rows) ([]domain.Block, error) {
	var out []domain.Block
	for rows.Next() {
		var b domain.Block
		var parent sql.NullString
		var propsJSON string
		var created, updated int64
		if err := rows.Scan(
			&b.ID,
			&parent,
			&b.Content,
			&propsJSON,
			&b.Metadata.SourcePath,
			&b.Metadata.LineStart,
			&b.Metadata.LineEnd,
			&b.Metadata.Level,
			&created,
			&updated,
		); err != nil {
			return nil, fmt.Errorf("scan block: %w", err)
		}
		if parent.Valid {
			b.ParentID = parent.String
		}
		if propsJSON != "" && propsJSON != "{}" {
			if err := json.Unmarshal([]byte(propsJSON), &b.Properties); err != nil {
				return nil, fmt.Errorf("properties: %w", err)
			}
		}
		if b.Properties == nil {
			b.Properties = map[string]string{}
		}
		b.Metadata.CreatedAt = time.Unix(created, 0).UTC()
		b.Metadata.UpdatedAt = time.Unix(updated, 0).UTC()
		b.Content = s.revealContent(b.Content)
		out = append(out, b)
	}
	return out, rows.Err()
}
