package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dingbo/dingovault/internal/config"
	"github.com/dingbo/dingovault/internal/domain"
	"github.com/dingbo/dingovault/internal/export"
	"github.com/dingbo/dingovault/internal/graph"
	"github.com/dingbo/dingovault/internal/storage"
	"github.com/dingbo/dingovault/internal/tenant"
)

// App is the Wails-facing API surface (bound to the frontend).
type App struct {
	ctx       context.Context
	store     storage.Provider
	graph     *graph.Service
	notesRoot string
}

// NewApp constructs the bridge.
func NewApp(store storage.Provider, g *graph.Service, notesRoot string) *App {
	return &App{
		store:     store,
		graph:     g,
		notesRoot: notesRoot,
	}
}

// Startup is called by Wails on init; ctx is used for runtime events later.
func (a *App) Startup(ctx context.Context) {
	a.ctx = tenant.WithUserID(ctx, tenant.LocalUserID)
}

// GetTheme returns persisted UI theme: "dark" or "light".
func (a *App) GetTheme() (string, error) {
	c, err := config.Load()
	if err != nil {
		return "dark", err
	}
	if c.Theme != "light" && c.Theme != "dark" {
		return "dark", nil
	}
	return c.Theme, nil
}

// SetTheme persists theme and should be paired with updating document.documentElement.dataset.theme in the UI.
func (a *App) SetTheme(theme string) error {
	theme = strings.ToLower(strings.TrimSpace(theme))
	if theme != "light" && theme != "dark" {
		return fmt.Errorf("theme must be light or dark")
	}
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	c.Theme = theme
	return config.Save(c)
}

// SearchBlocks queries the FTS5 index (blocks_fts) and returns ranked hits with snippets.
func (a *App) SearchBlocks(query string) ([]storage.BlockSearchHit, error) {
	if a.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.store.SearchBlocksFTSWithAliases(ctx, query, 50)
}

// ListVaultPages returns vault-relative .md paths for all indexed pages, most recently updated first.
func (a *App) ListVaultPages() ([]string, error) {
	if a.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	absPaths, err := a.store.ListSourcePathsByRecency(ctx, 3000)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range absPaths {
		rel, err := graph.VaultRelativePath(a.notesRoot, p)
		if err != nil {
			continue
		}
		if !strings.EqualFold(filepath.Ext(rel), ".md") {
			continue
		}
		out = append(out, rel)
	}
	return out, nil
}

// NotesRoot returns the configured vault directory (absolute path).
func (a *App) NotesRoot() string {
	abs, err := filepath.Abs(a.notesRoot)
	if err != nil {
		return a.notesRoot
	}
	return abs
}

// GetPage loads all blocks for a vault-relative or absolute Markdown path and returns a tree of roots.
func (a *App) GetPage(path string) ([]PageBlock, error) {
	if a.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	abs, err := graph.ResolveVaultPath(a.notesRoot, path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	if !strings.EqualFold(filepath.Ext(abs), ".md") {
		return nil, fmt.Errorf("not a markdown path: %s", path)
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	if _, statErr := os.Stat(abs); statErr != nil {
		if alt, ok, _ := a.store.ResolveAliasToPath(ctx, a.notesRoot, path); ok {
			abs = alt
		} else if base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs)); base != "" {
			if alt, ok, _ := a.store.ResolveAliasToPath(ctx, a.notesRoot, base); ok {
				abs = alt
			}
		}
	}
	blocks, err := a.store.ListDomainBlocksBySourcePath(ctx, abs)
	if err != nil {
		return nil, fmt.Errorf("list blocks: %w", err)
	}
	return buildPageTree(blocks), nil
}

