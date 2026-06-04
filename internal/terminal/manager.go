package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

const (
	EventSessionStarted = "terminal-session-started"
	EventOutput         = "terminal-output"
	EventExit           = "terminal-exit"
	EventError          = "terminal-error"

	defaultRows     = 24
	defaultCols     = 100
	maxRows         = 200
	maxCols         = 400
	maxSessions     = 8
	maxCaptureBytes = 128 * 1024
)

// Emitter publishes terminal events to the UI layer.
type Emitter func(name string, payload map[string]any)

// SessionInfo is returned when a PTY session starts.
type SessionInfo struct {
	ID  string `json:"id"`
	Cwd string `json:"cwd"`
}

// CommandResult is a bounded capture from a PTY-backed command run.
type CommandResult struct {
	SessionID  string `json:"sessionId"`
	Command    string `json:"command"`
	Cwd        string `json:"cwd"`
	Output     string `json:"output"`
	ExitCode   int    `json:"exitCode"`
	DurationMs int64  `json:"durationMs"`
}

// Manager owns ephemeral PTY sessions scoped to a vault root.
type Manager struct {
	root     string
	emit     Emitter
	newID    func() string
	shellEnv func() []string

	mu       sync.Mutex
	sessions map[string]*Session
}

// Session is one interactive PTY process.
type Session struct {
	id  string
	cwd string
	cmd *exec.Cmd
	pty *os.File
}

// NewManager constructs a terminal manager rooted at vaultRoot.
func NewManager(vaultRoot string, emit Emitter) (*Manager, error) {
	root, err := filepath.Abs(strings.TrimSpace(vaultRoot))
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("terminal root is not available: %s", root)
	}
	if emit == nil {
		emit = func(string, map[string]any) {}
	}
	return &Manager{
		root:     root,
		emit:     emit,
		newID:    func() string { return uuid.NewString() },
		shellEnv: TerminalEnv,
		sessions: make(map[string]*Session),
	}, nil
}

// Root returns the absolute vault root used for cwd resolution.
func (m *Manager) Root() string {
	return m.root
}

// ResolveCwd resolves cwd as vault-relative by default and rejects paths outside the vault root.
func (m *Manager) ResolveCwd(cwd string) (string, error) {
	root := filepath.Clean(m.root)
	p := strings.TrimSpace(cwd)
	var abs string
	if p == "" {
		abs = root
	} else if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Clean(filepath.Join(root, p))
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("terminal cwd escapes vault root")
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return "", fmt.Errorf("terminal cwd is not available: %s", abs)
	}
	return abs, nil
}

// StartSession starts an interactive shell PTY.
func (m *Manager) StartSession(ctx context.Context, cwd string, rows, cols int) (SessionInfo, error) {
	absCwd, err := m.ResolveCwd(cwd)
	if err != nil {
		return SessionInfo{}, err
	}
	shell, args := interactiveShell()
	cmd := exec.CommandContext(contextWithoutCancel(ctx), shell, args...)
	cmd.Dir = absCwd
	cmd.Env = m.shellEnv()

	id := m.newID()
	f, err := pty.StartWithSize(cmd, winsize(rows, cols))
	if err != nil {
		return SessionInfo{}, fmt.Errorf("start pty: %w", err)
	}
	s := &Session{id: id, cwd: absCwd, cmd: cmd, pty: f}

	m.mu.Lock()
	if len(m.sessions) >= maxSessions {
		m.mu.Unlock()
		_ = f.Close()
		_ = cmd.Process.Kill()
		return SessionInfo{}, fmt.Errorf("too many terminal sessions")
	}
	m.sessions[id] = s
	m.mu.Unlock()

	m.emit(EventSessionStarted, map[string]any{"sessionId": id, "cwd": absCwd, "kind": "interactive"})
	go m.readLoop(s, nil, nil)
	return SessionInfo{ID: id, Cwd: absCwd}, nil
}

// WriteInput writes raw terminal input to an interactive PTY.
func (m *Manager) WriteInput(sessionID, input string) error {
	s, err := m.session(sessionID)
	if err != nil {
		return err
	}
	_, err = io.WriteString(s.pty, input)
	return err
}

// Resize resizes an interactive PTY.
func (m *Manager) Resize(sessionID string, rows, cols int) error {
	s, err := m.session(sessionID)
	if err != nil {
		return err
	}
	return pty.Setsize(s.pty, winsize(rows, cols))
}

// CloseSession kills an interactive PTY and removes it from the manager.
func (m *Manager) CloseSession(sessionID string) error {
	s, err := m.removeSession(sessionID)
	if err != nil {
		return err
	}
	_ = s.pty.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return nil
}

