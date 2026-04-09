package graph

import (
	"context"
	"fmt"
	"os"
)

// ReorderSiblingBefore moves movingID's subtree to sit immediately before beforeID in the file.
// Both blocks must share the same source file and parent_id.
func (s *Service) ReorderSiblingBefore(ctx context.Context, movingID, beforeID string) error {
	if movingID == beforeID {
		return nil
	}
	a, err := s.store.GetBlockByID(ctx, movingID)
	if err != nil {
		return fmt.Errorf("lookup moving block: %w", err)
	}
	b, err := s.store.GetBlockByID(ctx, beforeID)
	if err != nil {
		return fmt.Errorf("lookup anchor block: %w", err)
	}
	if a.Metadata.SourcePath != b.Metadata.SourcePath {
		return fmt.Errorf("blocks must be in the same file")
	}
	if a.ParentID != b.ParentID {
		return fmt.Errorf("blocks must share the same parent (siblings only)")
	}

	path := a.Metadata.SourcePath
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	lines, eol, trailingNL, err := splitFileLines(data)
	if err != nil {
		return err
	}

	lsA, leA := a.Metadata.LineStart, a.Metadata.LineEnd
	lsB, leB := b.Metadata.LineStart, b.Metadata.LineEnd
	idxA := collectSubtreeLineIndices(lines, lsA, leA)
	idxB := collectSubtreeLineIndices(lines, lsB, leB)
	if len(idxA) == 0 || len(idxB) == 0 {
		return fmt.Errorf("could not resolve block line ranges")
	}
	startA, endA := idxA[0], idxA[len(idxA)-1]
	startB, _ := idxB[0], idxB[len(idxB)-1]

	if startA <= startB && endA >= startB {
		return fmt.Errorf("moving range overlaps anchor")
	}

	chunk := make([]string, endA-startA+1)
	copy(chunk, lines[startA:endA+1])

	without := make([]string, 0, len(lines)-(endA-startA+1))
	without = append(without, lines[:startA]...)
	without = append(without, lines[endA+1:]...)

	// After removal, line indices below the removed chunk shift up by len(chunk).
	newStartB := startB
	if startB > endA {
		newStartB = startB - (endA - startA + 1)
	}

	outLines := make([]string, 0, len(without)+len(chunk))
	outLines = append(outLines, without[:newStartB]...)
	outLines = append(outLines, chunk...)
	outLines = append(outLines, without[newStartB:]...)

	out := joinFileLines(outLines, eol, trailingNL)
	if err := AtomicWriteFile(path, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	return s.ReindexFile(ctx, path)
}
