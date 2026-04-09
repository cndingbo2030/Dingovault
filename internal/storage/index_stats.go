package storage

import (
	"context"
	"fmt"
)

// IndexStats aggregates global index metrics for SaaS monitoring (all tenants).
type IndexStats struct {
	BlockCount  int `json:"blockCount"`
	PageCount   int `json:"pageCount"`
	TenantCount int `json:"tenantCount"`
}

// IndexStats returns counts across the whole database (not scoped by request tenant).
func (s *Store) IndexStats(ctx context.Context) (IndexStats, error) {
	const q = `
SELECT
	(SELECT COUNT(*) FROM blocks) AS blocks,
	(SELECT COUNT(DISTINCT source_path) FROM blocks) AS pages,
	(SELECT COUNT(DISTINCT user_id) FROM blocks) AS tenants`
	var st IndexStats
	err := s.db.QueryRowContext(ctx, q).Scan(&st.BlockCount, &st.PageCount, &st.TenantCount)
	if err != nil {
		return IndexStats{}, fmt.Errorf("index stats: %w", err)
	}
	return st, nil
}
