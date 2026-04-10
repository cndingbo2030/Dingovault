package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// CurrentSchemaVersion is the PRAGMA user_version Dingovault expects after all migrations run.
// Increment when adding a new migration step in RunSchemaMigrations.
const CurrentSchemaVersion = 3

// ReadUserVersion returns SQLite PRAGMA user_version.
func ReadUserVersion(ctx context.Context, db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
		return 0, fmt.Errorf("read user_version: %w", err)
	}
	return v, nil
}

// WriteUserVersion sets PRAGMA user_version (must match migration progress).
func WriteUserVersion(ctx context.Context, db *sql.DB, v int) error {
	if v < 0 {
		return fmt.Errorf("invalid user_version %d", v)
	}
	// user_version is an integer pragma; value is not user-controlled SQL.
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
		return fmt.Errorf("write user_version: %w", err)
	}
	return nil
}

// RunSchemaMigrations applies incremental migrations from PRAGMA user_version to CurrentSchemaVersion.
// Safe on fresh DBs (version 0) and on legacy DBs created before versioning.
func RunSchemaMigrations(ctx context.Context, db *sql.DB) error {
	v, err := ReadUserVersion(ctx, db)
	if err != nil {
		return err
	}
	if v > CurrentSchemaVersion {
		return fmt.Errorf("database schema newer than this binary (user_version=%d, app supports up to %d) — upgrade Dingovault", v, CurrentSchemaVersion)
	}
	for v < CurrentSchemaVersion {
		switch v {
		case 0:
			if err := migrateV0ToV1(ctx, db); err != nil {
				return fmt.Errorf("migrate 0→1: %w", err)
			}
		case 1:
			if err := migrateV1ToV2(ctx, db); err != nil {
				return fmt.Errorf("migrate 1→2: %w", err)
			}
		case 2:
			if err := migrateV2ToV3(ctx, db); err != nil {
				return fmt.Errorf("migrate 2→3: %w", err)
			}
		default:
			return fmt.Errorf("internal error: unhandled schema step from version %d", v)
		}
		v++
		if err := WriteUserVersion(ctx, db, v); err != nil {
			return err
		}
	}
	return nil
}

// migrateV0ToV1 adds multi-tenant columns/tables for legacy databases (idempotent on modern DDL).
func migrateV0ToV1(ctx context.Context, db *sql.DB) error {
	if err := migrateBlocksUserID(ctx, db); err != nil {
		return err
	}
	if err := migratePagePropertiesUserID(ctx, db); err != nil {
		return err
	}
	if err := migratePageAliasesUserID(ctx, db); err != nil {
		return err
	}
	return ensureIndexes(ctx, db)
}

// migrateV1ToV2 reserved for forward-compatible metadata (plugins, feature flags, etc.).
func migrateV1ToV2(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS dingovault_meta (
	k TEXT PRIMARY KEY NOT NULL,
	v TEXT NOT NULL
)`)
	if err != nil {
		return fmt.Errorf("dingovault_meta: %w", err)
	}
	return nil
}

// migrateV2ToV3 adds block_vectors for embedding storage (RAG groundwork; BLOB float32[]).
func migrateV2ToV3(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS block_vectors (
	user_id TEXT NOT NULL DEFAULT 'local',
	block_id TEXT NOT NULL,
	model TEXT NOT NULL,
	dim INTEGER NOT NULL,
	embedding BLOB NOT NULL,
	updated_at INTEGER NOT NULL,
	PRIMARY KEY (user_id, block_id, model),
	FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
)`)
	if err != nil {
		return fmt.Errorf("block_vectors: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_block_vectors_user ON block_vectors(user_id)`); err != nil {
		return fmt.Errorf("idx_block_vectors_user: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_block_vectors_block ON block_vectors(block_id)`); err != nil {
		return fmt.Errorf("idx_block_vectors_block: %w", err)
	}
	return nil
}

// MigrateMultiTenant upgrades legacy schemas to include user_id tenant columns and indexes.
// Deprecated: use RunSchemaMigrations; kept as a thin wrapper for tests and external callers.
func MigrateMultiTenant(ctx context.Context, db *sql.DB) error {
	return migrateV0ToV1(ctx, db)
}

func migrateBlocksUserID(ctx context.Context, db *sql.DB) error {
	ok, err := columnExists(ctx, db, "blocks", "user_id")
	if err != nil || ok {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE blocks ADD COLUMN user_id TEXT NOT NULL DEFAULT 'local'`); err != nil {
		return fmt.Errorf("migrate blocks.user_id: %w", err)
	}
	return nil
}

func migratePagePropertiesUserID(ctx context.Context, db *sql.DB) error {
	ok, err := columnExists(ctx, db, "page_properties", "user_id")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `ALTER TABLE page_properties RENAME TO page_properties_legacy`); err != nil {
		return fmt.Errorf("rename page_properties: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
CREATE TABLE page_properties (
	user_id TEXT NOT NULL DEFAULT 'local',
	source_path TEXT NOT NULL,
	prop_key TEXT NOT NULL,
	prop_value TEXT NOT NULL,
	PRIMARY KEY (user_id, source_path, prop_key)
)`); err != nil {
		return fmt.Errorf("create page_properties: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO page_properties (user_id, source_path, prop_key, prop_value)
SELECT 'local', source_path, prop_key, prop_value FROM page_properties_legacy`); err != nil {
		return fmt.Errorf("copy page_properties: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE page_properties_legacy`); err != nil {
		return fmt.Errorf("drop page_properties_legacy: %w", err)
	}
	return tx.Commit()
}

func migratePageAliasesUserID(ctx context.Context, db *sql.DB) error {
	ok, err := columnExists(ctx, db, "page_aliases", "user_id")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `ALTER TABLE page_aliases RENAME TO page_aliases_legacy`); err != nil {
		return fmt.Errorf("rename page_aliases: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
CREATE TABLE page_aliases (
	user_id TEXT NOT NULL DEFAULT 'local',
	alias_normalized TEXT NOT NULL,
	source_path TEXT NOT NULL,
	PRIMARY KEY (user_id, alias_normalized)
)`); err != nil {
		return fmt.Errorf("create page_aliases: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO page_aliases (user_id, alias_normalized, source_path)
SELECT 'local', alias_normalized, source_path FROM page_aliases_legacy`); err != nil {
		return fmt.Errorf("copy page_aliases: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE page_aliases_legacy`); err != nil {
		return fmt.Errorf("drop page_aliases_legacy: %w", err)
	}
	return tx.Commit()
}

func ensureIndexes(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_blocks_user_id ON blocks(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_blocks_user_source ON blocks(user_id, source_path)`,
		`CREATE INDEX IF NOT EXISTS idx_page_props_user_key ON page_properties(user_id, prop_key)`,
		`CREATE INDEX IF NOT EXISTS idx_page_aliases_user ON page_aliases(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_page_aliases_path ON page_aliases(source_path)`,
	}
	for _, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("index: %w", err)
		}
	}
	return nil
}

func columnExists(ctx context.Context, db *sql.DB, table, col string) (bool, error) {
	// table is internal constant only — not user input.
	q := fmt.Sprintf(`PRAGMA table_info(%q)`, table)
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}
