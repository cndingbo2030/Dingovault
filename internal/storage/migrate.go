package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// MigrateMultiTenant upgrades legacy schemas to include user_id tenant columns and indexes.
func MigrateMultiTenant(ctx context.Context, db *sql.DB) error {
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
