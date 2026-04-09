package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/parser"
)

// ResolveAliasToPath returns the absolute source_path for a wikilink-style target when it matches
// a YAML alias (normalized). notesRoot is used to reject rows outside the vault.
func (s *Store) ResolveAliasToPath(ctx context.Context, notesRoot, target string) (abs string, ok bool, err error) {
	key := parser.NormalizeAliasKey(target)
	if key == "" {
		return "", false, nil
	}
	var sp string
	uid := storeUserID(ctx)
	qerr := s.db.QueryRowContext(ctx,
		`SELECT source_path FROM page_aliases WHERE user_id = ? AND alias_normalized = ?`, uid, key,
	).Scan(&sp)
	if errors.Is(qerr, sql.ErrNoRows) {
		return "", false, nil
	}
	if qerr != nil {
		return "", false, fmt.Errorf("alias lookup: %w", qerr)
	}
	root, err := filepath.Abs(filepath.Clean(notesRoot))
	if err != nil {
		return "", false, err
	}
	absSp, err := filepath.Abs(sp)
	if err != nil {
		return "", false, err
	}
	rootWithSep := root + string(filepath.Separator)
	if absSp != root && !strings.HasPrefix(absSp+string(filepath.Separator), rootWithSep) {
		return "", false, nil
	}
	return absSp, true, nil
}

// ListSourcePathsByPageProperty returns indexed pages with a matching YAML property (case-insensitive key/value).
func (s *Store) ListSourcePathsByPageProperty(ctx context.Context, key, value string) ([]string, error) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return nil, fmt.Errorf("empty property key or value")
	}
	const q = `
SELECT source_path FROM page_properties
WHERE user_id = ? AND lower(prop_key) = lower(?) AND lower(prop_value) = lower(?)
ORDER BY source_path`
	uid := storeUserID(ctx)
	rows, err := s.db.QueryContext(ctx, q, uid, key, value)
	if err != nil {
		return nil, fmt.Errorf("page property query: %w", err)
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
