package scanner

import (
	"path/filepath"
	"strings"
)

// ignoredDirNames are path segments that should never be watched or indexed (large / VCS trees).
var ignoredDirNames = map[string]struct{}{
	".git":         {},
	".svn":         {},
	".hg":          {},
	"node_modules": {},
	"__pycache__":  {},
	".venv":        {},
	"venv":         {},
	".idea":        {},
	".vscode":      {},
}

// shouldIgnorePath returns true if absPath (under the indexer root) passes through a skipped directory.
func (x *Indexer) shouldIgnorePath(absPath string) bool {
	rel, err := filepath.Rel(x.root, absPath)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return true
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			continue
		}
		if _, ok := ignoredDirNames[seg]; ok {
			return true
		}
	}
	return false
}
