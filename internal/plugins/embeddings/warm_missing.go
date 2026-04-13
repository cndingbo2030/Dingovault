package embeddings

import (
	"context"
	"log"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/bus"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

// ScheduleWarmMissing publishes after:block:indexed for sources that have blocks
// missing embeddings for embeddingModel (debounced by the embeddings plugin).
func ScheduleWarmMissing(ctx context.Context, store storage.Provider, b *bus.Bus, embeddingModel string, limit int) {
	if b == nil || store == nil || strings.TrimSpace(embeddingModel) == "" || limit <= 0 {
		return
	}
	st, ok := store.(*storage.Store)
	if !ok {
		return
	}
	uid := tenant.UserID(ctx)
	paths, err := st.ListSourcePathsMissingEmbeddings(ctx, uid, embeddingModel, limit)
	if err != nil {
		log.Printf("embeddings warm: list missing: %v", err)
		return
	}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		b.Publish(ctx, bus.TopicAfterBlockIndexed, bus.AfterBlockIndexedPayload{SourcePath: p})
	}
	if len(paths) > 0 {
		log.Printf("embeddings warm: queued %d source(s) missing vectors for model %q", len(paths), embeddingModel)
	}
}
