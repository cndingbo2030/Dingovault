package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// WipeIndexedContent removes all search-index rows (blocks, FTS mirrors via triggers,
// page metadata, aliases, embeddings) without touching vault Markdown files on disk.
// Caller should run a full vault re-scan afterwards.
func (s *Store) WipeIndexedContent(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	return s.WithWriteLock(func(db *sql.DB) error {
		if _, err := db.ExecContext(ctx, `DELETE FROM blocks`); err != nil {
			return fmt.Errorf("wipe blocks: %w", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM page_properties`); err != nil {
			return fmt.Errorf("wipe page_properties: %w", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM page_aliases`); err != nil {
			return fmt.Errorf("wipe page_aliases: %w", err)
		}
		return nil
	})
}
