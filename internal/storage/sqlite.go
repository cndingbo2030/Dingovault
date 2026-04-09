package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SchemaDDL is applied after pragmas are set. Blocks use a self-referential parent_id
// for arbitrary-depth trees; recursive queries use SQLite WITH RECURSIVE.
const SchemaDDL = `
CREATE TABLE IF NOT EXISTS blocks (
	id TEXT PRIMARY KEY NOT NULL,
	user_id TEXT NOT NULL DEFAULT 'local',
	parent_id TEXT,
	content TEXT NOT NULL DEFAULT '',
	properties_json TEXT NOT NULL DEFAULT '{}',
	source_path TEXT NOT NULL,
	line_start INTEGER NOT NULL DEFAULT 1,
	line_end INTEGER NOT NULL DEFAULT 1,
	outline_level INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	FOREIGN KEY (parent_id) REFERENCES blocks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_blocks_parent_id ON blocks(parent_id);
CREATE INDEX IF NOT EXISTS idx_blocks_user_id ON blocks(user_id);
CREATE INDEX IF NOT EXISTS idx_blocks_user_source ON blocks(user_id, source_path);
CREATE INDEX IF NOT EXISTS idx_blocks_source_path ON blocks(source_path);
CREATE INDEX IF NOT EXISTS idx_blocks_source_line ON blocks(source_path, line_start);
CREATE INDEX IF NOT EXISTS idx_blocks_updated_at ON blocks(updated_at);

-- FTS5 mirrors block text; rowid aligns with blocks.rowid for external content sync.
CREATE VIRTUAL TABLE IF NOT EXISTS blocks_fts USING fts5(
	content,
	content='blocks',
	content_rowid='rowid'
);

-- Keep FTS in sync when blocks change (blocks must remain a rowid table; TEXT PK is not rowid alias).
CREATE TRIGGER IF NOT EXISTS blocks_ai AFTER INSERT ON blocks BEGIN
	INSERT INTO blocks_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS blocks_ad AFTER DELETE ON blocks BEGIN
	INSERT INTO blocks_fts(blocks_fts, rowid, content) VALUES('delete', old.rowid, old.content);
END;

CREATE TRIGGER IF NOT EXISTS blocks_au AFTER UPDATE ON blocks BEGIN
	INSERT INTO blocks_fts(blocks_fts, rowid, content) VALUES('delete', old.rowid, old.content);
	INSERT INTO blocks_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TABLE IF NOT EXISTS block_wikilinks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_block_id TEXT NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
	target TEXT NOT NULL,
	display_alias TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_block_wikilinks_target ON block_wikilinks(target);
CREATE INDEX IF NOT EXISTS idx_block_wikilinks_source ON block_wikilinks(source_block_id);
CREATE INDEX IF NOT EXISTS idx_block_wikilinks_target_lower ON block_wikilinks(lower(target));

CREATE TABLE IF NOT EXISTS block_tags (
	block_id TEXT NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
	tag TEXT NOT NULL,
	PRIMARY KEY (block_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_block_tags_tag ON block_tags(tag);

CREATE TABLE IF NOT EXISTS page_properties (
	user_id TEXT NOT NULL DEFAULT 'local',
	source_path TEXT NOT NULL,
	prop_key TEXT NOT NULL,
	prop_value TEXT NOT NULL,
	PRIMARY KEY (user_id, source_path, prop_key)
);

CREATE INDEX IF NOT EXISTS idx_page_props_key ON page_properties(prop_key);
CREATE INDEX IF NOT EXISTS idx_page_props_user_key ON page_properties(user_id, prop_key);
CREATE INDEX IF NOT EXISTS idx_page_props_key_lower ON page_properties(lower(prop_key));
CREATE INDEX IF NOT EXISTS idx_page_props_value_lower ON page_properties(lower(prop_value));

-- One target per (tenant, alias) — last successful reindex wins on conflict.
CREATE TABLE IF NOT EXISTS page_aliases (
	user_id TEXT NOT NULL DEFAULT 'local',
	alias_normalized TEXT NOT NULL,
	source_path TEXT NOT NULL,
	PRIMARY KEY (user_id, alias_normalized)
);

CREATE INDEX IF NOT EXISTS idx_page_aliases_user ON page_aliases(user_id);
CREATE INDEX IF NOT EXISTS idx_page_aliases_path ON page_aliases(source_path);
`

// Store wraps database access with a write mutex so goroutine-heavy indexing can
// serialize mutations while reads use the pool concurrently.
type Store struct {
	db           *sql.DB
	mu           sync.Mutex
	masterCipher *MasterCipher // optional; DINGO_MASTER_KEY — encrypts block content in DB
}

// OpenSQLite opens (or creates) a SQLite file, applies schema DDL, and returns a Store.
func OpenSQLite(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(time.Hour)

	if err := InitSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}

	st := &Store{db: db}
	if mk := strings.TrimSpace(os.Getenv("DINGO_MASTER_KEY")); mk != "" {
		ciph, err := NewMasterCipher(mk)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("DINGO_MASTER_KEY: %w", err)
		}
		st.masterCipher = ciph
		log.Printf("DINGO_MASTER_KEY set: block content is encrypted at rest in SQLite (FTS body search is ineffective on ciphertext)")
	}
	return st, nil
}

// DB returns the underlying *sql.DB for read-heavy queries (callers must not close it).
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close releases the database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close sqlite: %w", err)
	}
	return nil
}

// WithWriteLock runs fn while holding the store write lock. Use for inserts/updates/deletes.
func (s *Store) WithWriteLock(fn func(*sql.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(s.db)
}

// InitSchema executes pragma and DDL statements. Safe to call on an existing database.
func InitSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL;`); err != nil {
		return fmt.Errorf("pragma journal_mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA synchronous = NORMAL;`); err != nil {
		return fmt.Errorf("pragma synchronous: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000;`); err != nil {
		return fmt.Errorf("pragma busy_timeout: %w", err)
	}

	if _, err := db.ExecContext(ctx, SchemaDDL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	if err := RunSchemaMigrations(ctx, db); err != nil {
		return fmt.Errorf("schema migrations: %w", err)
	}
	return nil
}
