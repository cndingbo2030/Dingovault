package parser

import (
	"bytes"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type tagTransformer struct{}

// NewTagASTTransformer runs after inline parsing and splits plain *ast.Text runs into
// Text + HashTag fragments for Logseq-style #tags (letters, digits, _, /, -, Unicode letters).
func NewTagASTTransformer() parser.ASTTransformer {
	return &tagTransformer{}
}

func (t *tagTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	var batch []*ast.Text
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		tn, ok := n.(*ast.Text)
		if !ok || tn.IsRaw() {
			return ast.WalkContinue, nil
		}
		if inCodeSpan(tn) {
			return ast.WalkContinue, nil
		}
		if bytes.IndexByte(tn.Value(source), '#') < 0 {
			return ast.WalkContinue, nil
		}
		batch = append(batch, tn)
		return ast.WalkContinue, nil
	})
	for _, tn := range batch {
		splitTextForHashTags(tn, source)
	}
}

func inCodeSpan(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == ast.KindCodeSpan {
			return true
		}
	}
	return false
}

func splitTextForHashTags(t *ast.Text, source []byte) {
	parent := t.Parent()
	if parent == nil {
		return
	}
	seg := t.Segment
	raw := seg.Value(source)
	if len(raw) == 0 {
		return
	}

	frags := collectTagFragments(raw)
	if len(frags) == 0 {
		return
	}
	onlyText := len(frags) == 1 && frags[0].kind == 't' && frags[0].start == 0 && frags[0].end == len(raw)
	if onlyText {
		return
	}

	next := t.NextSibling()
	parent.RemoveChild(parent, t)
	base := seg.Start
	for _, f := range frags {
		absStart := base + f.start
		absEnd := base + f.end
		s := text.NewSegment(absStart, absEnd)
		switch f.kind {
		case 't':
			if absStart >= absEnd {
				continue
			}
			// InsertBefore(self, v1, insertee): place insertee before v1; v1 nil => append.
			parent.InsertBefore(parent, next, ast.NewTextSegment(s))
		case 'h':
			parent.InsertBefore(parent, next, NewHashTag(f.name, s))
		}
	}
}

type fragment struct {
	kind  byte // 't' text, 'h' hashtag
	start int
	end   int
	name  []byte
}

func collectTagFragments(raw []byte) []fragment {
	var frags []fragment
	for i := 0; i < len(raw); {
		if raw[i] != '#' {
			start, next := scanPlainText(raw, i)
			if start < next {
				frags = append(frags, fragment{kind: 't', start: start, end: next})
			}
			i = next
			continue
		}
		f, next := scanHashTag(raw, i)
		frags = append(frags, f)
		i = next
	}
	return frags
}

func scanPlainText(raw []byte, i int) (start int, next int) {
	start = i
	for i < len(raw) && raw[i] != '#' {
		i++
	}
	return start, i
}

func scanHashTag(raw []byte, i int) (fragment, int) {
	// ## at markdown-style — keep literal first '#', continue scanning from next char
	if i+1 < len(raw) && raw[i+1] == '#' {
		return fragment{kind: 't', start: i, end: i + 1}, i + 1
	}
	j := i + 1
	if j >= len(raw) {
		return fragment{kind: 't', start: i, end: i + 1}, i + 1
	}
	r, w := utf8.DecodeRune(raw[j:])
	if !isTagStartRune(r) {
		return fragment{kind: 't', start: i, end: i + 1}, i + 1
	}
	j += w
	for j < len(raw) {
		r2, w2 := utf8.DecodeRune(raw[j:])
		if !isTagContinueRune(r2) {
			break
		}
		j += w2
	}
	if j <= i+1 {
		return fragment{kind: 't', start: i, end: i + 1}, i + 1
	}
	return fragment{kind: 'h', start: i, end: j, name: raw[i+1 : j]}, j
}

func isTagStartRune(r rune) bool {
	if r == utf8.RuneError {
		return false
	}
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_'
}

func isTagContinueRune(r rune) bool {
	if r == utf8.RuneError {
		return false
	}
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '/' || r == '-'
}

var _ parser.ASTTransformer = (*tagTransformer)(nil)
