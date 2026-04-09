package graph

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/dingbo/dingovault/internal/storage"
)

// ResolveWikilink returns the absolute path for a [[wikilink]] target: existing file, else alias match, else the default would-be path.
func ResolveWikilink(ctx context.Context, st storage.Provider, notesRoot, target string) (string, error) {
	cand, err := WikilinkToAbsPath(notesRoot, target)
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
		return cand, nil
	}
	if abs, ok, err := st.ResolveAliasToPath(ctx, notesRoot, target); ok && err == nil {
		return abs, nil
	}
	// Match alias to basename-style targets (no path) when title differs from filename.
	base := strings.TrimSuffix(filepath.Base(cand), ".md")
	if base != "" && !strings.EqualFold(strings.TrimSpace(target), base) {
		if abs, ok, err := st.ResolveAliasToPath(ctx, notesRoot, base); ok && err == nil {
			return abs, nil
		}
	}
	return cand, nil
}
