// Package domain defines core Dingovault data models shared across layers.
package domain

import "time"

// Block is the atomic unit of content in Dingovault (typically one logical line).
// Blocks form a tree per page/file via ParentID; the empty ParentID denotes a
// root block for that document subtree.
type Block struct {
	ID         string            `json:"id"`
	ParentID   string            `json:"parentId"`
	Content    string            `json:"content"`
	Properties map[string]string `json:"properties,omitempty"`
	Metadata   BlockMetadata     `json:"metadata"`
}

// BlockMetadata holds indexing and provenance fields separate from user-facing content.
type BlockMetadata struct {
	SourcePath string    `json:"sourcePath"`
	LineStart  int       `json:"lineStart"`
	LineEnd    int       `json:"lineEnd"`
	Level      int       `json:"level"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Root returns true when the block has no parent in the current document tree.
func (b Block) Root() bool {
	return b.ParentID == ""
}
