package graph

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// InsertChildBlock appends a new list item as the last child of parentID.
func (s *Service) InsertChildBlock(ctx context.Context, parentID, initialText string) error {
	parent, err := s.store.GetBlockByID(ctx, parentID)
	if err != nil {
		return fmt.Errorf("lookup parent block: %w", err)
	}
	path := parent.Metadata.SourcePath
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	lines, eol, trailingNL, err := splitFileLines(data)
	if err != nil {
		return err
	}
	parentIdx := collectSubtreeLineIndices(lines, parent.Metadata.LineStart, parent.Metadata.LineEnd)
	if len(parentIdx) == 0 {
		return fmt.Errorf("could not resolve parent line range")
	}
	parentLine := lines[parentIdx[0]]
	if !isListMarkerLine(parentLine) {
		return fmt.Errorf("parent must be a list item")
	}

	text := strings.TrimSpace(initialText)
	if text == "" {
		text = " "
	}
	indent := strings.Repeat(" ", leadingWhitespacePrefixLen(parentLine)+indentStepSpaces)
	childLines := childListItemLines(indent, text)
	insertAt := parentIdx[len(parentIdx)-1] + 1

	outLines := make([]string, 0, len(lines)+len(childLines))
	outLines = append(outLines, lines[:insertAt]...)
	outLines = append(outLines, childLines...)
	outLines = append(outLines, lines[insertAt:]...)
	out := joinFileLines(outLines, eol, trailingNL)
	if err := AtomicWriteFile(path, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	return s.ReindexFile(ctx, path)
}

func childListItemLines(indent, text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(normalized, "\n")
	if len(parts) == 0 {
		parts = []string{" "}
	}
	if strings.TrimSpace(parts[0]) == "" {
		parts[0] = " "
	}
	out := make([]string, 0, len(parts))
	out = append(out, indent+"- "+strings.TrimSpace(parts[0]))
	cont := indent + strings.Repeat(" ", indentStepSpaces)
	for _, part := range parts[1:] {
		out = append(out, cont+part)
	}
	return out
}
