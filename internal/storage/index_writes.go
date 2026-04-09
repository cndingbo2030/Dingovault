package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/parser"
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

		if err := clearIndexedSource(ctx, tx, uid, absSourcePath); err != nil {
			return err
		}
		if err := s.insertBlocks(ctx, tx, uid, absSourcePath, res); err != nil {
			return err
		}
		if err := insertWikiLinks(ctx, tx, res); err != nil {
			return err
		}
		if err := insertTags(ctx, tx, res); err != nil {
			return err
		}
		if err := insertPageProps(ctx, tx, uid, absSourcePath, pageProps); err != nil {
			return err
		}
		if err := insertAliases(ctx, tx, uid, absSourcePath, aliases); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		return nil
	})
}

func clearIndexedSource(ctx context.Context, tx *sql.Tx, uid, absSourcePath string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM page_properties WHERE user_id = ? AND source_path = ?`, uid, absSourcePath); err != nil {
		return fmt.Errorf("delete page properties: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM page_aliases WHERE user_id = ? AND source_path = ?`, uid, absSourcePath); err != nil {
		return fmt.Errorf("delete page aliases: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM blocks WHERE user_id = ? AND source_path = ?`, uid, absSourcePath); err != nil {
		return fmt.Errorf("delete old blocks: %w", err)
	}
	return nil
}

func (s *Store) insertBlocks(ctx context.Context, tx *sql.Tx, uid, absSourcePath string, res parser.ParseResult) error {
	const insBlock = `
INSERT INTO blocks (
	id, user_id, parent_id, content, properties_json, source_path,
	line_start, line_end, outline_level, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	for _, b := range res.Blocks {
		props, err := marshalBlockProps(b.Properties)
		if err != nil {
			return err
		}
		content, err := s.sealContent(b.Content)
		if err != nil {
			return fmt.Errorf("seal content: %w", err)
		}
		parent := sql.NullString{String: b.ParentID, Valid: b.ParentID != ""}
		if _, err := tx.ExecContext(ctx, insBlock,
			b.ID,
			uid,
			parent,
			content,
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
	return nil
}

func marshalBlockProps(props map[string]string) (string, error) {
	if len(props) == 0 {
		return "{}", nil
	}
	raw, err := json.Marshal(props)
	if err != nil {
		return "", fmt.Errorf("marshal properties: %w", err)
	}
	return string(raw), nil
}

func insertWikiLinks(ctx context.Context, tx *sql.Tx, res parser.ParseResult) error {
	const insWiki = `INSERT INTO block_wikilinks (source_block_id, target, display_alias) VALUES (?, ?, ?)`
	for _, w := range res.Wikilinks {
		if _, err := tx.ExecContext(ctx, insWiki, w.SourceBlockID, w.Target, w.Alias); err != nil {
			return fmt.Errorf("insert wikilink: %w", err)
		}
	}
	return nil
}

func insertTags(ctx context.Context, tx *sql.Tx, res parser.ParseResult) error {
	const insTag = `INSERT OR IGNORE INTO block_tags (block_id, tag) VALUES (?, ?)`
	for _, t := range res.Tags {
		if _, err := tx.ExecContext(ctx, insTag, t.BlockID, t.Tag); err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}
	return nil
}

func insertPageProps(ctx context.Context, tx *sql.Tx, uid, absSourcePath string, pageProps map[string]string) error {
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
	return nil
}

func insertAliases(ctx context.Context, tx *sql.Tx, uid, absSourcePath string, aliases []string) error {
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
	return nil
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
