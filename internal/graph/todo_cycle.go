package graph

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var reTodoLeading = regexp.MustCompile(`(?i)^(TODO|DOING|DONE)(?:\s+|$)(.*)$`)

// CycleBlockTodo advances TODO → DOING → DONE → (clear) on the first physical line's body
// (text after list marker / heading marker). Atomic write + ReindexFile.
func (s *Service) CycleBlockTodo(ctx context.Context, blockID string) error {
	return s.mutateBlockFirstLine(ctx, blockID, cycleTodoBody)
}

func cycleTodoBody(body string) string {
	s := strings.TrimSpace(body)
	if s == "" {
		return "TODO"
	}
	m := reTodoLeading.FindStringSubmatch(s)
	if m == nil {
		return "TODO " + s
	}
	kw := strings.ToUpper(m[1])
	rest := strings.TrimSpace(m[2])
	switch kw {
	case "TODO":
		if rest == "" {
			return "DOING"
		}
		return "DOING " + rest
	case "DOING":
		if rest == "" {
			return "DONE"
		}
		return "DONE " + rest
	case "DONE":
		return rest
	default:
		return "TODO " + s
	}
}

func (s *Service) mutateBlockFirstLine(ctx context.Context, blockID string, fn func(body string) string) error {
	b, err := s.store.GetBlockByID(ctx, blockID)
	if err != nil {
		return fmt.Errorf("lookup block: %w", err)
	}
	path := b.Metadata.SourcePath
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	lines, eol, trailingNL, err := splitFileLines(data)
	if err != nil {
		return err
	}
	ls := b.Metadata.LineStart
	if ls < 1 || ls > len(lines) {
		return fmt.Errorf("invalid line start %d", ls)
	}

	first := strings.TrimRight(lines[ls-1], "\r")
	prefix, body, _ := splitMarkdownPrefix(first)
	newBody := fn(body)
	lines[ls-1] = prefix + newBody

	out := joinFileLines(lines, eol, trailingNL)
	if err := AtomicWriteFile(path, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	return s.ReindexFile(ctx, path)
}