// RunCommand runs command in a PTY-backed non-interactive shell and returns bounded output.
func (m *Manager) RunCommand(ctx context.Context, cwd, command string) (CommandResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return CommandResult{}, fmt.Errorf("empty command")
	}
	absCwd, err := m.ResolveCwd(cwd)
	if err != nil {
		return CommandResult{}, err
	}
	shell, args := commandShell(command)
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Dir = absCwd
	cmd.Env = m.shellEnv()

	id := m.newID()
	start := time.Now()
	f, err := pty.StartWithSize(cmd, winsize(defaultRows, defaultCols))
	if err != nil {
		return CommandResult{}, fmt.Errorf("start command pty: %w", err)
	}
	s := &Session{id: id, cwd: absCwd, cmd: cmd, pty: f}

	capture := &boundedCapture{limit: maxCaptureBytes}
	m.emit(EventSessionStarted, map[string]any{"sessionId": id, "cwd": absCwd, "kind": "command", "command": command})
	done := make(chan CommandResult, 1)
	go m.readLoop(s, capture.write, func(exitCode int) {
		done <- CommandResult{
			SessionID:  id,
			Command:    command,
			Cwd:        absCwd,
			Output:     capture.String(),
			ExitCode:   exitCode,
			DurationMs: time.Since(start).Milliseconds(),
		}
	})

	select {
	case <-ctx.Done():
		_ = f.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return CommandResult{}, ctx.Err()
	case result := <-done:
		return result, nil
	}
}

func (m *Manager) readLoop(s *Session, onOutput func(string), onExit func(exitCode int)) {
	exitCode := 0
	buf := make([]byte, 4096)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			data := string(buf[:n])
			if onOutput != nil {
				onOutput(data)
			}
			m.emit(EventOutput, map[string]any{"sessionId": s.id, "data": data})
		}
		if err != nil {
			break
		}
	}
	if err := s.cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			m.emit(EventError, map[string]any{"sessionId": s.id, "message": err.Error()})
		}
	}
	m.removeSessionNoError(s.id)
	m.emit(EventExit, map[string]any{"sessionId": s.id, "exitCode": exitCode})
	if onExit != nil {
		onExit(exitCode)
	}
}

func (m *Manager) session(sessionID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[strings.TrimSpace(sessionID)]
	if s == nil {
		return nil, fmt.Errorf("terminal session not found")
	}
	return s, nil
}

func (m *Manager) removeSession(sessionID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := strings.TrimSpace(sessionID)
	s := m.sessions[id]
	if s == nil {
		return nil, fmt.Errorf("terminal session not found")
	}
	delete(m.sessions, id)
	return s, nil
}

func (m *Manager) removeSessionNoError(sessionID string) {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}

func winsize(rows, cols int) *pty.Winsize {
	if rows <= 0 {
		rows = defaultRows
	}
	if cols <= 0 {
		cols = defaultCols
	}
	if rows > maxRows {
		rows = maxRows
	}
	if cols > maxCols {
		cols = maxCols
	}
	return &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
}

func contextWithoutCancel(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func interactiveShell() (string, []string) {
	shell := firstShell()
	if filepath.Base(shell) == "sh" {
		return shell, []string{"-i"}
	}
	return shell, []string{"-l"}
}

func commandShell(command string) (string, []string) {
	shell := firstShell()
	if filepath.Base(shell) == "sh" {
		return shell, []string{"-c", command}
	}
	return shell, []string{"-lc", command}
}

func firstShell() string {
	for _, shell := range []string{os.Getenv("SHELL"), "/bin/zsh", "/bin/bash", "/bin/sh", "sh"} {
		shell = strings.TrimSpace(shell)
		if shell == "" {
			continue
		}
		if shell != "sh" {
			if st, err := os.Stat(shell); err != nil || st.IsDir() {
				continue
			}
		}
		return shell
	}
	return "sh"
}

// TerminalEnv returns a PATH suitable for a local developer terminal.
func TerminalEnv() []string {
	env := os.Environ()
	path := os.Getenv("PATH")
	parts := []string{
		filepath.Join(os.Getenv("HOME"), "go", "bin"),
		"/opt/homebrew/bin",
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
	}
	if strings.TrimSpace(path) != "" {
		parts = append(parts, path)
	}
	env = append(env, "PATH="+strings.Join(parts, ":"))
	env = append(env, "DINGOVAULT_TERMINAL=1")
	return env
}

type boundedCapture struct {
	mu      sync.Mutex
	buf     strings.Builder
	limit   int
	dropped bool
}

func (c *boundedCapture) write(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.limit <= 0 || s == "" {
		return
	}
	remaining := c.limit - c.buf.Len()
	if remaining <= 0 {
		c.dropped = true
		return
	}
	if len(s) > remaining {
		c.buf.WriteString(s[:remaining])
		c.dropped = true
		return
	}
	c.buf.WriteString(s)
}

func (c *boundedCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.buf.String()
	if c.dropped {
		if !strings.HasSuffix(out, "\n") && out != "" {
			out += "\n"
		}
		out += "[output truncated]\n"
	}
	return out
}
