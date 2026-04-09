package parser

import (
	"fmt"

	"github.com/google/uuid"
)

// StableBlockID returns a deterministic UUID (v5-style SHA-1) for a block span in a file.
// IDs remain stable across re-parses as long as the same source path and line span represent
// the same block; after edits, line ranges change and IDs update accordingly.
func StableBlockID(sourcePath string, lineStart, lineEnd int) string {
	name := fmt.Sprintf("%s:%d:%d", sourcePath, lineStart, lineEnd)
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(name)).String()
}
