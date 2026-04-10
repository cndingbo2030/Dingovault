package storage

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// UpsertBlockEmbedding stores embedding as little-endian float32 BLOB (dim * 4 bytes).
func (s *Store) UpsertBlockEmbedding(ctx context.Context, userID, blockID, model string, vec []float32) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if len(vec) == 0 {
		return nil
	}
	blob := make([]byte, 4*len(vec))
	for i, f := range vec {
		binary.LittleEndian.PutUint32(blob[i*4:], math.Float32bits(f))
	}
	now := time.Now().Unix()
	return s.WithWriteLock(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `
INSERT INTO block_vectors (user_id, block_id, model, dim, embedding, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, block_id, model) DO UPDATE SET
	dim = excluded.dim,
	embedding = excluded.embedding,
	updated_at = excluded.updated_at
`, userID, blockID, model, len(vec), blob, now)
		if err != nil {
			return fmt.Errorf("upsert block_vectors: %w", err)
		}
		return nil
	})
}
