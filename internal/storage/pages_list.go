package storage

import (
	"context"
	"fmt"
)

// ListSourcePathsByRecency returns distinct indexed file paths, most recently updated first.
func (s *Store) ListSourcePathsByRecency(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 2000
	}
	if limit > 10000 {
		limit = 10000
	}
	const q = `
SELECT source_path, MAX(updated_at) AS mx
FROM blocks
WHERE user_id = ?
GROUP BY source_path
ORDER BY mx DESC
LIMIT ?`
	uid := storeUserID(ctx)
	rows, err := s.db.QueryContext(ctx, q, uid, limit)
	if err != nil {
		return nil, fmt.Errorf("list source paths: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		var mx int64
		if err := rows.Scan(&p, &mx); err != nil {
			return nil, fmt.Errorf("scan path: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
