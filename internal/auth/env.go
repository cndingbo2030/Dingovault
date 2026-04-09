package auth

import (
	"os"
	"strings"
)

// IsProduction reports whether Dingovault should use production hardening rules.
// Set DINGO_ENV=production (or prod) on servers; unset or "development" keeps dev-friendly defaults.
func IsProduction() bool {
	e := strings.ToLower(strings.TrimSpace(os.Getenv("DINGO_ENV")))
	switch e {
	case "production", "prod":
		return true
	default:
		return false
	}
}
