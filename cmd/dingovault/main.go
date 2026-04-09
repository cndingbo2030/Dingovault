package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cndingbo2030/dingovault/internal/auth"
	"github.com/cndingbo2030/dingovault/internal/blob"
	"github.com/cndingbo2030/dingovault/internal/bus"
	"github.com/cndingbo2030/dingovault/internal/config"
	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/plugins/summarizer"
	"github.com/cndingbo2030/dingovault/internal/scanner"
	"github.com/cndingbo2030/dingovault/internal/server"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
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
	eventBus := bus.New()
	graphSvc.SetBus(eventBus)
	_ = summarizer.Register(eventBus, store, engine)

	if handled, err := handleDebugCommand(store, dbFile, strings.TrimSpace(notesPath), flag.Args()); handled {
		if err != nil {
			log.Fatalf("debug command: %v", err)
		}
		return
	}

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

func handleDebugCommand(store *storage.Store, dbPath, notesPath string, args []string) (bool, error) {
	if len(args) == 0 || strings.ToLower(strings.TrimSpace(args[0])) != "debug" {
		return false, nil
	}
	if len(args) < 2 {
		return true, fmt.Errorf("usage: dingovault debug <graph|doctor|migrate-redo>")
	}
	switch strings.ToLower(strings.TrimSpace(args[1])) {
	case "graph":
		return true, runDebugGraph(store)
	case "doctor":
		return true, runDebugDoctor(store, dbPath, notesPath)
	case "migrate-redo":
		return true, runDebugMigrateRedo(store)
	default:
		return true, fmt.Errorf("unknown debug command %q (expected graph, doctor, or migrate-redo)", args[1])
	}
}

func runDebugGraph(store *storage.Store) error {
	st, err := store.IndexStats(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("graph summary:\n")
	fmt.Printf("  blocks: %d\n", st.BlockCount)
	fmt.Printf("  pages: %d\n", st.PageCount)
	fmt.Printf("  tenants: %d\n", st.TenantCount)
	return nil
}

func runDebugDoctor(store *storage.Store, dbPath, notesPath string) error {
	fmt.Println("doctor report:")
	fmt.Printf("  db_path: %s\n", dbPath)
	if err := reportPathPerms("db_file", dbPath); err != nil {
		return err
	}
	if err := reportPathPerms("db_dir", filepath.Dir(dbPath)); err != nil {
		return err
	}
	if strings.TrimSpace(notesPath) != "" {
		if err := reportPathPerms("notes_dir", notesPath); err != nil {
			return err
		}
	} else {
		fmt.Printf("  notes_dir: not set\n")
	}
	if err := reportSQLiteWAL(store.DB(), dbPath); err != nil {
		return err
	}
	reportJWTStrength()
	return nil
}

func reportPathPerms(label, path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s stat %q: %w", label, path, err)
	}
	mode := st.Mode().Perm()
	fmt.Printf("  %s_perms: %s (%#o)\n", label, mode.String(), mode)
	return nil
}

func reportSQLiteWAL(db *sql.DB, dbPath string) error {
	var mode string
	if err := db.QueryRowContext(context.Background(), `PRAGMA journal_mode`).Scan(&mode); err != nil {
		return fmt.Errorf("sqlite journal_mode: %w", err)
	}
	fmt.Printf("  sqlite_journal_mode: %s\n", strings.ToUpper(strings.TrimSpace(mode)))
	walPath := dbPath + "-wal"
	if st, err := os.Stat(walPath); err == nil {
		fmt.Printf("  sqlite_wal_file: present (%d bytes)\n", st.Size())
	} else if os.IsNotExist(err) {
		fmt.Printf("  sqlite_wal_file: not present\n")
	} else {
		return fmt.Errorf("sqlite wal stat %q: %w", walPath, err)
	}
	return nil
}

func reportJWTStrength() {
	secret := strings.TrimSpace(os.Getenv("DINGO_JWT_SECRET"))
	if secret == "" {
		secret = auth.DefaultDevSecret
		fmt.Printf("  jwt_secret_source: default-dev\n")
	} else {
		fmt.Printf("  jwt_secret_source: env\n")
	}
	n := len(secret)
	sum := sha256.Sum256([]byte(secret))
	fmt.Printf("  jwt_secret_len: %d\n", n)
	fmt.Printf("  jwt_secret_sha256_prefix: %x\n", sum[:4])
	switch {
	case n >= 48:
		fmt.Printf("  jwt_secret_strength: strong\n")
	case n >= 32:
		fmt.Printf("  jwt_secret_strength: good\n")
	case n >= 16:
		fmt.Printf("  jwt_secret_strength: weak\n")
	default:
		fmt.Printf("  jwt_secret_strength: invalid (<16 bytes)\n")
	}
}

func runDebugMigrateRedo(store *storage.Store) error {
	ctx := context.Background()
	db := store.DB()
	v, err := storage.ReadUserVersion(ctx, db)
	if err != nil {
		return err
	}
	if v <= 0 {
		fmt.Printf("migrate-redo: user_version=%d, nothing to redo\n", v)
		return nil
	}
	target := v - 1
	if err := storage.WriteUserVersion(ctx, db, target); err != nil {
		return err
	}
	if err := storage.RunSchemaMigrations(ctx, db); err != nil {
		return err
	}
	finalV, err := storage.ReadUserVersion(ctx, db)
	if err != nil {
		return err
	}
	fmt.Printf("migrate-redo: replayed from %d -> %d\n", target, finalV)
	return nil
}
