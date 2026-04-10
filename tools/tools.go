//go:build tools

// Package tools holds build-tool-only imports so "go mod tidy" retains golang.org/x/mobile for gomobile/gobind on CI.
package tools

import _ "golang.org/x/mobile/bind"
