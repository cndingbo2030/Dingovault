package bridge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const consoleCommandTimeout = 45 * time.Second

// ConsoleCommandResult is a single local command result for the desktop console panel.
type ConsoleCommandResult struct {
	Command    string `json:"command"`
	Cwd        string `json:"cwd"`
	Output     string `json:"output"`
	ExitCode   int    `json:"exitCode"`
	DurationMs int64  `json:"durationMs"`
	TimedOut   bool   `json:"timedOut"`
}

// RunVaultCommand runs a local shell command from the current vault root.
//
// This intentionally behaves like a local developer console: it can execute arbitrary
// commands typed by the user, but it stays scoped to the configured vault directory.
func (a *App) RunVaultCommand(command string) (ConsoleCommandResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return ConsoleCommandResult{}, fmt.Errorf("empty command")
	}

	cwd := a.notesRoot
	if cwd == "" {
		cwd = "."
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return ConsoleCommandResult{}, err
	}
	if st, err := os.Stat(absCwd); err != nil || !st.IsDir() {
		return ConsoleCommandResult{}, fmt.Errorf("console cwd is not available: %s", absCwd)
	}

	ctx, cancel := context.WithTimeout(context.Background(), consoleCommandTimeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "/bin/zsh", "-lc", command)
	cmd.Dir = absCwd
	cmd.Env = consoleEnv()

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()

	result := ConsoleCommandResult{
		Command:    command,
		Cwd:        absCwd,
		Output:     out.String(),
		ExitCode:   0,
		DurationMs: time.Since(start).Milliseconds(),
		TimedOut:   ctx.Err() == context.DeadlineExceeded,
	}
	if result.TimedOut {
		result.ExitCode = -1
		if !strings.HasSuffix(result.Output, "\n") && result.Output != "" {
			result.Output += "\n"
		}
		result.Output += "Command timed out.\n"
		return result, nil
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		return result, err
	}
	return result, nil
}

func consoleEnv() []string {
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
	env = append(env, "DINGOVAULT_CONSOLE=1")
	return env
}
