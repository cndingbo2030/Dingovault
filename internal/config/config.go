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
	VaultPath string `json:"vaultPath"`
	Theme     string `json:"theme"`
	// Locale is a BCP 47 tag; supported UI catalogs: en, zh-CN (empty = first-run detect in desktop).
	Locale          string `json:"locale,omitempty"`
	Window          Window `json:"window"`
	GitRemoteOrigin string `json:"gitRemoteOrigin,omitempty"` // optional GitHub (or other) remote for vault sync
	// CloudMode uses RemoteStore (HTTP API) instead of local SQLite; vaultPath remains the on-disk markdown root.
	CloudMode   bool   `json:"cloudMode,omitempty"`
	CloudAPIURL string `json:"cloudApiUrl,omitempty"`
	CloudToken  string `json:"cloudToken,omitempty"` // JWT; keep machine-local — never commit this file from ~/.config
	// AI configures local (Ollama) or cloud (OpenAI) LLM + embeddings.
	AI AISettings `json:"ai,omitempty"`
	// Sync holds optional WebDAV mirror settings and LAN pairing (credentials stay on this machine only).
	Sync SyncSettings `json:"sync,omitempty"`
}

// SyncSettings configures WebDAV vault mirror and optional mDNS + PIN pairing on the LAN.
type SyncSettings struct {
	WebDAVURL        string `json:"webdavUrl,omitempty"`
	WebDAVUser       string `json:"webdavUser,omitempty"`
	WebDAVPassword   string `json:"webdavPassword,omitempty"`
	WebDAVRemoteRoot string `json:"webdavRemoteRoot,omitempty"` // path prefix on server, e.g. /vault
	// PairingPort is the TCP port for LAN PIN handshake (default 17375 when advertising).
	PairingPort int `json:"pairingPort,omitempty"`
	// AdvertiseLAN publishes this device via mDNS while pairing is active.
	AdvertiseLAN bool `json:"advertiseLan,omitempty"`
	// LANInstanceName is shown to other devices (default: hostname).
	LANInstanceName string `json:"lanInstanceName,omitempty"`
	// S3* configures optional bucket sync (AWS S3 or S3-compatible endpoints such as MinIO).
	S3Region    string `json:"s3Region,omitempty"`
	S3Bucket    string `json:"s3Bucket,omitempty"`
	S3Prefix    string `json:"s3Prefix,omitempty"`
	S3AccessKey string `json:"s3AccessKey,omitempty"`
	S3SecretKey string `json:"s3SecretKey,omitempty"`
	S3Endpoint  string `json:"s3Endpoint,omitempty"`
}

// AISettings is persisted in config.json alongside vault preferences.
type AISettings struct {
	Provider        string  `json:"provider,omitempty"`        // "ollama" | "openai"
	Model           string  `json:"model,omitempty"`           // chat model name
	Endpoint        string  `json:"endpoint,omitempty"`        // Ollama base URL or OpenAI API root (optional)
	APIKey          string  `json:"apiKey,omitempty"`          // OpenAI key; keep local
	Temperature     float64 `json:"temperature,omitempty"`     // sampling temperature
	EmbeddingsModel string  `json:"embeddingsModel,omitempty"` // separate embedding model (e.g. nomic-embed-text)
	// DisableEmbeddings skips background embedding writes after index (reduces load on Ollama/OpenAI).
	DisableEmbeddings bool `json:"disableEmbeddings,omitempty"`
	// SystemPrompt is sent as the system message for AI Chat (RAG). Empty uses the built-in default.
	SystemPrompt string `json:"systemPrompt,omitempty"`
	// SemanticTopK is how many similar blocks from the vault to inject as RAG context (0 = default 8).
	SemanticTopK int `json:"semanticTopK,omitempty"`
}

// NormalizeAISettings fills empty fields with defaults (safe for API calls).
func NormalizeAISettings(a AISettings) AISettings {
	d := Default().AI
	out := a
	if strings.TrimSpace(out.Provider) == "" {
		out.Provider = d.Provider
	}
	if strings.TrimSpace(out.Model) == "" {
		out.Model = d.Model
	}
	if strings.TrimSpace(out.Endpoint) == "" {
		out.Endpoint = d.Endpoint
	}
	if out.Temperature <= 0 {
		out.Temperature = d.Temperature
	}
	if !out.DisableEmbeddings && strings.TrimSpace(out.EmbeddingsModel) == "" {
		out.EmbeddingsModel = d.EmbeddingsModel
	}
	if strings.TrimSpace(out.SystemPrompt) == "" {
		out.SystemPrompt = d.SystemPrompt
	}
	if out.SemanticTopK <= 0 {
		out.SemanticTopK = d.SemanticTopK
	}
	return out
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
		AI: AISettings{
			Provider:        "ollama",
			Model:           "llama3.2",
			Endpoint:        "http://127.0.0.1:11434",
			Temperature:     0.7,
			EmbeddingsModel: "nomic-embed-text",
			SystemPrompt: "You are Dingovault AI. Answer using only the provided context from the user's vault " +
				"(current page and semantically related blocks). If the answer is not supported by that context, " +
				"say clearly that you do not know or cannot find it in the vault.",
			SemanticTopK: 8,
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
	c.AI = NormalizeAISettings(c.AI)
	c.Sync = NormalizeSyncSettings(c.Sync)
	return c, nil
}

// NormalizeSyncSettings fills defaults for sync-related fields.
func NormalizeSyncSettings(s SyncSettings) SyncSettings {
	out := s
	if out.PairingPort <= 0 {
		out.PairingPort = 17375
	}
	return out
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
