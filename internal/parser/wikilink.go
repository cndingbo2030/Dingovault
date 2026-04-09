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
	line, _ := block.PeekLine()
	if line == nil || len(line) < 2 || line[0] != '[' || line[1] != '[' {
		return nil
	}

	block.Advance(2)
	line, _ = block.PeekLine()
	if line == nil {
		block.SetPosition(savedLine, savedPos)
		return nil
	}

	closeIdx := -1
	for i := 0; i < len(line)-1; i++ {
		if line[i] == ']' && line[i+1] == ']' {
			// Skip escaped ] — uncommon inside wikilinks; respect backslash escape
			if i > 0 && line[i-1] == '\\' {
				esc := 0
				for j := i - 1; j >= 0 && line[j] == '\\'; j-- {
					esc++
				}
				if esc%2 == 1 {
					continue
				}
			}
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		block.SetPosition(savedLine, savedPos)
		return nil
	}

	inner := line[:closeIdx]
	target, alias := splitWikilinkInner(inner)
	if len(target) == 0 {
		block.SetPosition(savedLine, savedPos)
		return nil
	}

	block.Advance(closeIdx + 2)
	return NewWikilink(target, alias)
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
