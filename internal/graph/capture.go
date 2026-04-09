package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppendQuickCapture appends bullet line(s) to the end of a page (EnsurePage if missing), then reindexes.
// Multi-line text becomes one list item with indented continuation lines.
func (s *Service) AppendQuickCapture(ctx context.Context, absPath, text string) error {
	absPath = filepath.Clean(absPath)
	if err := s.EnsurePage(ctx, absPath); err != nil {
		return err
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	lines, eol, _, err := splitFileLines(raw)
	if err != nil {
		return err
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("empty capture text")
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	parts := strings.Split(text, "\n")
	var add []string
	if len(parts) == 1 {
		add = []string{"- " + parts[0]}
	} else {
		add = append(add, "- "+parts[0])
		for _, p := range parts[1:] {
			add = append(add, "  "+p)
		}
	}

	lines = append(lines, add...)
	out := joinFileLines(lines, eol, true)
	if err := AtomicWriteFile(absPath, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	return s.ReindexFile(ctx, absPath)
}
