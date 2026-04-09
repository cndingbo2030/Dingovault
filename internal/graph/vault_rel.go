package graph

import (
	"fmt"
	"path/filepath"
	"strings"
)

// VaultRelativePath returns a slash-separated path relative to notesRoot, or an error if absPath is outside the vault.
func VaultRelativePath(notesRoot, absPath string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(notesRoot))
	if err != nil {
		return "", fmt.Errorf("notes root: %w", err)
	}
	abs, err := filepath.Abs(filepath.Clean(absPath))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside vault")
	}
	return filepath.ToSlash(rel), nil
}
