package platform

import "path/filepath"

// AndroidScopedVaultPath returns the Markdown vault directory under Android app-specific
// external storage. Pass the absolute path from Context.getExternalFilesDir(null) on the host.
func AndroidScopedVaultPath(externalFilesDir string) string {
	if externalFilesDir == "" {
		return ""
	}
	return filepath.Join(filepath.Clean(externalFilesDir), "Dingovault")
}
