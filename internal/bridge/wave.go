package bridge

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// WaveOpenResult reports whether Dingovault could hand off the current vault to Wave.
type WaveOpenResult struct {
	Opened  bool   `json:"opened"`
	Command string `json:"command"`
	Message string `json:"message"`
}

// OpenInWave opens Wave Terminal at the vault root or a vault-relative cwd when Wave is installed.
func (a *App) OpenInWave(cwd string) (WaveOpenResult, error) {
	mgr, err := a.getTerminalManager()
	if err != nil {
		return WaveOpenResult{}, err
	}
	absCwd, err := mgr.ResolveCwd(cwd)
	if err != nil {
		return WaveOpenResult{}, err
	}

	if bin, err := findWaveBinary(); err == nil {
		cmd := exec.Command(bin, absCwd)
		if err := cmd.Start(); err != nil {
			return WaveOpenResult{}, err
		}
		return WaveOpenResult{Opened: true, Command: bin + " " + absCwd, Message: "Wave opened."}, nil
	}

	if runtime.GOOS == "darwin" {
		cmd := exec.Command("open", "-a", "Wave", absCwd)
		if err := cmd.Start(); err == nil {
			return WaveOpenResult{Opened: true, Command: "open -a Wave " + absCwd, Message: "Wave opened."}, nil
		}
	}

	msg := "Wave is not installed or no Wave CLI/app launcher was found."
	return WaveOpenResult{Opened: false, Command: "", Message: msg}, nil
}

func findWaveBinary() (string, error) {
	for _, name := range []string{"wave", "waveterm", "Wave"} {
		if p, err := exec.LookPath(name); err == nil && strings.TrimSpace(p) != "" {
			return p, nil
		}
	}
	return "", fmt.Errorf("wave not found")
}
