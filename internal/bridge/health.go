package bridge

import (
	"context"
	"fmt"

	"github.com/cndingbo2030/dingovault/internal/locale"
	"github.com/cndingbo2030/dingovault/internal/storage"
)

// SetHealthRescan registers a callback that re-walks the vault after the SQLite index
// is cleared (desktop / mobile). Optional; required for HealthResetLocalSearchIndex.
func (a *App) SetHealthRescan(fn func(context.Context) error) {
	a.healthRescan = fn
}

// HealthResetLocalSearchIndex wipes the local SQLite search index (blocks, FTS,
// embeddings metadata, page props/aliases) and re-indexes from Markdown on disk.
// Markdown files in the vault are not deleted.
func (a *App) HealthResetLocalSearchIndex() error {
	if a.healthRescan == nil {
		return fmt.Errorf("%s", a.t(locale.ErrHealthResetUnavailable))
	}
	st, ok := a.store.(*storage.Store)
	if !ok {
		return fmt.Errorf("%s", a.t(locale.ErrHealthResetCloud))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	if err := st.WipeIndexedContent(ctx); err != nil {
		return err
	}
	a.invalidatePageCache()
	return a.healthRescan(ctx)
}
