package parser

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Wikilink is an inline AST node for Logseq-style [[page]] or [[page|alias]].
type Wikilink struct {
	ast.BaseInline
	Target []byte
	Alias  []byte
}

var kindWikilink = ast.NewNodeKind("DingovaultWikilink")

// Kind implements ast.Node.Kind.
func (n *Wikilink) Kind() ast.NodeKind {
	return kindWikilink
}

// Dump implements ast.Node.Dump.
func (n *Wikilink) Dump(source []byte, level int) {
	m := map[string]string{"Target": string(n.Target)}
	if len(n.Alias) > 0 {
		m["Alias"] = string(n.Alias)
	}
	ast.DumpHelper(n, source, level, m, nil)
}

// NewWikilink returns a wikilink node with trimmed target and optional display alias.
func NewWikilink(target, alias []byte) *Wikilink {
	return &Wikilink{
		Target: trimSpaceBytes(target),
		Alias:  trimSpaceBytes(alias),
	}
}

// HashTag is an inline AST node for #tags discovered after plain-text tokenization.
type HashTag struct {
	ast.BaseInline
	Name    []byte
	Segment text.Segment
}

var kindHashTag = ast.NewNodeKind("DingovaultHashTag")

// Kind implements ast.Node.Kind.
func (n *HashTag) Kind() ast.NodeKind {
	return kindHashTag
}

// Dump implements ast.Node.Dump.
func (n *HashTag) Dump(source []byte, level int) {
	m := map[string]string{"Name": string(n.Name)}
	ast.DumpHelper(n, source, level, m, nil)
}

// NewHashTag constructs a tag node; seg covers the full "#name" span in the source.
func NewHashTag(name []byte, seg text.Segment) *HashTag {
	return &HashTag{
		Name:    append([]byte(nil), name...),
		Segment: seg,
	}
}

func trimSpaceBytes(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
