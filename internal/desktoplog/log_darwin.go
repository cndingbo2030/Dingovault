//go:build darwin

package desktoplog

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// Install writes log output to ~/Library/Logs/Dingovault.log in addition to stderr.
func Install() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	dir := filepath.Join(home, "Library", "Logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	p := filepath.Join(dir, "Dingovault.log")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.Printf("Dingovault starting (log file: %s)", p)
}
