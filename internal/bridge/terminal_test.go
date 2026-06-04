package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

func TestRunBlockCommandRejectsUnconfirmedUnsafeCommand(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	mdPath := filepath.Join(dir, "p.md")
	if err := os.WriteFile(mdPath, []byte("- run command\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.OpenSQLite(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	svc := graph.NewService(store, parser.NewEngine())
	ctx := tenant.WithUserID(context.Background(), tenant.LocalUserID)
	if err := svc.ReindexFile(ctx, mdPath); err != nil {
		t.Fatal(err)
	}
	blocks, err := store.ListDomainBlocksBySourcePath(ctx, mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(blocks))
	}

	app := NewApp(store, svc, dir)

	_, err = app.RunBlockCommand(blocks[0].ID, fmt.Sprintf("printf hacked > %s", marker), "", false)
	if err == nil || !strings.Contains(err.Error(), "requires confirmation") {
		t.Fatalf("error = %v, want confirmation requirement", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("marker stat error = %v, want file not created", statErr)
	}
	after, err := store.ListDomainBlocksBySourcePath(ctx, mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("blocks after rejected command = %d, want 1", len(after))
	}
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "- run command\n" {
		t.Fatalf("markdown was modified after rejected command:\n%s", string(raw))
	}
}

func TestTerminalManagerShutdownOnRootSwitch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty lifecycle test requires a Unix-style PTY")
	}
	dirA := t.TempDir()
	dirB := t.TempDir()
	rec := &bridgeEventRecorder{ch: make(chan map[string]any, 16)}
	app := NewApp(nil, nil, dirA)
	app.EventEmitter = rec.emit

	mgrA, err := app.getTerminalManager()
	if err != nil {
		t.Fatal(err)
	}
	info, err := mgrA.StartSession(context.Background(), "", 20, 80)
	if err != nil {
		t.Fatal(err)
	}

	app.notesRoot = dirB
	mgrB, err := app.getTerminalManager()
	if err != nil {
		t.Fatal(err)
	}
	if mgrB == mgrA {
		t.Fatal("root switch reused terminal manager")
	}
	if err := mgrA.WriteInput(info.ID, "printf leak\n"); err == nil {
		t.Fatal("old manager accepted input after root switch")
	}
	if !rec.waitExit(info.ID, 3*time.Second) {
		t.Fatalf("old session did not exit after root switch; events=%v", rec.snapshot())
	}
}

func TestAppShutdownClosesTerminalSessions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty lifecycle test requires a Unix-style PTY")
	}
	rec := &bridgeEventRecorder{ch: make(chan map[string]any, 16)}
	app := NewApp(nil, nil, t.TempDir())
	app.EventEmitter = rec.emit

	mgr, err := app.getTerminalManager()
	if err != nil {
		t.Fatal(err)
	}
	info, err := mgr.StartSession(context.Background(), "", 20, 80)
	if err != nil {
		t.Fatal(err)
	}

	app.Shutdown(context.Background())
	if err := mgr.WriteInput(info.ID, "printf leak\n"); err == nil {
		t.Fatal("manager accepted input after app shutdown")
	}
	if !rec.waitExit(info.ID, 3*time.Second) {
		t.Fatalf("session did not exit after app shutdown; events=%v", rec.snapshot())
	}
}

type bridgeEventRecorder struct {
	mu     sync.Mutex
	events []map[string]any
	ch     chan map[string]any
}

func (r *bridgeEventRecorder) emit(name string, payload map[string]any) {
	ev := map[string]any{"name": name}
	for k, v := range payload {
		ev[k] = v
	}
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
	r.ch <- ev
}

func (r *bridgeEventRecorder) waitExit(sessionID string, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-r.ch:
			if ev["name"] == "terminal-exit" && ev["sessionId"] == sessionID {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func (r *bridgeEventRecorder) snapshot() []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]map[string]any, len(r.events))
	copy(out, r.events)
	return out
}
