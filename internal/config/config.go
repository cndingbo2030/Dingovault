package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const appDirName = "dingovault"

// Config is persisted under the OS user config directory.
type Config struct {
	VaultPath       string `json:"vaultPath"`
	Theme           string `json:"theme"`
	Window          Window `json:"window"`
	GitRemoteOrigin string `json:"gitRemoteOrigin,omitempty"` // optional GitHub (or other) remote for vault sync
	// CloudMode uses RemoteStore (HTTP API) instead of local SQLite; vaultPath remains the on-disk markdown root.
	CloudMode   bool   `json:"cloudMode,omitempty"`
	CloudAPIURL string `json:"cloudApiUrl,omitempty"`
	CloudToken  string `json:"cloudToken,omitempty"` // JWT; keep machine-local — never commit this file from ~/.config
}

// Window holds last known frame geometry.
type Window struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	X      int `json:"x"`
	Y      int `json:"y"`
}

// Dir returns ~/.config/dingovault (or platform equivalent).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

// Path is the full path to config.json.
func Path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// Default returns factory defaults.
func Default() Config {
	return Config{
		Theme: "dark",
		Window: Window{
			Width:  1280,
			Height: 800,
		},
	}
}

// Load reads config from disk; missing file yields defaults with nil error.
func Load() (Config, error) {
	c := Default()
	p, err := Path()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.Window.Width <= 0 {
		c.Window.Width = 1280
	}
	if c.Window.Height <= 0 {
		c.Window.Height = 800
	}
	if c.Theme == "" {
		c.Theme = "dark"
	}
	return c, nil
}

// ShouldOpenBundledDemo is true when no vault was passed on the CLI and none is saved in config.
// The desktop app uses this to materialize the built-in Demo Vault for first-time onboarding.
func ShouldOpenBundledDemo(notesCLI string, c Config) bool {
	return strings.TrimSpace(notesCLI) == "" && strings.TrimSpace(c.VaultPath) == ""
}

// Save writes config atomically (write temp + rename in same dir).
func Save(c Config) error {
	d, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	p, err := Path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(d, "config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil {
		_ = os.Remove(tmpPath)
		return werr
	}
	if cerr != nil {
		_ = os.Remove(tmpPath)
		return cerr
	}
	if err := os.Rename(tmpPath, p); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
