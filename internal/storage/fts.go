package storage

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// BlockSearchHit is one row from an FTS5 search joined back to blocks.
type BlockSearchHit struct {
	ID           string  `json:"id"`
	SourcePath   string  `json:"sourcePath"`
	Content      string  `json:"content"`
	LineStart    int     `json:"lineStart"`
	LineEnd      int     `json:"lineEnd"`
	OutlineLevel int     `json:"outlineLevel"`
	Snippet      string  `json:"snippet"`
	Rank         float64 `json:"rank"`
}

var ftsTokenRE = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}_./-]*`)

// SearchBlocksFTS runs a full-text search on blocks_fts and returns ranked hits.
// The query string is tokenized into prefix terms (AND); special FTS5 syntax from the user is not passed through raw.
func (s *Store) SearchBlocksFTS(ctx context.Context, query string, limit int) ([]BlockSearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty search query")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	match, err := buildFTS5MatchQuery(query)
	if err != nil {
		return nil, err
	}

	const sqlStmt = `
SELECT
	b.id,
	b.source_path,
	b.content,
	b.line_start,
	b.line_end,
	b.outline_level,
	snippet(blocks_fts, 0, '«', '»', '…', 24) AS snippet,
	bm25(blocks_fts) AS rank
FROM blocks_fts
JOIN blocks b ON b.rowid = blocks_fts.rowid
WHERE b.user_id = ? AND blocks_fts MATCH ?
ORDER BY rank DESC
LIMIT ?`

	uid := storeUserID(ctx)
	rows, err := s.db.QueryContext(ctx, sqlStmt, uid, match, limit)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer rows.Close()

	var out []BlockSearchHit
	for rows.Next() {
		var h BlockSearchHit
		if err := rows.Scan(
			&h.ID,
			&h.SourcePath,
			&h.Content,
			&h.LineStart,
			&h.LineEnd,
			&h.OutlineLevel,
			&h.Snippet,
			&h.Rank,
		); err != nil {
			return nil, fmt.Errorf("scan fts row: %w", err)
		}
		h.Content = s.revealContent(h.Content)
		h.ID = logicalBlockID(ctx, h.ID)
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fts: %w", err)
	}
	return out, nil
}

// SearchQueryTokens returns alphanumeric tokens from a user search string (same rules as FTS AND tokens).
func SearchQueryTokens(query string) []string {
	return ftsTokenRE.FindAllString(strings.TrimSpace(query), -1)
}

// SearchBlocksFTSWithAliases runs FTS and appends hits for pages whose YAML alias matches all tokens.
func (s *Store) SearchBlocksFTSWithAliases(ctx context.Context, query string, limit int) ([]BlockSearchHit, error) {
	hits, err := s.SearchBlocksFTS(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	toks := SearchQueryTokens(query)
	if len(toks) == 0 {
		return hits, nil
	}

	seen := make(map[string]struct{}, len(hits))
	for _, h := range hits {
		seen[h.SourcePath] = struct{}{}
	}

	const qAliases = `SELECT alias_normalized, source_path FROM page_aliases WHERE user_id = ?`
	rows, err := s.db.QueryContext(ctx, qAliases, storeUserID(ctx))
	if err != nil {
		return hits, fmt.Errorf("list aliases: %w", err)
	}
	defer rows.Close()

	type pair struct {
		alias string
		path  string
	}
	var pairs []pair
	for rows.Next() {
		var a, p string
		if err := rows.Scan(&a, &p); err != nil {
			return hits, fmt.Errorf("scan alias: %w", err)
		}
		pairs = append(pairs, pair{alias: strings.ToLower(a), path: p})
	}
	if err := rows.Err(); err != nil {
		return hits, err
	}

	for _, pr := range pairs {
		ok := true
		for _, t := range toks {
			if !strings.Contains(pr.alias, strings.ToLower(t)) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if _, dup := seen[pr.path]; dup {
			continue
		}
		seen[pr.path] = struct{}{}
		b, err := s.firstBlockRowBySourcePath(ctx, pr.path, storeUserID(ctx))
		if err != nil {
			continue
		}
		snippet := "«alias: " + pr.alias + "»"
		hits = append(hits, BlockSearchHit{
			ID:           logicalBlockID(ctx, b.ID),
			SourcePath:   b.SourcePath,
			Content:      b.Content,
			LineStart:    b.LineStart,
			LineEnd:      b.LineEnd,
			OutlineLevel: b.OutlineLevel,
			Snippet:      snippet,
			Rank:         -1,
		})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

func (s *Store) firstBlockRowBySourcePath(ctx context.Context, sourcePath, userID string) (*BlockRow, error) {
	const q = `
SELECT id, parent_id, content, source_path, line_start, line_end, outline_level
FROM blocks
WHERE source_path = ? AND user_id = ?
ORDER BY line_start ASC, id ASC
LIMIT 1`
	var r BlockRow
	err := s.db.QueryRowContext(ctx, q, sourcePath, userID).Scan(
		&r.ID, &r.ParentID, &r.Content, &r.SourcePath, &r.LineStart, &r.LineEnd, &r.OutlineLevel,
	)
	if err != nil {
		return nil, err
	}
	r.Content = s.revealContent(r.Content)
	return &r, nil
}

// FTSMatchFromUserQuery builds a safe FTS5 MATCH string from free text (prefix terms ANDed).
func FTSMatchFromUserQuery(user string) (string, error) {
	return buildFTS5MatchQuery(user)
}

func buildFTS5MatchQuery(user string) (string, error) {
	toks := ftsTokenRE.FindAllString(strings.TrimSpace(user), -1)
	if len(toks) == 0 {
		return "", fmt.Errorf("no searchable terms in query")
	}
	var b strings.Builder
	for i, t := range toks {
		if i > 0 {
			b.WriteString(" AND ")
		}
		// Prefix query per token: token* (tokens are already restricted by ftsTokenRE).
		b.WriteString(t)
		b.WriteByte('*')
	}
	return b.String(), nil
}

// BlockRow is a flat block row for tree assembly (used by bridge / graph).
type BlockRow struct {
	ID           string
	ParentID     sql.NullString
	Content      string
	SourcePath   string
	LineStart    int
	LineEnd      int
	OutlineLevel int
}

// ListBlocksBySourcePath returns all blocks for a file ordered for tree building (parents before children).
func (s *Store) ListBlocksBySourcePath(ctx context.Context, sourcePath string) ([]BlockRow, error) {
	const q = `
SELECT id, parent_id, content, source_path, line_start, line_end, outline_level
FROM blocks
WHERE source_path = ? AND user_id = ?
ORDER BY line_start ASC, id ASC`

	uid := storeUserID(ctx)
	rows, err := s.db.QueryContext(ctx, q, sourcePath, uid)
	if err != nil {
		return nil, fmt.Errorf("list blocks: %w", err)
	}
	defer rows.Close()

	var out []BlockRow
	for rows.Next() {
		var r BlockRow
		if err := rows.Scan(&r.ID, &r.ParentID, &r.Content, &r.SourcePath, &r.LineStart, &r.LineEnd, &r.OutlineLevel); err != nil {
			return nil, fmt.Errorf("scan block: %w", err)
		}
		r.Content = s.revealContent(r.Content)
		out = append(out, r)
	}
	return out, rows.Err()
}
