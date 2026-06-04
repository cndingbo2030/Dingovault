package bridge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cndingbo2030/dingovault/internal/config"
	"github.com/cndingbo2030/dingovault/internal/domain"
	"github.com/cndingbo2030/dingovault/internal/export"
	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/locale"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
	"github.com/cndingbo2030/dingovault/internal/terminal"
	"github.com/cndingbo2030/dingovault/internal/version"
)

// App is the Wails-facing API surface (bound to the frontend).
type App struct {
	ctx       context.Context
	store     storage.Provider
	graph     *graph.Service
	notesRoot string

	// EventEmitter is optional (Android WebView). When set, code that would use Wails runtime events
	// should prefer this path so the native shell can forward to JavaScript.
	EventEmitter func(name string, payload map[string]any)

	lanMu   sync.Mutex
	stopLAN func()

	terminalMu      sync.Mutex
	terminalManager *terminal.Manager

	pageMu       sync.Mutex
	pageCacheAbs string
	pageCacheMod int64
	pageCacheAt  time.Time
	pageCacheBuf []PageBlock

	healthRescan func(context.Context) error
}

// VaultFileDTO is a vault-relative file entry exposed to the desktop UI.
type VaultFileDTO struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	Ext          string `json:"ext"`
	Kind         string `json:"kind"`
	Size         int64  `json:"size"`
	ModifiedUnix int64  `json:"modifiedUnix"`
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

// Shutdown releases app-owned runtime resources.
func (a *App) Shutdown(_ context.Context) {
	a.shutdownTerminalSessions()
}

func (a *App) uiLocale() string {
	c, err := config.Load()
	if err != nil {
		return "en"
	}
	return locale.Normalize(c.Locale)
}

func (a *App) t(key string) string {
	return locale.Message(a.uiLocale(), key)
}

// GetLocale returns the persisted locale tag (e.g. en, zh-CN) or empty if never set (first run).
func (a *App) GetLocale() (string, error) {
	c, err := config.Load()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(c.Locale) == "" {
		return "", nil
	}
	return locale.Normalize(c.Locale), nil
}

// SetLocale persists UI language; only en and zh-CN are supported for now.
func (a *App) SetLocale(tag string) error {
	n := locale.Normalize(tag)
	if !locale.Supported(n) {
		return fmt.Errorf("%s", a.t(locale.ErrLocaleUnsupported))
	}
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	c.Locale = n
	return config.Save(c)
}

