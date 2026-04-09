package parser

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dingbo/dingovault/internal/domain"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// WikilinkRef records a wikilink emitted by a specific block.
type WikilinkRef struct {
	SourceBlockID string
	Target        string
	Alias         string
}

// TagRef records a hashtag attached to a block.
type TagRef struct {
	BlockID string
	Tag     string
}

// ParseResult is the output of Markdown parsing for one file.
type ParseResult struct {
	Blocks    []domain.Block
	Wikilinks []WikilinkRef
	Tags      []TagRef
}

// Engine holds a reusable Goldmark parser configured for Dingovault (wikilinks + #tags).
type Engine struct {
	gp parser.Parser
}

// NewEngine builds a parser with:
//   - default block/inline parsers, plus a [[wikilink]] inline parser before LinkParser;
//   - an AST transformer that splits #tags out of plain text (skipping code spans).
func NewEngine() *Engine {
	inline := append([]util.PrioritizedValue{
		util.Prioritized(NewWikilinkInlineParser(), 150),
	}, parser.DefaultInlineParsers()...)

	gp := parser.NewParser(
		parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(inline...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
		parser.WithASTTransformers(
			util.Prioritized(NewTagASTTransformer(), 1000),
		),
	)
	return &Engine{gp: gp}
}

// ParseSource parses UTF-8 Markdown into blocks, wikilinks, and tags. It does not read
// or write the filesystem.
func (e *Engine) ParseSource(src []byte, sourcePath string) (ParseResult, error) {
	if !utf8.Valid(src) {
		return ParseResult{}, fmt.Errorf("source is not valid UTF-8")
	}
	reader := text.NewReader(src)
	doc := e.gp.Parse(reader)
	root, ok := doc.(*ast.Document)
	if !ok {
		return ParseResult{}, fmt.Errorf("unexpected root node %T", doc)
	}

	lineStarts := buildLineStarts(src)
	var out ParseResult
	now := time.Now().UTC()

	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		e.emitTopLevelBlock(n, src, sourcePath, lineStarts, now, &out)
	}
	return out, nil
}

func (e *Engine) emitTopLevelBlock(
	n ast.Node,
	src []byte,
	sourcePath string,
	lineStarts []int,
	now time.Time,
	out *ParseResult,
) {
	switch n.Kind() {
	case ast.KindParagraph:
		p := n.(*ast.Paragraph)
		e.emitParagraphBlock(p, src, sourcePath, "", lineStarts, 0, now, out)
	case ast.KindHeading:
		h := n.(*ast.Heading)
		e.emitHeadingBlock(h, src, sourcePath, lineStarts, now, out)
	case ast.KindList:
		l := n.(*ast.List)
		e.walkList(l, src, sourcePath, "", lineStarts, 0, now, out)
	case ast.KindFencedCodeBlock:
		fb := n.(*ast.FencedCodeBlock)
		e.emitFencedCodeBlock(fb, src, sourcePath, lineStarts, now, out)
	case ast.KindCodeBlock:
		cb := n.(*ast.CodeBlock)
		e.emitIndentedCodeBlock(cb, src, sourcePath, lineStarts, now, out)
	case ast.KindBlockquote:
		bq := n.(*ast.Blockquote)
		for c := bq.FirstChild(); c != nil; c = c.NextSibling() {
			e.emitTopLevelBlock(c, src, sourcePath, lineStarts, now, out)
		}
	default:
		// ThematicBreak, HTMLBlock, etc. — skip empty structural nodes
	}
}

