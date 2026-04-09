package graph

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"
)

// ApplySlashOp transforms the block's first physical line (and span for code fences) in the markdown file.
// op: today | todo | h1 | h2 | h3 | code
func (s *Service) ApplySlashOp(ctx context.Context, blockID, op string) error {
	op = strings.ToLower(strings.TrimSpace(op))
	switch op {
	case "today", "todo", "h1", "h2", "h3", "code":
	default:
		return fmt.Errorf("unknown slash op %q", op)
	}

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
	ls, le := b.Metadata.LineStart, b.Metadata.LineEnd
	if ls < 1 || le < ls || le > len(lines) {
		return fmt.Errorf("invalid line range %d-%d", ls, le)
	}

	first := strings.TrimRight(lines[ls-1], "\r")
	prefix, body, _ := splitMarkdownPrefix(first)

	switch op {
	case "today":
		d := time.Now().Format("2006-01-02")
		rest := strings.TrimSpace(stripTodoLead(body))
		var nb string
		if rest == "" {
			nb = d
		} else {
			nb = d + " " + rest
		}
		lines[ls-1] = prefix + nb
	case "todo":
		lines[ls-1] = prefix + forceTodoBody(body)
	case "h1", "h2", "h3":
		level := int(op[1] - '0')
		if level < 1 || level > 3 {
			return fmt.Errorf("invalid heading level")
		}
		title := strings.TrimSpace(stripTodoLead(body))
		if title == "" {
			title = " "
		}
		// Heading lines: no list indent preserved (zen outliner → top-level heading).
		lines[ls-1] = strings.Repeat("#", level) + " " + title
	case "code":
		text := strings.TrimSpace(stripTodoLead(body))
		if text == "" {
			text = " "
		}
		newChunk := []string{"```", text, "```"}
		lines = replaceLineRange(lines, ls, le, newChunk)
	default:
		return fmt.Errorf("unhandled op %q", op)
	}

	out := joinFileLines(lines, eol, trailingNL)
	if err := AtomicWriteFile(path, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	return s.ReindexFile(ctx, path)
}

func replaceLineRange(lines []string, lineStart, lineEnd int, replacement []string) []string {
	// 1-based inclusive lineStart, lineEnd
	if lineStart < 1 || lineEnd < lineStart {
		return lines
	}
	lo, hi := lineStart-1, lineEnd
	out := make([]string, 0, len(lines)-hi+lo+len(replacement))
	out = append(out, lines[:lo]...)
	out = append(out, replacement...)
	out = append(out, lines[hi:]...)
	return out
}

func forceTodoBody(body string) string {
	rest := strings.TrimSpace(stripTodoLead(body))
	if rest == "" {
		return "TODO"
	}
	return "TODO " + rest
}

func stripTodoLead(body string) string {
	s := strings.TrimLeftFunc(body, unicode.IsSpace)
	m := reTodoLeading.FindStringSubmatch(s)
	if m == nil {
		return body
	}
	return strings.TrimSpace(m[2])
}
