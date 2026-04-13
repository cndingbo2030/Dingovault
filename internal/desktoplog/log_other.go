//go:build !darwin

package desktoplog

// Install is a no-op outside macOS.
func Install() {}
