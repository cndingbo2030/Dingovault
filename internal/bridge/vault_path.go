package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/locale"
)

// resolveVaultMarkdownAbs returns the absolute path to a vault page, applying alias fallback when the file is missing.
func (a *App) resolveVaultMarkdownAbs(ctx context.Context, pagePath string) (string, error) {
	if a.notesRoot == "" {
		return "", fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	abs, err := graph.ResolveVaultPath(a.notesRoot, pagePath)
	if err != nil {
		return "", fmt.Errorf("%s: %w", a.t(locale.ErrResolvePath), err)
	}
	if a.store == nil {
		return abs, nil
	}
	if _, statErr := os.Stat(abs); statErr != nil {
		if alt, ok, _ := a.store.ResolveAliasToPath(ctx, a.notesRoot, pagePath); ok {
			return alt, nil
		}
		if base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs)); base != "" {
			if alt, ok, _ := a.store.ResolveAliasToPath(ctx, a.notesRoot, base); ok {
				return alt, nil
			}
		}
	}
	return abs, nil
}
