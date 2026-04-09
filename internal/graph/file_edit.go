package graph

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reFenceOpen = regexp.MustCompile(`^(\s*)` + "```" + `[a-zA-Z0-9_-]*\s*$`)
	reHeading   = regexp.MustCompile(`^(\s*#{1,6}\s+)(.*)$`)
	reBullet    = regexp.MustCompile(`^(\s*[-*+]\s+)(.*)$`)
	reOrdered   = regexp.MustCompile(`^(\s*)(\d+)\.(\s+)(.*)$`)
	rePlain     = regexp.MustCompile(`^(\s*)(.*)$`)
)

func splitFileLines(data []byte) (lines []string, eol string, trailingNL bool, err error) {
	if len(data) == 0 {
		return []string{""}, "\n", false, nil
	}
	eol = "\n"
	raw := string(data)
	if bytes.Contains(data, []byte("\r\n")) {
		eol = "\r\n"
		raw = strings.ReplaceAll(raw, "\r\n", "\n")
	}
	trailingNL = strings.HasSuffix(raw, "\n")
	if trailingNL {
		raw = strings.TrimSuffix(raw, "\n")
	}
	if raw == "" {
		return []string{""}, eol, trailingNL, nil
	}
	lines = strings.Split(raw, "\n")
	return lines, eol, trailingNL, nil
}

func joinFileLines(lines []string, eol string, trailingNL bool) []byte {
	s := strings.Join(lines, eol)
	if trailingNL {
		s += eol
	}
	return []byte(s)
}

// AtomicWriteFile writes data to path by creating a temp file in the same directory, syncing, then renaming.
func AtomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".dingovault.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := f.Name()
	clean := true
	defer func() {
		if clean {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename to target: %w", err)
	}
	clean = false
	return nil
}

// ReplaceBlockLineRange replaces 1-based inclusive line range [lineStart, lineEnd] with lines derived from newContent.
// Markdown list / heading prefixes on the first original line are preserved so sibling/nested structure below is untouched.
func ReplaceBlockLineRange(file []byte, lineStart, lineEnd int, newContent string) ([]byte, error) {
	lines, eol, trailingNL, err := splitFileLines(file)
	if err != nil {
		return nil, err
	}
	if lineStart < 1 || lineEnd < lineStart || lineEnd > len(lines) {
		return nil, fmt.Errorf("line range %d-%d invalid for file with %d lines", lineStart, lineEnd, len(lines))
	}

	oldChunk := make([]string, lineEnd-lineStart+1)
	copy(oldChunk, lines[lineStart-1:lineEnd])

	newLines := buildReplacementLines(oldChunk, newContent)
	out := make([]string, 0, len(lines)-len(oldChunk)+len(newLines))
	out = append(out, lines[:lineStart-1]...)
	out = append(out, newLines...)
	out = append(out, lines[lineEnd:]...)
	return joinFileLines(out, eol, trailingNL), nil
}

// InsertLinesAfter inserts new physical lines immediately after 1-based line index `afterLine`.
func InsertLinesAfter(file []byte, afterLine int, newLines []string) ([]byte, error) {
	lines, eol, trailingNL, err := splitFileLines(file)
	if err != nil {
		return nil, err
	}
	if afterLine < 0 || afterLine > len(lines) {
		return nil, fmt.Errorf("afterLine %d invalid (lines=%d)", afterLine, len(lines))
	}
	insertAt := afterLine // 0-based index of insertion (line after `afterLine` is at index afterLine)
	out := make([]string, 0, len(lines)+len(newLines))
	out = append(out, lines[:insertAt]...)
	out = append(out, newLines...)
	out = append(out, lines[insertAt:]...)
	return joinFileLines(out, eol, trailingNL), nil
}

func buildReplacementLines(oldLines []string, newContent string) []string {
	nc := strings.ReplaceAll(newContent, "\r\n", "\n")
	parts := strings.Split(nc, "\n")
	if len(parts) == 1 && parts[0] == "" {
		parts = []string{""}
	}
	if len(oldLines) == 0 {
		return parts
	}

	first := oldLines[0]
	if reFenceOpen.MatchString(strings.TrimRight(first, "\r")) || strings.HasPrefix(strings.TrimLeft(first, " \t"), "```") {
		// Fenced / code-ish: replace raw span with user text split into lines (no prefix magic).
		return parts
	}

	prefix, _, kind := splitMarkdownPrefix(first)
	cont := continuationPad(oldLines, prefix, kind)

	out := make([]string, 0, len(parts))
	out = append(out, prefix+strings.TrimSpace(parts[0]))
	for i := 1; i < len(parts); i++ {
		out = append(out, cont+parts[i])
	}
	return out
}

func splitMarkdownPrefix(line string) (prefix string, body string, kind string) {
	line = strings.TrimRight(line, "\r")
	if m := reHeading.FindStringSubmatch(line); m != nil {
		return m[1], m[2], "heading"
	}
	if m := reBullet.FindStringSubmatch(line); m != nil {
		return m[1], m[2], "bullet"
	}
	if m := reOrdered.FindStringSubmatch(line); m != nil {
		return m[1] + m[2] + "." + m[3], m[4], "ordered"
	}
	if m := rePlain.FindStringSubmatch(line); m != nil {
		return m[1], m[2], "plain"
	}
	return "", line, "plain"
}

func continuationPad(oldLines []string, prefix string, kind string) string {
	if len(oldLines) > 1 {
		if m := regexp.MustCompile(`^(\s*)`).FindStringSubmatch(oldLines[1]); m != nil {
			return m[1]
		}
	}
	if kind == "bullet" || kind == "ordered" {
		if m := regexp.MustCompile(`^(\s*)`).FindStringSubmatch(prefix); m != nil {
			return m[1] + "  "
		}
	}
	if kind == "heading" {
		return ""
	}
	if m := regexp.MustCompile(`^(\s*)`).FindStringSubmatch(prefix); m != nil {
		return m[1] + "  "
	}
	return "  "
}

// NewSiblingLine builds one Markdown line to insert after anchorLine (first line of preceding block).
func NewSiblingLine(anchorLine string, text string) string {
	anchorLine = strings.TrimRight(anchorLine, "\r")
	text = strings.TrimSpace(text)
	if text == "" {
		text = " "
	}

	if m := reBullet.FindStringSubmatch(anchorLine); m != nil {
		indent := leadingListIndent(anchorLine)
		return indent + "- " + text
	}
	if m := reOrdered.FindStringSubmatch(anchorLine); m != nil {
		n, _ := strconv.Atoi(m[2])
		return fmt.Sprintf("%s%d.%s%s", m[1], n+1, m[3], text)
	}
	if m := reHeading.FindStringSubmatch(anchorLine); m != nil {
		return text
	}
	if strings.TrimSpace(anchorLine) == "" {
		return text
	}
	return text
}

func leadingListIndent(line string) string {
	m := reBullet.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	pre := m[1]
	for i, r := range pre {
		if r == '-' || r == '*' || r == '+' {
			return pre[:i]
		}
	}
	return ""
}