func (e *Engine) emitParagraphBlock(
	p *ast.Paragraph,
	src []byte,
	sourcePath, parentID string,
	lineStarts []int,
	outline int,
	now time.Time,
	out *ParseResult,
) {
	lineStart, lineEnd := segmentsLineRange(p.Lines(), lineStarts)
	id := StableBlockID(sourcePath, lineStart, lineEnd)
	content := strings.TrimSpace(string(p.Lines().Value(src)))
	out.Blocks = append(out.Blocks, domain.Block{
		ID:         id,
		ParentID:   parentID,
		Content:    content,
		Properties: nil,
		Metadata: domain.BlockMetadata{
			SourcePath: sourcePath,
			LineStart:  lineStart,
			LineEnd:    lineEnd,
			Level:      outline,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	})
	collectRefs(id, p, src, out)
}

func (e *Engine) emitHeadingBlock(
	h *ast.Heading,
	src []byte,
	sourcePath string,
	lineStarts []int,
	now time.Time,
	out *ParseResult,
) {
	lineStart, lineEnd := segmentsLineRange(h.Lines(), lineStarts)
	id := StableBlockID(sourcePath, lineStart, lineEnd)
	content := strings.TrimSpace(string(h.Lines().Value(src)))
	out.Blocks = append(out.Blocks, domain.Block{
		ID:         id,
		ParentID:   "",
		Content:    content,
		Properties: nil,
		Metadata: domain.BlockMetadata{
			SourcePath: sourcePath,
			LineStart:  lineStart,
			LineEnd:    lineEnd,
			Level:      h.Level,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	})
	collectRefs(id, h, src, out)
}

func (e *Engine) emitFencedCodeBlock(
	fb *ast.FencedCodeBlock,
	src []byte,
	sourcePath string,
	lineStarts []int,
	now time.Time,
	out *ParseResult,
) {
	lineStart, lineEnd := segmentsLineRange(fb.Lines(), lineStarts)
	id := StableBlockID(sourcePath, lineStart, lineEnd)
	content := strings.TrimSpace(string(fb.Lines().Value(src)))
	out.Blocks = append(out.Blocks, domain.Block{
		ID:         id,
		ParentID:   "",
		Content:    content,
		Properties: nil,
		Metadata: domain.BlockMetadata{
			SourcePath: sourcePath,
			LineStart:  lineStart,
			LineEnd:    lineEnd,
			Level:      0,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	})
}

func (e *Engine) emitIndentedCodeBlock(
	cb *ast.CodeBlock,
	src []byte,
	sourcePath string,
	lineStarts []int,
	now time.Time,
	out *ParseResult,
) {
	lineStart, lineEnd := segmentsLineRange(cb.Lines(), lineStarts)
	id := StableBlockID(sourcePath, lineStart, lineEnd)
	content := strings.TrimSpace(string(cb.Lines().Value(src)))
	out.Blocks = append(out.Blocks, domain.Block{
		ID:         id,
		ParentID:   "",
		Content:    content,
		Properties: nil,
		Metadata: domain.BlockMetadata{
			SourcePath: sourcePath,
			LineStart:  lineStart,
			LineEnd:    lineEnd,
			Level:      0,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	})
}

func (e *Engine) walkList(
	list *ast.List,
	src []byte,
	sourcePath, parentID string,
	lineStarts []int,
	outline int,
	now time.Time,
	out *ParseResult,
) {
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		li, ok := item.(*ast.ListItem)
		if !ok {
			continue
		}
		// ListItem.Lines() spans the whole list in Goldmark; use the first paragraph/text block instead.
		lineStart, lineEnd := listItemBodyLineRange(li, lineStarts)
		id := StableBlockID(sourcePath, lineStart, lineEnd)
		content := strings.TrimSpace(listItemPlainText(li, src))
		out.Blocks = append(out.Blocks, domain.Block{
			ID:         id,
			ParentID:   parentID,
			Content:    content,
			Properties: nil,
			Metadata: domain.BlockMetadata{
				SourcePath: sourcePath,
				LineStart:  lineStart,
				LineEnd:    lineEnd,
				Level:      outline,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
		})
		collectRefs(id, li, src, out)

		for c := li.FirstChild(); c != nil; c = c.NextSibling() {
			if nested, ok := c.(*ast.List); ok {
				e.walkList(nested, src, sourcePath, id, lineStarts, outline+1, now, out)
			}
		}
	}
}

func listItemBodyLineRange(li *ast.ListItem, lineStarts []int) (int, int) {
	for c := li.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindParagraph:
			p := c.(*ast.Paragraph)
			return segmentsLineRange(p.Lines(), lineStarts)
		case ast.KindTextBlock:
			tb := c.(*ast.TextBlock)
			return segmentsLineRange(tb.Lines(), lineStarts)
		default:
			// Nested lists handled separately in walkList.
		}
	}
	return segmentsLineRange(li.Lines(), lineStarts)
}

func listItemPlainText(li *ast.ListItem, src []byte) string {
	var buf bytes.Buffer
	for c := li.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindParagraph:
			p := c.(*ast.Paragraph)
			buf.Write(p.Lines().Value(src))
		case ast.KindTextBlock:
			tb := c.(*ast.TextBlock)
			buf.Write(tb.Lines().Value(src))
		default:
			// Nested lists handled in walkList; skip here
		}
	}
	return buf.String()
}

func collectRefs(blockID string, root ast.Node, src []byte, out *ParseResult) {
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch x := n.(type) {
		case *Wikilink:
			out.Wikilinks = append(out.Wikilinks, WikilinkRef{
				SourceBlockID: blockID,
				Target:        string(x.Target),
				Alias:         string(x.Alias),
			})
		case *HashTag:
			out.Tags = append(out.Tags, TagRef{
				BlockID: blockID,
				Tag:     string(x.Name),
			})
		}
		return ast.WalkContinue, nil
	})
}

func buildLineStarts(src []byte) []int {
	starts := []int{0}
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func segmentsLineRange(lines *text.Segments, lineStarts []int) (int, int) {
	if lines == nil || lines.Len() == 0 {
		return 1, 1
	}
	first := lines.At(0)
	last := lines.At(lines.Len() - 1)
	startLine := offsetToLine(lineStarts, first.Start)
	endLine := offsetToLine(lineStarts, max(0, last.Stop-1))
	return startLine, endLine
}

func offsetToLine(lineStarts []int, offset int) int {
	if len(lineStarts) == 0 {
		return 1
	}
	lo, hi := 0, len(lineStarts)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		start := lineStarts[mid]
		var next int
		if mid+1 < len(lineStarts) {
			next = lineStarts[mid+1]
		} else {
			next = 1 << 30
		}
		if offset >= start && offset < next {
			return mid + 1
		}
		if offset < start {
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}
	return len(lineStarts)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