// GetAppVersion returns the build version (set via -ldflags for release binaries).
func (a *App) GetAppVersion() string {
	return version.String
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
		return fmt.Errorf("%s", a.t(locale.ErrThemeInvalid))
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
		return nil, fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
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
		return nil, fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
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

// ListVaultFiles returns supported user files in the vault, including Markdown,
// Office/WPS documents, PDF, images, and CAD drawings.
func (a *App) ListVaultFiles() ([]VaultFileDTO, error) {
	if strings.TrimSpace(a.notesRoot) == "" {
		return nil, fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	root := a.NotesRoot()
	var out []VaultFileDTO
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && shouldSkipVaultFileDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		kind := vaultFileKind(ext)
		if kind == "" {
			return nil
		}
		rel, err := graph.VaultRelativePath(root, path)
		if err != nil {
			return nil
		}
		var size int64
		var modified int64
		if info, statErr := d.Info(); statErr == nil {
			size = info.Size()
			modified = info.ModTime().Unix()
		}
		out = append(out, VaultFileDTO{
			Path:         rel,
			Name:         name,
			Ext:          strings.TrimPrefix(ext, "."),
			Kind:         kind,
			Size:         size,
			ModifiedUnix: modified,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return vaultFileKindRank(out[i].Kind) < vaultFileKindRank(out[j].Kind)
		}
		return strings.ToLower(out[i].Path) < strings.ToLower(out[j].Path)
	})
	return out, nil
}

// OpenVaultFile opens a non-Markdown vault file with the operating system's default app.
func (a *App) OpenVaultFile(path string) error {
	if strings.TrimSpace(a.notesRoot) == "" {
		return fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	abs, err := graph.ResolveVaultPath(a.notesRoot, path)
	if err != nil {
		return fmt.Errorf("%s: %w", a.t(locale.ErrResolvePath), err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("cannot open directory")
	}
	if vaultFileKind(filepath.Ext(abs)) == "" {
		return fmt.Errorf("unsupported file type")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", abs)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", abs)
	default:
		cmd = exec.Command("xdg-open", abs)
	}
	return cmd.Start()
}

func shouldSkipVaultFileDir(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return n == "" || strings.HasPrefix(n, ".") || n == "node_modules" || n == "vendor"
}

func vaultFileKind(ext string) string {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "md", "markdown":
		return "markdown"
	case "doc", "docx", "xls", "xlsx", "ppt", "pptx", "wps", "et", "dps":
		return "office"
	case "pdf":
		return "pdf"
	case "png", "jpg", "jpeg", "gif", "webp", "svg":
		return "image"
	case "dwg", "dxf":
		return "cad"
	default:
		return ""
	}
}

func vaultFileKindRank(kind string) int {
	switch kind {
	case "markdown":
		return 0
	case "office":
		return 1
	case "pdf":
		return 2
	case "image":
		return 3
	case "cad":
		return 4
	default:
		return 9
	}
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
		return nil, fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
	}
	abs, err := graph.ResolveVaultPath(a.notesRoot, path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", a.t(locale.ErrResolvePath), err)
	}
	if !strings.EqualFold(filepath.Ext(abs), ".md") {
		return nil, fmt.Errorf("%s", a.t(locale.ErrNotMarkdown))
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
	var modNs int64
	if st, err := os.Stat(abs); err == nil {
		modNs = st.ModTime().UnixNano()
	}
	a.pageMu.Lock()
	if a.pageCacheAbs == abs && a.pageCacheMod == modNs && len(a.pageCacheBuf) > 0 && time.Since(a.pageCacheAt) < 8*time.Second {
		out := append([]PageBlock(nil), a.pageCacheBuf...)
		a.pageMu.Unlock()
		return out, nil
	}
	a.pageMu.Unlock()

	blocks, err := a.store.ListDomainBlocksBySourcePath(ctx, abs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", a.t(locale.ErrListBlocks), err)
	}
	out := buildPageTree(blocks)
	a.pageMu.Lock()
	a.pageCacheAbs = abs
	a.pageCacheMod = modNs
	a.pageCacheBuf = append([]PageBlock(nil), out...)
	a.pageCacheAt = time.Now()
	a.pageMu.Unlock()
	return out, nil
}

func (a *App) invalidatePageCache() {
	a.pageMu.Lock()
	a.pageCacheBuf = nil
	a.pageCacheAbs = ""
	a.pageMu.Unlock()
}

// UpdateBlock surgically replaces the block's line span in the backing file and re-indexes.
func (a *App) UpdateBlock(blockID, newContent string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err := a.graph.UpdateBlock(ctx, blockID, newContent)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// InsertBlockAfter appends a new Markdown line after the given block (Logseq-style Enter).
func (a *App) InsertBlockAfter(blockID, initialText string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err := a.graph.InsertBlockAfter(ctx, blockID, initialText)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// InsertChildBlock appends a new direct child under the given Markdown list block.
func (a *App) InsertChildBlock(parentID, initialText string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err := a.graph.InsertChildBlock(ctx, parentID, initialText)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// ReorderBlockBefore moves movingID immediately before beforeID among sibling blocks in the same file.
func (a *App) ReorderBlockBefore(movingID, beforeID string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err := a.graph.ReorderSiblingBefore(ctx, movingID, beforeID)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// MoveBlockUnder moves a block subtree to become the last child of a target block in the same Markdown file.
func (a *App) MoveBlockUnder(movingID, newParentID string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err := a.graph.MoveBlockUnder(ctx, movingID, newParentID)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// GetWikiGraph returns indexed pages as nodes and resolved wikilinks as directed edges.
func (a *App) GetWikiGraph() (storage.WikiGraph, error) {
	if a.store == nil {
		return storage.WikiGraph{}, fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
	}
	if a.notesRoot == "" {
		return storage.WikiGraph{}, fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.store.WikiGraph(ctx, a.notesRoot)
}

// GetSemanticGraphEdges returns page–page edges derived from embedding similarity (local SQLite only).
func (a *App) GetSemanticGraphEdges() ([]storage.WikiGraphSemanticEdge, error) {
	if a.store == nil {
		return nil, fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
	}
	if a.notesRoot == "" {
		return nil, fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	c.AI = config.NormalizeAISettings(c.AI)
	const minCos float32 = 0.58
	const maxEdges = 72
	return a.store.SemanticPageEdges(ctx, c.AI.EmbeddingsModel, minCos, maxEdges)
}

// IndentBlock increases list indentation by two spaces for the block (and nested list lines under it).
func (a *App) IndentBlock(blockID string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err := a.graph.IndentBlock(ctx, blockID)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// OutdentBlock decreases list indentation by two spaces for the same span.
func (a *App) OutdentBlock(blockID string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err := a.graph.OutdentBlock(ctx, blockID)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// CycleBlockTodo cycles TODO → DOING → DONE → (clear) on the first line of the block in the file.
func (a *App) CycleBlockTodo(blockID string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err := a.graph.CycleBlockTodo(ctx, blockID)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// ApplySlashOp applies a slash command to the block: today, todo, h1, h2, h3, code.
func (a *App) ApplySlashOp(blockID, op string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err := a.graph.ApplySlashOp(ctx, blockID, op)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// EnsurePage creates path if missing (vault-relative or absolute under vault).
func (a *App) EnsurePage(path string) error {
	if a.graph == nil {
		return fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	abs, err := graph.ResolveVaultPath(a.notesRoot, path)
	if err != nil {
		return fmt.Errorf("%s: %w", a.t(locale.ErrResolvePath), err)
	}
	if !strings.EqualFold(filepath.Ext(abs), ".md") {
		abs += ".md"
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	err = a.graph.EnsurePage(ctx, abs)
	if err == nil {
		a.invalidatePageCache()
	}
	return err
}

// ResolveWikilink returns the absolute .md path for a [[wikilink]] target string.
func (a *App) ResolveWikilink(target string) (string, error) {
	if a.notesRoot == "" {
		return "", fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	if a.store == nil {
		return "", fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
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
		return nil, fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
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
		return fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
	}
	if a.notesRoot == "" {
		return fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	abs, err := graph.ResolveVaultPath(a.notesRoot, pagePath)
	if err != nil {
		return fmt.Errorf("%s: %w", a.t(locale.ErrResolvePath), err)
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
		return fmt.Errorf("%s: %w", a.t(locale.ErrReadPage), err)
	}
	title := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	htmlBytes, err := export.MarkdownFileToStandaloneHTML(raw, title)
	if err != nil {
		return err
	}
	if err := os.WriteFile(destPath, htmlBytes, 0o644); err != nil {
		return fmt.Errorf("%s: %w", a.t(locale.ErrWriteExport), err)
	}
	return nil
}

// GetBacklinks returns blocks (any page) whose content links to the given vault-relative page via [[wikilinks]].
func (a *App) GetBacklinks(pagePath string) ([]domain.Block, error) {
	if a.graph == nil {
		return nil, fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
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
		return nil, fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.graph.QueryBlocks(ctx, dsl)
}
