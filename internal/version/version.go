// Package version holds the application version string embedded at link time for desktop builds.
package version

// String is overridden with -ldflags "-X github.com/cndingbo2030/dingovault/internal/version.String=..." in release builds.
var String = "1.1.0-beta"
