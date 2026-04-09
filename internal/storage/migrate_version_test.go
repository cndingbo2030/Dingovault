package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRunSchemaMigrations_UserVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "m.db")
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	v, err := ReadUserVersion(context.Background(), db.DB())
	if err != nil {
		t.Fatal(err)
	}
	if v != CurrentSchemaVersion {
		t.Fatalf("user_version got %d want %d", v, CurrentSchemaVersion)
	}
}
