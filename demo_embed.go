//go:build !bindings

package main

import "embed"

//go:embed demo-vault
var embeddedDemoVault embed.FS
