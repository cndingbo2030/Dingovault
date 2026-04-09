package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dingbo/dingovault/internal/tenant"
)

func TestStore_IndexStats_Empty(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "t.db")
	s, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := tenant.WithUserID(context.Background(), tenant.LocalUserID)
	st, err := s.IndexStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.BlockCount != 0 || st.PageCount != 0 || st.TenantCount != 0 {
		t.Fatalf("%+v", st)
	}
}
