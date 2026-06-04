package terminal

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestResolveCwd_Table(t *testing.T) {
	root := t.TempDir()
	m, err := NewManager(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "project")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		cwd     string
		want    string
		wantErr string
	}{
		{name: "empty uses root", cwd: "", want: root},
		{name: "relative child", cwd: "project", want: child},
		{name: "absolute child", cwd: child, want: child},
		{name: "escape rejected", cwd: "..", wantErr: "escapes"},
		{name: "missing rejected", cwd: "missing", wantErr: "not available"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.ResolveCwd(tt.cwd)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != filepath.Clean(tt.want) {
				t.Fatalf("cwd = %q, want %q", got, filepath.Clean(tt.want))
			}
		})
	}
}

func TestSessionLifecycleEchoAndKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty lifecycle test requires a Unix-style PTY")
	}
	events := newEventRecorder()
	m, err := NewManager(t.TempDir(), events.emit)
	if err != nil {
		t.Fatal(err)
	}

	info, err := m.StartSession(context.Background(), "", 20, 80)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID == "" || info.Cwd == "" {
		t.Fatalf("session info = %+v", info)
	}
	if err := m.WriteInput(info.ID, "printf 'DINGO_LIFE\\n'\n"); err != nil {
		t.Fatal(err)
	}
	if !events.waitOutput(info.ID, "DINGO_LIFE", 3*time.Second) {
		t.Fatalf("did not capture echo output; events=%v", events.snapshot())
	}
	if err := m.Resize(info.ID, 30, 120); err != nil {
		t.Fatal(err)
	}
	if err := m.CloseSession(info.ID); err != nil {
		t.Fatal(err)
	}
	if !events.waitEvent(EventExit, info.ID, 3*time.Second) {
		t.Fatalf("did not receive exit event; events=%v", events.snapshot())
	}
}

func TestRunCommandCapturesPTYOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty command test requires a Unix-style PTY")
	}
	events := newEventRecorder()
	m, err := NewManager(t.TempDir(), events.emit)
	if err != nil {
		t.Fatal(err)
	}

	got, err := m.RunCommand(context.Background(), "", "printf 'DINGO_RUN\\n'")
	if err != nil {
		t.Fatal(err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("exitCode = %d, output = %q", got.ExitCode, got.Output)
	}
	if !strings.Contains(got.Output, "DINGO_RUN") {
		t.Fatalf("output = %q, want marker", got.Output)
	}
	if !events.waitEvent(EventExit, got.SessionID, 2*time.Second) {
		t.Fatalf("did not receive command exit event; events=%v", events.snapshot())
	}
}

func TestShutdownClosesAllSessions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty lifecycle test requires a Unix-style PTY")
	}
	events := newEventRecorder()
	m, err := NewManager(t.TempDir(), events.emit)
	if err != nil {
		t.Fatal(err)
	}

	first, err := m.StartSession(context.Background(), "", 20, 80)
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.StartSession(context.Background(), "", 20, 80)
	if err != nil {
		t.Fatal(err)
	}
	m.Shutdown()

	if err := m.WriteInput(first.ID, "printf leak\n"); err == nil {
		t.Fatal("WriteInput after Shutdown succeeded, want session not found")
	}
	if err := m.WriteInput(second.ID, "printf leak\n"); err == nil {
		t.Fatal("WriteInput after Shutdown succeeded, want session not found")
	}
	if !events.waitEvents(EventExit, []string{first.ID, second.ID}, 3*time.Second) {
		t.Fatalf("did not receive exit events for all sessions; events=%v", events.snapshot())
	}
}

type recordedEvent struct {
	name string
	id   string
	data string
}

type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
	ch     chan recordedEvent
}

func newEventRecorder() *eventRecorder {
	return &eventRecorder{ch: make(chan recordedEvent, 64)}
}

func (r *eventRecorder) emit(name string, payload map[string]any) {
	ev := recordedEvent{name: name, id: stringPayload(payload, "sessionId"), data: stringPayload(payload, "data")}
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
	r.ch <- ev
}

func (r *eventRecorder) waitOutput(sessionID, marker string, timeout time.Duration) bool {
	deadline := time.After(timeout)
	var out strings.Builder
	for {
		select {
		case ev := <-r.ch:
			if ev.name == EventOutput && ev.id == sessionID {
				out.WriteString(ev.data)
				if strings.Contains(out.String(), marker) {
					return true
				}
			}
		case <-deadline:
			return false
		}
	}
}

func (r *eventRecorder) waitEvent(name, sessionID string, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-r.ch:
			if ev.name == name && ev.id == sessionID {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func (r *eventRecorder) waitEvents(name string, sessionIDs []string, timeout time.Duration) bool {
	want := map[string]bool{}
	for _, id := range sessionIDs {
		want[id] = true
	}
	deadline := time.After(timeout)
	for len(want) > 0 {
		select {
		case ev := <-r.ch:
			if ev.name == name && want[ev.id] {
				delete(want, ev.id)
			}
		case <-deadline:
			return false
		}
	}
	return true
}

func (r *eventRecorder) snapshot() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedEvent(nil), r.events...)
}

func stringPayload(payload map[string]any, key string) string {
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}
