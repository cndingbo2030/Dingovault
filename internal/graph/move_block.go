package graph

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// MoveBlockUnder moves movingID's whole Markdown subtree to become the last child
// of newParentID. The Markdown file remains the source of truth; parentage is
// recomputed by ReindexFile after the physical line move.
func (s *Service) MoveBlockUnder(ctx context.Context, movingID, newParentID string) error {
	if movingID == newParentID {
		return fmt.Errorf("cannot move a block under itself")
	}
	moving, err := s.store.GetBlockByID(ctx, movingID)
	if err != nil {
		return fmt.Errorf("lookup moving block: %w", err)
	}
	parent, err := s.store.GetBlockByID(ctx, newParentID)
	if err != nil {
		return fmt.Errorf("lookup parent block: %w", err)
	}
	if moving.Metadata.SourcePath != parent.Metadata.SourcePath {
		return fmt.Errorf("blocks must be in the same file")
	}
	if err := s.ensureNotMovingUnderDescendant(ctx, movingID, newParentID, moving.Metadata.SourcePath); err != nil {
		return err
	}

	path := moving.Metadata.SourcePath
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	lines, eol, trailingNL, err := splitFileLines(data)
	if err != nil {
		return err
	}

	moveIdx := collectSubtreeLineIndices(lines, moving.Metadata.LineStart, moving.Metadata.LineEnd)
	parentIdx := collectSubtreeLineIndices(lines, parent.Metadata.LineStart, parent.Metadata.LineEnd)
	if len(moveIdx) == 0 || len(parentIdx) == 0 {
		return fmt.Errorf("could not resolve block line ranges")
	}
	startMove, endMove := moveIdx[0], moveIdx[len(moveIdx)-1]
	startParent, endParent := parentIdx[0], parentIdx[len(parentIdx)-1]
	if startMove <= startParent && endMove >= startParent {
		return fmt.Errorf("cannot move a block under its own subtree")
	}
	if !isListMarkerLine(lines[startParent]) {
		return fmt.Errorf("new parent must be a list item")
	}

	chunk := make([]string, endMove-startMove+1)
	copy(chunk, lines[startMove:endMove+1])
	targetLead := leadingWhitespacePrefixLen(lines[startParent]) + indentStepSpaces
	currentLead := leadingWhitespacePrefixLen(chunk[0])
	shifted, err := shiftLineChunkIndent(chunk, targetLead-currentLead)
	if err != nil {
		return err
	}

	without := make([]string, 0, len(lines)-len(chunk))
	without = append(without, lines[:startMove]...)
	without = append(without, lines[endMove+1:]...)

	insertAfter := endParent
	if startMove < endParent {
		insertAfter -= len(chunk)
	}
	insertAt := insertAfter + 1
	if insertAt < 0 || insertAt > len(without) {
		return fmt.Errorf("computed insertion point out of range")
	}

	outLines := make([]string, 0, len(without)+len(shifted))
	outLines = append(outLines, without[:insertAt]...)
	outLines = append(outLines, shifted...)
	outLines = append(outLines, without[insertAt:]...)

	out := joinFileLines(outLines, eol, trailingNL)
	if err := AtomicWriteFile(path, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	return s.ReindexFile(ctx, path)
}

func (s *Service) ensureNotMovingUnderDescendant(ctx context.Context, movingID, newParentID, sourcePath string) error {
	blocks, err := s.store.ListDomainBlocksBySourcePath(ctx, sourcePath)
	if err != nil {
		return fmt.Errorf("list blocks: %w", err)
	}
	parentByID := make(map[string]string, len(blocks))
	for _, b := range blocks {
		parentByID[b.ID] = b.ParentID
	}
	for id := newParentID; id != ""; id = parentByID[id] {
		if id == movingID {
			return fmt.Errorf("cannot move a block under its own descendant")
		}
	}
	return nil
}

func shiftLineChunkIndent(lines []string, delta int) ([]string, error) {
	out := append([]string(nil), lines...)
	if delta == 0 {
		return out, nil
	}
	return applyIndentShift(out, nonBlankLineIndices(out), delta)
}

func nonBlankLineIndices(lines []string) []int {
	out := make([]int, 0, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, i)
	}
	return out
}
