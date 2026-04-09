package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/fsnotify/fsnotify"
)

// Indexer performs an initial Markdown walk and keeps the graph in sync via fsnotify.
type Indexer struct {
	root    string
	graph   *graph.Service
	watcher *fsnotify.Watcher

	mu     sync.Mutex
	timers map[string]*time.Timer
	delay  time.Duration

	notifyMu      sync.RWMutex
	onFileChanged func(path string) // optional: e.g. Wails EventsEmit after successful reindex/delete
}

// NewIndexer watches root (recursively by registering each subdirectory) and forwards
// Markdown changes to graph.
func NewIndexer(root string, g *graph.Service) (*Indexer, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("abs notes root: %w", err)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify: %w", err)
	}
	return &Indexer{
		root:    absRoot,
		graph:   g,
		watcher: w,
		timers:  make(map[string]*time.Timer),
		delay:   75 * time.Millisecond,
	}, nil
}

// SetOnFileChanged registers a callback invoked after a successful incremental reindex or delete
// (not called for FullScan). Safe to call before WatchRecursive.
func (x *Indexer) SetOnFileChanged(fn func(path string)) {
	x.notifyMu.Lock()
	x.onFileChanged = fn
	x.notifyMu.Unlock()
}

func (x *Indexer) notifyFileChanged(path string) {
	x.notifyMu.RLock()
	fn := x.onFileChanged
	x.notifyMu.RUnlock()
	if fn != nil {
		fn(path)
	}
}

// Close stops timers and the underlying watcher.
func (x *Indexer) Close() error {
	x.mu.Lock()
	for _, t := range x.timers {
		t.Stop()
	}
	x.timers = nil
	x.mu.Unlock()
	return x.watcher.Close()
}

// FullScan walks the tree once and indexes every *.md file (read-only).
func (x *Indexer) FullScan(ctx context.Context) error {
	return filepath.WalkDir(x.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if x.shouldIgnorePath(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if x.shouldIgnorePath(path) {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		if err := x.graph.ReindexFile(ctx, path); err != nil {
			return fmt.Errorf("index %s: %w", path, err)
		}
		return nil
	})
}

// WatchRecursive registers fsnotify watches on root and every subdirectory, then processes
// events until ctx is cancelled.
func (x *Indexer) WatchRecursive(ctx context.Context) error {
	if err := filepath.WalkDir(x.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if x.shouldIgnorePath(path) {
			return filepath.SkipDir
		}
		if err := x.watcher.Add(path); err != nil {
			return fmt.Errorf("watch %s: %w", path, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("register watches: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-x.watcher.Errors:
			if !ok {
				return nil
			}
			if err != nil {
				log.Printf("dingovault watcher: %v", err)
				continue
			}
		case ev, ok := <-x.watcher.Events:
			if !ok {
				return nil
			}
			x.handleEvent(ctx, ev)
		}
	}
}

func (x *Indexer) handleEvent(ctx context.Context, ev fsnotify.Event) {
	absPath, err := filepath.Abs(ev.Name)
	if err != nil {
		return
	}
	if x.shouldIgnorePath(absPath) {
		return
	}

	if ev.Has(fsnotify.Create) && isDir(absPath) {
		_ = x.watcher.Add(absPath)
		return
	}

	if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
		if strings.EqualFold(filepath.Ext(absPath), ".md") {
			x.schedule(ctx, absPath, func(p string) {
				if err := x.graph.DeleteFile(ctx, p); err == nil {
					x.notifyFileChanged(p)
				}
			})
		}
		return
	}

	if !strings.EqualFold(filepath.Ext(absPath), ".md") {
		return
	}

	if ev.Has(fsnotify.Write) || ev.Has(fsnotify.Create) || ev.Has(fsnotify.Chmod) {
		x.schedule(ctx, absPath, func(p string) {
			if err := x.graph.ReindexFile(ctx, p); err == nil {
				x.notifyFileChanged(p)
			}
		})
	}
}

func (x *Indexer) schedule(ctx context.Context, path string, fn func(string)) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.timers == nil {
		return
	}
	if t, ok := x.timers[path]; ok {
		t.Stop()
	}
	p := path
	x.timers[path] = time.AfterFunc(x.delay, func() {
		x.mu.Lock()
		delete(x.timers, p)
		x.mu.Unlock()
		if ctx.Err() != nil {
			return
		}
		fn(p)
	})
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
