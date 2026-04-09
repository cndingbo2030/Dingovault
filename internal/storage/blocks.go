package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dingbo/dingovault/internal/domain"
)

// GetBlockByID loads a single block row as domain.Block.
func (s *Store) GetBlockByID(ctx context.Context, id string) (domain.Block, error) {
	const q = `
SELECT id, parent_id, content, properties_json, source_path,
	line_start, line_end, outline_level, created_at, updated_at
FROM blocks WHERE id = ? AND user_id = ?`

	var b domain.Block
	var parent sql.NullString
	var propsJSON string
	var created, updated int64

	uid := storeUserID(ctx)
	pid := physicalBlockID(ctx, id)
	err := s.db.QueryRowContext(ctx, q, pid, uid).Scan(
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
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Block{}, fmt.Errorf("block not found: %s", id)
		}
		return domain.Block{}, fmt.Errorf("query block: %w", err)
	}
	if parent.Valid {
		b.ParentID = parent.String
	}
	if propsJSON != "" && propsJSON != "{}" {
		if err := json.Unmarshal([]byte(propsJSON), &b.Properties); err != nil {
			return domain.Block{}, fmt.Errorf("properties: %w", err)
		}
	}
	if b.Properties == nil {
		b.Properties = map[string]string{}
	}
	b.Metadata.CreatedAt = time.Unix(created, 0).UTC()
	b.Metadata.UpdatedAt = time.Unix(updated, 0).UTC()
	return decodeBlock(ctx, b), nil
}

// ListDomainBlocksBySourcePath returns all blocks for a page as domain values (ordered by line).
func (s *Store) ListDomainBlocksBySourcePath(ctx context.Context, sourcePath string) ([]domain.Block, error) {
	const q = `
SELECT id, parent_id, content, properties_json, source_path,
	line_start, line_end, outline_level, created_at, updated_at
FROM blocks WHERE source_path = ? AND user_id = ?
ORDER BY line_start ASC, id ASC`

	uid := storeUserID(ctx)
	rows, err := s.db.QueryContext(ctx, q, sourcePath, uid)
	if err != nil {
		return nil, fmt.Errorf("list blocks: %w", err)
	}
	defer rows.Close()

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
			return nil, fmt.Errorf("scan: %w", err)
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
		out = append(out, decodeBlock(ctx, b))
	}
	return out, rows.Err()
}
