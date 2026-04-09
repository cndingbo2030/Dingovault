package graph

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const indentStepSpaces = 2

// IndentBlock adds two leading spaces to the block line (and its nested subtree for list items),
// then re-indexes the file. Parent/level in SQLite come from the parser after ReindexFile.
func (s *Service) IndentBlock(ctx context.Context, blockID string) error {
	return s.adjustBlockIndent(ctx, blockID, indentStepSpaces)
}

// OutdentBlock removes two leading spaces from the same span; fails if any affected line has fewer than two leading spaces.
func (s *Service) OutdentBlock(ctx context.Context, blockID string) error {
	return s.adjustBlockIndent(ctx, blockID, -indentStepSpaces)
}

func (s *Service) adjustBlockIndent(ctx context.Context, blockID string, delta int) error {
	if delta != indentStepSpaces && delta != -indentStepSpaces {
		return fmt.Errorf("invalid indent delta %d", delta)
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
	idx := collectSubtreeLineIndices(lines, ls, le)
	if len(idx) == 0 {
		return fmt.Errorf("no lines to adjust for block %s", blockID)
	}

	newLines, err := applyIndentShift(lines, idx, delta)
	if err != nil {
		return err
	}
	out := joinFileLines(newLines, eol, trailingNL)
	if err := AtomicWriteFile(path, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	return s.ReindexFile(ctx, path)
}

// collectSubtreeLineIndices returns 0-based line indices to shift: always the block's [ls,le],
// and for list blocks, following lines until a line at the same or lesser indent (non-blank).
func collectSubtreeLineIndices(lines []string, ls, le int) []int {
	if ls < 1 || le < ls || le > len(lines) {
		return nil
	}
	start := ls - 1
	end := le - 1
	first := strings.TrimRight(lines[start], "\r")
	baseLead := leadingWhitespacePrefixLen(first)

	if !isListMarkerLine(first) {
		var out []int
		for i := start; i <= end; i++ {
			out = append(out, i)
		}
		return out
	}

	var out []int
	for i := start; i < len(lines); i++ {
		if i <= end {
			out = append(out, i)
			continue
		}
		line := strings.TrimRight(lines[i], "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if leadingWhitespacePrefixLen(line) > baseLead {
			out = append(out, i)
			continue
		}
		break
	}
	return out
}

func isListMarkerLine(line string) bool {
	s := strings.TrimRight(line, "\r")
	return reBullet.MatchString(s) || reOrdered.MatchString(s)
}

func leadingWhitespacePrefixLen(line string) int {
	n := 0
	for _, r := range line {
		if r == ' ' || r == '\t' {
			n++
			continue
		}
		break
	}
	return n
}

func applyIndentShift(lines []string, indices []int, delta int) ([]string, error) {
	out := append([]string(nil), lines...)
	if delta > 0 {
		pad := strings.Repeat(" ", delta)
		for _, i := range indices {
			if i < 0 || i >= len(out) {
				continue
			}
			out[i] = pad + out[i]
		}
		return out, nil
	}
	rem := -delta
	for _, i := range indices {
		if i < 0 || i >= len(out) {
			continue
		}
		trimmed, ok := stripLeadingASCIISpaces(out[i], rem)
		if !ok {
			return nil, fmt.Errorf("cannot outdent: line %d has fewer than %d leading spaces", i+1, rem)
		}
		out[i] = trimmed
	}
	return out, nil
}

func stripLeadingASCIISpaces(line string, n int) (string, bool) {
	if len(line) < n {
		return "", false
	}
	for i := 0; i < n; i++ {
		if line[i] != ' ' {
			return "", false
		}
	}
	return line[n:], true
}
