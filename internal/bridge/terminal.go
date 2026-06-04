package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cndingbo2030/dingovault/internal/locale"
	"github.com/cndingbo2030/dingovault/internal/terminal"
	"github.com/google/uuid"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// TerminalSessionDTO describes an ephemeral PTY terminal block.
type TerminalSessionDTO struct {
	ID  string `json:"id"`
	Cwd string `json:"cwd"`
}

// TerminalCommandResultDTO is a PTY-backed command result that can be written back into notes.
type TerminalCommandResultDTO struct {
	SessionID  string `json:"sessionId"`
	Command    string `json:"command"`
	Cwd        string `json:"cwd"`
	Output     string `json:"output"`
	ExitCode   int    `json:"exitCode"`
	DurationMs int64  `json:"durationMs"`
}

func (a *App) getTerminalManager() (*terminal.Manager, error) {
	root := a.NotesRoot()
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	a.terminalMu.Lock()
	if a.terminalManager != nil && a.terminalManager.Root() == root {
		defer a.terminalMu.Unlock()
		return a.terminalManager, nil
	}
	old := a.terminalManager
	a.terminalManager = nil
	a.terminalMu.Unlock()
	if old != nil {
		old.Shutdown()
	}
	mgr, err := terminal.NewManager(root, a.emitTerminalEvent)
	if err != nil {
		return nil, err
	}
	a.terminalMu.Lock()
	defer a.terminalMu.Unlock()
	a.terminalManager = mgr
	return mgr, nil
}

// shutdownTerminalSessions closes all live PTY sessions for app shutdown or vault replacement.
func (a *App) shutdownTerminalSessions() {
	a.terminalMu.Lock()
	mgr := a.terminalManager
	a.terminalManager = nil
	a.terminalMu.Unlock()
	if mgr != nil {
		mgr.Shutdown()
	}
}

func (a *App) emitTerminalEvent(name string, payload map[string]any) {
	if a.EventEmitter != nil {
		a.EventEmitter(name, payload)
		return
	}
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, name, payload)
}

// StartTerminalSession opens an interactive PTY shell scoped to the vault root or a vault-relative cwd.
func (a *App) StartTerminalSession(cwd string) (TerminalSessionDTO, error) {
	mgr, err := a.getTerminalManager()
	if err != nil {
		return TerminalSessionDTO{}, err
	}
	info, err := mgr.StartSession(a.terminalContext(), cwd, 24, 100)
	if err != nil {
		return TerminalSessionDTO{}, err
	}
	return TerminalSessionDTO{ID: info.ID, Cwd: info.Cwd}, nil
}

// WriteTerminalInput sends raw stdin bytes to a PTY session.
func (a *App) WriteTerminalInput(sessionID, input string) error {
	mgr, err := a.getTerminalManager()
	if err != nil {
		return err
	}
	return mgr.WriteInput(sessionID, input)
}

// ResizeTerminal resizes a PTY session.
func (a *App) ResizeTerminal(sessionID string, rows, cols int) error {
	mgr, err := a.getTerminalManager()
	if err != nil {
		return err
	}
	return mgr.Resize(sessionID, rows, cols)
}

// CloseTerminalSession kills a PTY session.
func (a *App) CloseTerminalSession(sessionID string) error {
	mgr, err := a.getTerminalManager()
	if err != nil {
		return err
	}
	return mgr.CloseSession(sessionID)
}

// RunBlockCommand executes command in a PTY, streams output to the console, and appends a child result block.
func (a *App) RunBlockCommand(blockID, command, cwd string, confirmed bool) (TerminalCommandResultDTO, error) {
	readOnly, reason := terminal.ClassifyCommand(command)
	if !readOnly && !confirmed {
		return TerminalCommandResultDTO{}, fmt.Errorf("block command requires confirmation: %s", reason)
	}
	if a.graph == nil {
		return TerminalCommandResultDTO{}, fmt.Errorf("%s", a.t(locale.ErrGraphNotInit))
	}
	mgr, err := a.getTerminalManager()
	if err != nil {
		return TerminalCommandResultDTO{}, err
	}
	ctx := a.terminalContext()
	result, err := mgr.RunCommand(ctx, cwd, command)
	if err != nil {
		return TerminalCommandResultDTO{}, err
	}
	content := terminalResultBlockContent(result)
	if err := a.graph.InsertChildBlock(ctx, blockID, content); err != nil {
		return TerminalCommandResultDTO{}, err
	}
	a.invalidatePageCache()
	return TerminalCommandResultDTO{
		SessionID:  result.SessionID,
		Command:    result.Command,
		Cwd:        result.Cwd,
		Output:     result.Output,
		ExitCode:   result.ExitCode,
		DurationMs: result.DurationMs,
	}, nil
}

func (a *App) terminalContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func terminalResultBlockContent(result terminal.CommandResult) string {
	output := strings.TrimRight(result.Output, "\r\n")
	if output == "" {
		output = "(no output)"
	}
	now := time.Now().UTC()
	fence := markdownFence(output)
	return strings.Join([]string{
		"Terminal result",
		"properties:",
		"runId:: " + uuid.NewString(),
		"source:: terminal",
		fmt.Sprintf("exitCode:: %d", result.ExitCode),
		"ranAt:: " + now.Format(time.RFC3339Nano),
		fmt.Sprintf("durationMs:: %d", result.DurationMs),
		"command:: " + singleLinePropertyValue(result.Command),
		"output:",
		fence + "text",
		output,
		fence,
	}, "\n")
}

func singleLinePropertyValue(value string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(value, "\r", "\n")), " ")
}

func markdownFence(content string) string {
	n := 3
	run := 0
	for _, r := range content {
		if r == '`' {
			run++
			if run >= n {
				n = run + 1
			}
			continue
		}
		run = 0
	}
	return strings.Repeat("`", n)
}

// ResolveTerminalCwd returns a safe absolute cwd for a vault-relative path or the vault root.
func (a *App) ResolveTerminalCwd(cwd string) (string, error) {
	mgr, err := a.getTerminalManager()
	if err != nil {
		return "", err
	}
	return mgr.ResolveCwd(cwd)
}
