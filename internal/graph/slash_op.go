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
	if !validSlashOp(op) {
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

	lines, err = applySlashOpToLines(lines, ls, le, op)
	if err != nil {
		return err
	}

	out := joinFileLines(lines, eol, trailingNL)
	if err := AtomicWriteFile(path, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	return s.ReindexFile(ctx, path)
}

func validSlashOp(op string) bool {
	switch op {
	case "today", "todo", "h1", "h2", "h3", "code":
		return true
	default:
		return false
	}
}

func applySlashOpToLines(lines []string, ls, le int, op string) ([]string, error) {
	first := strings.TrimRight(lines[ls-1], "\r")
	switch op {
	case "today":
		return applySlashToday(lines, ls, first)
	case "todo":
		return applySlashTodo(lines, ls, first)
	case "h1", "h2", "h3":
		return applySlashHeading(lines, ls, op, first)
	case "code":
		return applySlashCodeBlock(lines, ls, le, first)
	default:
		return nil, fmt.Errorf("unhandled op %q", op)
	}
}

func applySlashToday(lines []string, ls int, firstLine string) ([]string, error) {
	prefix, body, _ := splitMarkdownPrefix(firstLine)
	lines[ls-1] = prefix + buildTodayBody(body)
	return lines, nil
}

func applySlashTodo(lines []string, ls int, firstLine string) ([]string, error) {
	prefix, body, _ := splitMarkdownPrefix(firstLine)
	lines[ls-1] = prefix + forceTodoBody(body)
	return lines, nil
}

func applySlashHeading(lines []string, ls int, op, firstLine string) ([]string, error) {
	_, body, _ := splitMarkdownPrefix(firstLine)
	line, err := buildHeadingLine(op, body)
	if err != nil {
		return nil, err
	}
	lines[ls-1] = line
	return lines, nil
}

func applySlashCodeBlock(lines []string, ls, le int, firstLine string) ([]string, error) {
	_, body, _ := splitMarkdownPrefix(firstLine)
	lines = replaceLineRange(lines, ls, le, buildCodeFenceChunk(body))
	return lines, nil
}

func buildTodayBody(body string) string {
	d := time.Now().Format("2006-01-02")
	rest := strings.TrimSpace(stripTodoLead(body))
	if rest == "" {
		return d
	}
	return d + " " + rest
}

func buildHeadingLine(op, body string) (string, error) {
	level := int(op[1] - '0')
	if level < 1 || level > 3 {
		return "", fmt.Errorf("invalid heading level")
	}
	title := strings.TrimSpace(stripTodoLead(body))
	if title == "" {
		title = " "
	}
	// Heading lines: no list indent preserved (zen outliner → top-level heading).
	return strings.Repeat("#", level) + " " + title, nil
}

func buildCodeFenceChunk(body string) []string {
	text := strings.TrimSpace(stripTodoLead(body))
	if text == "" {
		text = " "
	}
	return []string{"```", text, "```"}
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
