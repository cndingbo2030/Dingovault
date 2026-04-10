//go:build !bindings

package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/ai"
	"github.com/cndingbo2030/dingovault/internal/bridge"
	"github.com/cndingbo2030/dingovault/internal/bus"
	"github.com/cndingbo2030/dingovault/internal/config"
	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/onboarding"
	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/plugins/embeddings"
	"github.com/cndingbo2030/dingovault/internal/plugins/summarizer"
	"github.com/cndingbo2030/dingovault/internal/scanner"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfg := loadConfig()
	notesPath := resolveDesktopNotesPath(cfg)
	store := openDesktopStore(cfg)
	defer closeProvider(store)

	graphSvc := setupDesktopGraph(store, cfg)
	idx := buildDesktopIndexer(notesPath, graphSvc, cfg.CloudMode)
	defer func() { _ = idx.Close() }()

	app := bridge.NewApp(store, graphSvc, notesPath)
	runDesktopApp(cfg, app, idx, notesPath)
}

func loadConfig() config.Config {
	dbPath := flag.String("db", "dingovault.db", "path to SQLite database file (ignored in cloud mode)")
	notes := flag.String("notes", "", "directory of Markdown notes to index and watch (optional if saved in config)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Printf("config load: %v", err)
		cfg = config.Default()
	}
	cfg.Window.Width = max(cfg.Window.Width, 1)
	cfg.Window.Height = max(cfg.Window.Height, 1)
	cfg.VaultPath = strings.TrimSpace(cfg.VaultPath)
	cfg.CloudAPIURL = strings.TrimSpace(cfg.CloudAPIURL)
	cfg.CloudToken = strings.TrimSpace(cfg.CloudToken)
	cfg.CloudMode = cfg.CloudMode || os.Getenv("DINGO_CLOUD_MODE") == "1"
	if v := strings.TrimSpace(os.Getenv("DINGO_CLOUD_URL")); v != "" {
		cfg.CloudAPIURL = v
	}
	if v := strings.TrimSpace(os.Getenv("DINGO_CLOUD_TOKEN")); v != "" {
		cfg.CloudToken = v
	}
	_ = dbPath
	_ = notes
	return cfg
}

func resolveDesktopNotesPath(cfg config.Config) string {
	notesPath := strings.TrimSpace(flag.Lookup("notes").Value.String())
	if notesPath == "" {
		notesPath = cfg.VaultPath
	}
	if notesPath == "" && config.ShouldOpenBundledDemo(flag.Lookup("notes").Value.String(), cfg) {
		if os.Getenv("DINGO_NO_DEMO_VAULT") == "1" {
			log.Fatal("no vault path: pass -notes, set vaultPath in config, or unset DINGO_NO_DEMO_VAULT to use the built-in Demo Vault")
		}
		demoDir, err := onboarding.EnsureDemoVaultFromFS(embeddedDemoVault, onboarding.DemoVaultRootName)
		if err != nil {
			log.Fatalf("demo vault: %v", err)
		}
		notesPath = demoDir
		log.Printf("no vault configured — opening built-in Demo Vault at %s (use -notes for your own folder)", notesPath)
	}
	if notesPath == "" {
		log.Fatal("set -notes to your vault directory (path is saved to config for next launch)")
	}
	if _, err := os.Stat(notesPath); err != nil {
		log.Fatalf("notes directory: %v", err)
	}
	return notesPath
}

func openDesktopStore(cfg config.Config) storage.Provider {
	var store storage.Provider
	if cfg.CloudMode {
		if cfg.CloudAPIURL == "" || cfg.CloudToken == "" {
			log.Fatal("cloud mode requires cloudApiUrl + cloudToken in config, or DINGO_CLOUD_URL + DINGO_CLOUD_TOKEN")
		}
		rs, err := storage.NewRemoteStore(cfg.CloudAPIURL, cfg.CloudToken)
		if err != nil {
			log.Fatalf("remote store: %v", err)
		}
		store = rs
		log.Printf("cloud mode: API %s (local vault path from config)", cfg.CloudAPIURL)
	} else {
		dbPath := flag.Lookup("db").Value.String()
		st, err := storage.OpenSQLite(dbPath)
		if err != nil {
			log.Fatalf("open database: %v", err)
		}
		store = st
	}
	return store
}

func closeProvider(store storage.Provider) {
	if err := store.Close(); err != nil {
		log.Printf("close store: %v", err)
	}
}

func setupDesktopGraph(store storage.Provider, cfg config.Config) *graph.Service {
	engine := parser.NewEngine()
	graphSvc := graph.NewService(store, engine)
	eventBus := bus.New()
	graphSvc.SetBus(eventBus)
	aiProv, err := ai.NewProvider(cfg.AI)
	if err != nil {
		log.Printf("ai provider: %v (LLM features use offline fallback where possible)", err)
	}
	var llm ai.LLMProvider
	if err == nil {
		llm = aiProv
	}
	_ = summarizer.Register(eventBus, store, engine, llm)
	_ = embeddings.Register(eventBus, store, llm)
	return graphSvc
}

func buildDesktopIndexer(notesPath string, graphSvc *graph.Service, cloudMode bool) *scanner.Indexer {
	idx, err := scanner.NewIndexer(notesPath, graphSvc)
	if err != nil {
		log.Fatalf("indexer: %v", err)
	}
	ctxScan := tenant.WithUserID(context.Background(), tenant.LocalUserID)
	log.Printf("full scan of %s", notesPath)
	if cloudMode {
		log.Printf("(cloud mode: indexing pushes markdown to the SaaS API)")
	}
	if err := idx.FullScan(ctxScan); err != nil {
		log.Fatalf("full scan: %v", err)
	}
	return idx
}

func runDesktopApp(cfg config.Config, app *bridge.App, idx *scanner.Indexer, notesPath string) {
	watchCtx, watchStop := context.WithCancel(context.Background())
	defer watchStop()

	err := wails.Run(&options.App{
		Title:  "Dingovault",
		Width:  cfg.Window.Width,
		Height: cfg.Window.Height,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Slight transparency so macOS vibrancy / translucency shows through the webview.
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 22, A: 235},
		Mac: &mac.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			runtime.WindowSetSize(ctx, cfg.Window.Width, cfg.Window.Height)
			if cfg.Window.X != 0 || cfg.Window.Y != 0 {
				runtime.WindowSetPosition(ctx, cfg.Window.X, cfg.Window.Y)
			}
			idx.SetOnFileChanged(func(path string) {
				runtime.EventsEmit(ctx, "file-updated", map[string]string{"path": path})
			})
			go func() {
				if werr := idx.WatchRecursive(watchCtx); werr != nil && werr != context.Canceled {
					log.Printf("watcher: %v", werr)
				}
			}()
		},
		OnShutdown: func(ctx context.Context) {
			app.StopLANSyncAdvertise()
			fresh, err := config.Load()
			if err != nil {
				log.Printf("config reload on shutdown: %v", err)
				fresh = cfg
			}
			fresh.VaultPath = notesPath
			w, h := runtime.WindowGetSize(ctx)
			x, y := runtime.WindowGetPosition(ctx)
			fresh.Window.Width = w
			fresh.Window.Height = h
			fresh.Window.X = x
			fresh.Window.Y = y
			if err := config.Save(fresh); err != nil {
				log.Printf("save config: %v", err)
			}
			watchStop()
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