// UpdateBlock surgically replaces the block's line span in the backing file and re-indexes.
func (a *App) UpdateBlock(blockID, newContent string) error {
	if a.graph == nil {
		return fmt.Errorf("graph not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.UpdateBlock(ctx, blockID, newContent)
}

// InsertBlockAfter appends a new Markdown line after the given block (Logseq-style Enter).
func (a *App) InsertBlockAfter(blockID, initialText string) error {
	if a.graph == nil {
		return fmt.Errorf("graph not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.InsertBlockAfter(ctx, blockID, initialText)
}

// IndentBlock increases list indentation by two spaces for the block (and nested list lines under it).
func (a *App) IndentBlock(blockID string) error {
	if a.graph == nil {
		return fmt.Errorf("graph not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.IndentBlock(ctx, blockID)
}

// OutdentBlock decreases list indentation by two spaces for the same span.
func (a *App) OutdentBlock(blockID string) error {
	if a.graph == nil {
		return fmt.Errorf("graph not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.OutdentBlock(ctx, blockID)
}

// CycleBlockTodo cycles TODO → DOING → DONE → (clear) on the first line of the block in the file.
func (a *App) CycleBlockTodo(blockID string) error {
	if a.graph == nil {
		return fmt.Errorf("graph not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.CycleBlockTodo(ctx, blockID)
}

// ApplySlashOp applies a slash command to the block: today, todo, h1, h2, h3, code.
func (a *App) ApplySlashOp(blockID, op string) error {
	if a.graph == nil {
		return fmt.Errorf("graph not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.ApplySlashOp(ctx, blockID, op)
}

// EnsurePage creates path if missing (vault-relative or absolute under vault).
func (a *App) EnsurePage(path string) error {
	if a.graph == nil {
		return fmt.Errorf("graph not initialized")
	}
	abs, err := graph.ResolveVaultPath(a.notesRoot, path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	if !strings.EqualFold(filepath.Ext(abs), ".md") {
		abs += ".md"
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.EnsurePage(ctx, abs)
}

// ResolveWikilink returns the absolute .md path for a [[wikilink]] target string.
func (a *App) ResolveWikilink(target string) (string, error) {
	if a.notesRoot == "" {
		return "", fmt.Errorf("notes root not set")
	}
	if a.store == nil {
		return "", fmt.Errorf("store not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return graph.ResolveWikilink(ctx, a.store, a.notesRoot, target)
}

// ListPagesByProperty returns vault-relative .md paths whose YAML frontmatter has prop_key = prop_value (case-insensitive).
func (a *App) ListPagesByProperty(key, value string) ([]string, error) {
	if a.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	absPaths, err := a.store.ListSourcePathsByPageProperty(ctx, key, value)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range absPaths {
		rel, err := graph.VaultRelativePath(a.notesRoot, p)
		if err != nil {
			continue
		}
		out = append(out, rel)
	}
	return out, nil
}

// ExportPageHTML writes a standalone HTML file for a vault page using Goldmark. destPath should be absolute.
func (a *App) ExportPageHTML(pagePath, destPath string) error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	if a.notesRoot == "" {
		return fmt.Errorf("notes root not set")
	}
	abs, err := graph.ResolveVaultPath(a.notesRoot, pagePath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	if !strings.EqualFold(filepath.Ext(abs), ".md") {
		abs += ".md"
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	if _, statErr := os.Stat(abs); statErr != nil {
		if alt, ok, _ := a.store.ResolveAliasToPath(ctx, a.notesRoot, pagePath); ok {
			abs = alt
		} else if base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs)); base != "" {
			if alt, ok, _ := a.store.ResolveAliasToPath(ctx, a.notesRoot, base); ok {
				abs = alt
			}
		}
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("read page: %w", err)
	}
	title := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	htmlBytes, err := export.MarkdownFileToStandaloneHTML(raw, title)
	if err != nil {
		return err
	}
	if err := os.WriteFile(destPath, htmlBytes, 0o644); err != nil {
		return fmt.Errorf("write export: %w", err)
	}
	return nil
}

// GetBacklinks returns blocks (any page) whose content links to the given vault-relative page via [[wikilinks]].
func (a *App) GetBacklinks(pagePath string) ([]domain.Block, error) {
	if a.graph == nil {
		return nil, fmt.Errorf("graph not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.GetBacklinks(ctx, a.notesRoot, pagePath)
}

// QueryBlocks runs a small query DSL: "key:value" for properties, otherwise FTS on content.
func (a *App) QueryBlocks(dsl string) ([]domain.Block, error) {
	if a.graph == nil {
		return nil, fmt.Errorf("graph not initialized")
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.QueryBlocks(ctx, dsl)
}
