// Package mobile is built with gomobile bind for Android (.aar). It exposes a thin API for
// version and scoped-storage vault paths; expand here as the native shell grows.
package mobile

import (
	"github.com/cndingbo2030/dingovault/internal/platform"
	"github.com/cndingbo2030/dingovault/internal/version"
)

// Version returns the embedded Dingovault version string.
func Version() string {
	return version.String
}

// VaultPath returns the recommended vault root under Android scoped storage.
// androidExternalFilesDir must be the path from Context.getExternalFilesDir(null).
func VaultPath(androidExternalFilesDir string) string {
	return platform.AndroidScopedVaultPath(androidExternalFilesDir)
}
