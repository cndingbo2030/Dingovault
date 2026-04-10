package mobile

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cndingbo2030/dingovault/internal/ai"
	"github.com/cndingbo2030/dingovault/internal/bridge"
	"github.com/cndingbo2030/dingovault/internal/bus"
	"github.com/cndingbo2030/dingovault/internal/config"
	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/onboarding"
	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/platform"
	"github.com/cndingbo2030/dingovault/internal/plugins/embeddings"
	"github.com/cndingbo2030/dingovault/internal/plugins/summarizer"
	"github.com/cndingbo2030/dingovault/internal/scanner"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

var (
	mu          sync.Mutex
	app         *bridge.App
	store       storage.Provider
	idx         *scanner.Indexer
	watchStop   context.CancelFunc
	initialized bool
)

// Init opens the SQLite store, indexes the vault, and prepares the bridge API.
// filesDir should be Context.getFilesDir(). externalFilesDir should be Context.getExternalFilesDir(null).
func Init(filesDir, externalFilesDir string) error {
	mu.Lock()
	defer mu.Unlock()
	if initialized {
		return nil
	}
	filesDir = filepath.Clean(strings.TrimSpace(filesDir))
	ext := strings.TrimSpace(externalFilesDir)
	if ext != "" {
		ext = filepath.Clean(ext)
	}
	if filesDir == "" || filesDir == "." {
		return fmt.Errorf("mobile: filesDir required")
	}

	config.SetDataDir(filepath.Join(filesDir, "dingovault-app"))

	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}
	vaultPath := strings.TrimSpace(cfg.VaultPath)
	if vaultPath == "" {
		vaultPath = platform.AndroidScopedVaultPath(ext)
	}
	if vaultPath == "" {
		vaultPath = filepath.Join(filesDir, "dingovault-vault")
	}
	if err := os.MkdirAll(vaultPath, 0o755); err != nil {
		return fmt.Errorf("vault dir: %w", err)
	}
	if err := onboarding.EnsureDemoVaultFromFSTo(vaultPath, embeddedDemoVault, onboarding.DemoVaultRootName); err != nil {
		return fmt.Errorf("demo vault: %w", err)
	}
	cfg.VaultPath = vaultPath
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	dbPath := filepath.Join(filesDir, "dingovault", "dingovault.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return err
	}
	st, err := storage.OpenSQLite(dbPath)
	if err != nil {
		return err
	}
	store = st

	engine := parser.NewEngine()
	graphSvc := graph.NewService(store, engine)
	eventBus := bus.New()
	graphSvc.SetBus(eventBus)
	aiProv, aerr := ai.NewProvider(cfg.AI)
	if aerr != nil {
		log.Printf("mobile ai provider: %v", aerr)
	}
	var llm ai.LLMProvider
	if aerr == nil {
		llm = aiProv
	}
	_ = summarizer.Register(eventBus, store, engine, llm)
	_ = embeddings.Register(eventBus, store, llm)

	app = bridge.NewApp(store, graphSvc, vaultPath)
	app.Startup(context.Background())
	app.EventEmitter = func(name string, payload map[string]any) {
		emitToSink(name, payload)
	}

	idx, err = scanner.NewIndexer(vaultPath, graphSvc)
	if err != nil {
		_ = store.Close()
		store = nil
		app = nil
		return err
	}
	ctxScan := tenant.WithUserID(context.Background(), tenant.LocalUserID)
	if err := idx.FullScan(ctxScan); err != nil {
		_ = idx.Close()
		idx = nil
		_ = store.Close()
		store = nil
		app = nil
		return err
	}

	idx.SetOnFileChanged(func(path string) {
		emitToSink("file-updated", map[string]any{"path": path})
	})
	watchCtx, cancel := context.WithCancel(context.Background())
	watchStop = cancel
	go func() {
		if werr := idx.WatchRecursive(watchCtx); werr != nil && werr != context.Canceled {
			log.Printf("mobile watcher: %v", werr)
		}
	}()

	initialized = true
	return nil
}

// Shutdown stops the file watcher and closes databases.
func Shutdown() {
	mu.Lock()
	defer mu.Unlock()
	if !initialized {
		return
	}
	if watchStop != nil {
		watchStop()
		watchStop = nil
	}
	if idx != nil {
		_ = idx.Close()
		idx = nil
	}
	if app != nil {
		app.StopLANSyncAdvertise()
		app = nil
	}
	if store != nil {
		_ = store.Close()
		store = nil
	}
	initialized = false
	config.SetDataDir("")
}
