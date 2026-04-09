package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dingbo/dingovault/internal/auth"
	"github.com/dingbo/dingovault/internal/blob"
	"github.com/dingbo/dingovault/internal/bus"
	"github.com/dingbo/dingovault/internal/config"
	"github.com/dingbo/dingovault/internal/graph"
	"github.com/dingbo/dingovault/internal/parser"
	"github.com/dingbo/dingovault/internal/scanner"
	"github.com/dingbo/dingovault/internal/server"
	"github.com/dingbo/dingovault/internal/storage"
	"github.com/dingbo/dingovault/internal/tenant"
)

const defaultSaaSPort = "12030"
const defaultDesktopDB = "dingovault.db"
const saasDBFile = "dingovault_saas.db"

func main() {
	dbPath := flag.String("db", defaultDesktopDB, "path to SQLite database file")
	notes := flag.String("notes", "", "directory of Markdown notes to index and watch (optional if saved in config)")
	serverFlag := flag.Bool("server", false, "run HTTP SaaS API on DINGO_PORT (default "+defaultSaaSPort+")")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Printf("config load: %v", err)
		cfg = config.Default()
	}
	notesPath := *notes
	if notesPath == "" {
		notesPath = cfg.VaultPath
	}

	httpMode := *serverFlag || os.Getenv("DINGO_SERVER") == "1" || os.Getenv("DINGO_PORT") != ""

	dbFile := *dbPath
	if httpMode && *dbPath == defaultDesktopDB {
		dbFile = filepath.Clean(saasDBFile)
		log.Printf("SaaS mode: using isolated database %s (override with -db)", dbFile)
	}

	store, err := storage.OpenSQLite(dbFile)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	engine := parser.NewEngine()
	graphSvc := graph.NewService(store, engine)
	graphSvc.SetBus(bus.New())

	var httpSrv *http.Server
	blobCtx := context.Background()
	assetBlobs, err := blob.NewProviderFromEnv(blobCtx, strings.TrimSpace(notesPath))
	if err != nil {
		log.Fatalf("asset blob storage: %v", err)
	}
	if assetBlobs != nil {
		log.Printf("asset uploads: blob backend active")
	}

	if httpMode {
		port := os.Getenv("DINGO_PORT")
		if port == "" {
			port = defaultSaaSPort
		}
		if _, err := strconv.Atoi(port); err != nil {
			log.Fatalf("invalid DINGO_PORT %q: %v", port, err)
		}

		jwtSvc, err := auth.NewJWTFromEnv("dingovault-api", 24*time.Hour, true)
		if err != nil {
			log.Fatalf("jwt: %v", err)
		}

		mux := http.NewServeMux()
		server.MountAPI(mux, store, jwtSvc, graphSvc, strings.TrimSpace(notesPath), assetBlobs)

		var handler http.Handler = mux
		if o := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS")); o != "" {
			handler = server.CORSMiddleware(o, mux)
			log.Printf("CORS enabled (ALLOWED_ORIGINS=%q)", o)
		}

		httpSrv = &http.Server{
			Addr:              ":" + port,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			log.Printf("SaaS API listening on http://127.0.0.1:%s (prefix /api/v1)", port)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("http server: %v", err)
			}
		}()
	}

	if notesPath == "" {
		if httpSrv == nil {
			log.Fatal("set -notes or save vaultPath via the desktop app config (or use -server without notes for API-only)")
		}
		log.Printf("API-only mode: no vault path; file watcher disabled")
		waitShutdown(httpSrv)
		return
	}

	idx, err := scanner.NewIndexer(notesPath, graphSvc)
	if err != nil {
		log.Fatalf("indexer: %v", err)
	}
	defer func() { _ = idx.Close() }()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx = tenant.WithUserID(ctx, tenant.LocalUserID)

	log.Printf("full scan of %s", notesPath)
	if err := idx.FullScan(ctx); err != nil {
		log.Fatalf("full scan: %v", err)
	}
	log.Printf("initial index complete; watching for changes")

	go func() {
		if err := idx.WatchRecursive(ctx); err != nil && ctx.Err() == nil {
			log.Printf("watcher stopped: %v", err)
			cancel()
		}
	}()

	if httpSrv != nil {
		go func() {
			<-ctx.Done()
			shCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shCancel()
			_ = httpSrv.Shutdown(shCtx)
		}()
	}

	<-ctx.Done()
	log.Printf("shutdown: %v", ctx.Err())
	if httpSrv != nil {
		shCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shCancel()
		_ = httpSrv.Shutdown(shCtx)
	}
}

func waitShutdown(srv *http.Server) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()
	shCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shCancel()
	_ = srv.Shutdown(shCtx)
	log.Printf("shutdown complete")
}
