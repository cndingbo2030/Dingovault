package parser

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type wikilinkParser struct{}

// NewWikilinkInlineParser registers on '[' and wins over the standard Link parser when the
// lookahead is '[['. Priority should be lower numeric than LinkParser (200), e.g. 150.
func NewWikilinkInlineParser() parser.InlineParser {
	return &wikilinkParser{}
}

func (w *wikilinkParser) Trigger() []byte {
	return []byte{'['}
}

func (w *wikilinkParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	savedLine, savedPos := block.Position()
	if !advancePastWikilinkOpenBracketPair(block) {
		return nil
	}

	line, _ := block.PeekLine()
	if line == nil {
		return rollback(block, savedLine, savedPos)
	}

	closeIdx := findWikilinkClose(line)
	if closeIdx < 0 {
		return rollback(block, savedLine, savedPos)
	}

	inner := line[:closeIdx]
	target, alias := splitWikilinkInner(inner)
	if len(target) == 0 {
		return rollback(block, savedLine, savedPos)
	}

	block.Advance(closeIdx + 2)
	return NewWikilink(target, alias)
}

// advancePastWikilinkOpenBracketPair consumes "[[" when present; otherwise leaves the reader unchanged.
func advancePastWikilinkOpenBracketPair(block text.Reader) bool {
	line, _ := block.PeekLine()
	if !startsWikilink(line) {
		return false
	}
	block.Advance(2)
	return true
}

func startsWikilink(line []byte) bool {
	return line != nil && len(line) >= 2 && line[0] == '[' && line[1] == '['
}

func rollback(block text.Reader, savedLine int, savedPos text.Segment) ast.Node {
	block.SetPosition(savedLine, savedPos)
	return nil
}

func findWikilinkClose(line []byte) int {
	for i := 0; i < len(line)-1; i++ {
		if line[i] != ']' || line[i+1] != ']' {
			continue
		}
		if isEscapedBracket(line, i) {
			continue
		}
		return i
	}
	return -1
}

func isEscapedBracket(line []byte, idx int) bool {
	if idx <= 0 || line[idx-1] != '\\' {
		return false
	}
	esc := 0
	for j := idx - 1; j >= 0 && line[j] == '\\'; j-- {
		esc++
	}
	return esc%2 == 1
}

func splitWikilinkInner(inner []byte) (target, alias []byte) {
	idx := bytes.IndexByte(inner, '|')
	if idx < 0 {
		return inner, nil
	}
	return inner[:idx], inner[idx+1:]
}

// Ensure wikilinkParser is only used where valid (satisfies goldmark constraints).
var _ parser.InlineParser = (*wikilinkParser)(nil)
