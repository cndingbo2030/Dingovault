package storage

import (
	"context"
	"fmt"
	"strings"
)

// ListSourcePathsMissingEmbeddings returns distinct source_path values that have at least
// one non-empty block without a stored embedding row for the given model (local SQLite only).
func (s *Store) ListSourcePathsMissingEmbeddings(ctx context.Context, userID, embeddingModel string, limit int) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	userID = strings.TrimSpace(userID)
	model := strings.TrimSpace(embeddingModel)
	if userID == "" || model == "" || limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT b.source_path
FROM blocks b
WHERE b.user_id = ?
  AND trim(b.content) != ''
  AND NOT EXISTS (
    SELECT 1 FROM block_vectors v
    WHERE v.block_id = b.id AND v.user_id = b.user_id AND v.model = ?
  )
ORDER BY b.source_path
LIMIT ?
`, userID, model, limit)
	if err != nil {
		return nil, fmt.Errorf("list missing embeddings: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
