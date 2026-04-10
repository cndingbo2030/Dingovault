// Package mobile is built with gomobile bind for Android (.aar).
package mobile

import (
	"embed"

	"github.com/cndingbo2030/dingovault/internal/platform"
	"github.com/cndingbo2030/dingovault/internal/version"
)

//go:embed all:demo-vault
var embeddedDemoVault embed.FS

// Version returns the embedded Dingovault version string.
func Version() string {
	return version.String
}

// VaultPath returns the recommended vault root under Android scoped storage.
func VaultPath(androidExternalFilesDir string) string {
	return platform.AndroidScopedVaultPath(androidExternalFilesDir)
}

// EventSink receives JSON payloads for Wails-compatible events (e.g. ai-inline-chunk).
type EventSink interface {
	Emit(name string, payloadJSON string)
}
