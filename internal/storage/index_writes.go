package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dingbo/dingovault/internal/parser"
)

// ReplaceIndexedSource replaces blocks, wikilinks, tags, page_properties, and page_aliases
// for absSourcePath inside a single locked transaction.
func (s *Store) ReplaceIndexedSource(ctx context.Context, absSourcePath string, res parser.ParseResult, pageProps map[string]string, aliases []string) error {
	res = scopeParseResult(ctx, res)
	uid := storeUserID(ctx)

	return s.WithWriteLock(func(db *sql.DB) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		if _, err := tx.ExecContext(ctx, `DELETE FROM page_properties WHERE user_id = ? AND source_path = ?`, uid, absSourcePath); err != nil {
			return fmt.Errorf("delete page properties: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM page_aliases WHERE user_id = ? AND source_path = ?`, uid, absSourcePath); err != nil {
			return fmt.Errorf("delete page aliases: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM blocks WHERE user_id = ? AND source_path = ?`, uid, absSourcePath); err != nil {
			return fmt.Errorf("delete old blocks: %w", err)
		}

		const insBlock = `
INSERT INTO blocks (
	id, user_id, parent_id, content, properties_json, source_path,
	line_start, line_end, outline_level, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

		for _, b := range res.Blocks {
			props := "{}"
			if len(b.Properties) > 0 {
				raw, mErr := json.Marshal(b.Properties)
				if mErr != nil {
					return fmt.Errorf("marshal properties: %w", mErr)
				}
				props = string(raw)
			}
			parent := sql.NullString{String: b.ParentID, Valid: b.ParentID != ""}
			if _, err := tx.ExecContext(ctx, insBlock,
				b.ID,
				uid,
				parent,
				b.Content,
				props,
				absSourcePath,
				b.Metadata.LineStart,
				b.Metadata.LineEnd,
				b.Metadata.Level,
				b.Metadata.CreatedAt.Unix(),
				b.Metadata.UpdatedAt.Unix(),
			); err != nil {
				return fmt.Errorf("insert block %s: %w", b.ID, err)
			}
		}

		const insWiki = `INSERT INTO block_wikilinks (source_block_id, target, display_alias) VALUES (?, ?, ?)`
		for _, w := range res.Wikilinks {
			if _, err := tx.ExecContext(ctx, insWiki, w.SourceBlockID, w.Target, w.Alias); err != nil {
				return fmt.Errorf("insert wikilink: %w", err)
			}
		}

		const insTag = `INSERT OR IGNORE INTO block_tags (block_id, tag) VALUES (?, ?)`
		for _, t := range res.Tags {
			if _, err := tx.ExecContext(ctx, insTag, t.BlockID, t.Tag); err != nil {
				return fmt.Errorf("insert tag: %w", err)
			}
		}

		const insProp = `INSERT INTO page_properties (user_id, source_path, prop_key, prop_value) VALUES (?, ?, ?, ?)`
		for k, v := range pageProps {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, insProp, uid, absSourcePath, k, v); err != nil {
				return fmt.Errorf("insert page property: %w", err)
			}
		}

		const insAlias = `INSERT OR REPLACE INTO page_aliases (user_id, alias_normalized, source_path) VALUES (?, ?, ?)`
		for _, a := range aliases {
			k := parser.NormalizeAliasKey(a)
			if k == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, insAlias, uid, k, absSourcePath); err != nil {
				return fmt.Errorf("insert alias: %w", err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		return nil
	})
}

// DeleteIndexedSource removes blocks (and cascading edges), page_properties, and page_aliases for a path.
func (s *Store) DeleteIndexedSource(ctx context.Context, absSourcePath string) error {
	uid := storeUserID(ctx)
	return s.WithWriteLock(func(db *sql.DB) error {
		if _, err := db.ExecContext(ctx, `DELETE FROM page_properties WHERE user_id = ? AND source_path = ?`, uid, absSourcePath); err != nil {
			return fmt.Errorf("delete page properties: %w", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM page_aliases WHERE user_id = ? AND source_path = ?`, uid, absSourcePath); err != nil {
			return fmt.Errorf("delete page aliases: %w", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM blocks WHERE user_id = ? AND source_path = ?`, uid, absSourcePath); err != nil {
			return fmt.Errorf("delete blocks: %w", err)
		}
		return nil
	})
}
